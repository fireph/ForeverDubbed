//go:build windows && cgo

#define WIN32_LEAN_AND_MEAN
#define NOMINMAX
#include "dxgi_windows.h"
#include "dxgi_geometry.h"
#include <d3d11.h>
#include <dxgi1_2.h>
#include <dwmapi.h>
#include <wrl/client.h>
#include <cstdio>
#include <cstdint>
#include <cstring>
#include <memory>
#include <new>
#include <vector>

using Microsoft::WRL::ComPtr;
struct DesktopOutput {
    ComPtr<ID3D11Device> device;
    ComPtr<ID3D11DeviceContext> context;
    ComPtr<IDXGIOutputDuplication> duplication;
    ComPtr<ID3D11Texture2D> latest, staging;
    DXGI_OUTPUT_DESC desc{};
};
struct fdb_dxgi {
    HWND window = nullptr;
    DWORD pid = 0;
    RECT bounds{};
    ComPtr<IDXGIFactory1> factory;
    std::vector<std::unique_ptr<DesktopOutput>> outputs;
};
static int fail(char *error, size_t size, const char *message, HRESULT hr = S_OK) {
    if (FAILED(hr))
        snprintf(error, size, "%s (HRESULT 0x%08lx)", message, static_cast<unsigned long>(hr));
    else
        snprintf(error, size, "%s", message);
    return -1;
}
static fdb::Rect rect(RECT r) { return {int(r.left), int(r.top), int(r.right), int(r.bottom)}; }
static bool gameBounds(HWND window, DWORD pid, RECT *bounds) {
    DWORD owner = 0, cloaked = 0;
    if (!IsWindow(window) || !IsWindowVisible(window) || IsIconic(window))
        return false;
    GetWindowThreadProcessId(window, &owner);
    if (owner != pid || !owner ||
        (SUCCEEDED(DwmGetWindowAttribute(window, DWMWA_CLOAKED, &cloaked, sizeof(cloaked))) && cloaked))
        return false;
    // Keep the same window-relative coordinate convention as the shared decoder.
    return GetWindowRect(window, bounds) && !rect(*bounds).empty();
}
fdb_dxgi *fdb_dxgi_open() { return new (std::nothrow) fdb_dxgi; }
void fdb_dxgi_close(fdb_dxgi *c) { delete c; }
void fdb_dxgi_reset(fdb_dxgi *c) {
    c->outputs.clear();
    c->factory.Reset();
    c->window = nullptr;
    c->pid = 0;
    c->bounds = {};
}
void fdb_dxgi_poll(fdb_dxgi *c) {
    RECT bounds{};
    if (c->window && (!gameBounds(c->window, c->pid, &bounds) ||
                      !EqualRect(&bounds, &c->bounds) || (c->factory && !c->factory->IsCurrent())))
        fdb_dxgi_reset(c);
}
int fdb_dxgi_select(fdb_dxgi *c, HWND window, DWORD pid, int *width, int *height,
                    char *error, size_t size) {
    RECT bounds{};
    if (!gameBounds(window, pid, &bounds)) {
        fdb_dxgi_reset(c);
        return fail(error, size, "Game window is closed, minimized, or unavailable");
    }
    auto area = rect(bounds);
    if (int64_t(area.width()) * area.height() > 100000000)
        return fail(error, size, "Invalid game window dimensions");
    if (c->window != window || c->pid != pid || !EqualRect(&c->bounds, &bounds) ||
        (c->factory && !c->factory->IsCurrent()))
        fdb_dxgi_reset(c);
    c->window = window;
    c->pid = pid;
    c->bounds = bounds;
    *width = area.width();
    *height = area.height();
    return 0;
}
// Called only after validating a visible game window. Never opens duplication
// at app startup, while waiting for WoW, or as a fallback on Windows 11.
static HRESULT openOutputs(fdb_dxgi *c) {
    HRESULT hr = CreateDXGIFactory1(IID_PPV_ARGS(c->factory.GetAddressOf()));
    if (FAILED(hr))
        return hr;
    for (UINT ai = 0;; ++ai) {
        ComPtr<IDXGIAdapter1> adapter;
        hr = c->factory->EnumAdapters1(ai, adapter.GetAddressOf());
        if (hr == DXGI_ERROR_NOT_FOUND)
            break;
        if (FAILED(hr))
            return hr;
        for (UINT oi = 0;; ++oi) {
            ComPtr<IDXGIOutput> output;
            hr = adapter->EnumOutputs(oi, output.GetAddressOf());
            if (hr == DXGI_ERROR_NOT_FOUND)
                break;
            if (FAILED(hr))
                return hr;
            DXGI_OUTPUT_DESC desc{};
            hr = output->GetDesc(&desc);
            if (FAILED(hr))
                return hr;
            if (!desc.AttachedToDesktop || fdb::intersect(rect(desc.DesktopCoordinates), rect(c->bounds)).empty())
                continue;
            auto monitor = std::make_unique<DesktopOutput>();
            monitor->desc = desc;
            // The device must belong to this output's adapter (including hybrid GPUs).
            hr = D3D11CreateDevice(adapter.Get(), D3D_DRIVER_TYPE_UNKNOWN, nullptr,
                                   D3D11_CREATE_DEVICE_BGRA_SUPPORT, nullptr, 0, D3D11_SDK_VERSION,
                                   monitor->device.GetAddressOf(), nullptr, monitor->context.GetAddressOf());
            ComPtr<IDXGIOutput1> output1;
            if (SUCCEEDED(hr))
                hr = output.As(&output1);
            if (SUCCEEDED(hr))
                hr = output1->DuplicateOutput(monitor->device.Get(), monitor->duplication.GetAddressOf());
            if (FAILED(hr))
                return hr;
            c->outputs.push_back(std::move(monitor));
        }
    }
    return c->outputs.empty() ? DXGI_ERROR_NOT_FOUND : S_OK;
}
static HRESULT updateFrame(DesktopOutput &output) {
    DXGI_OUTDUPL_FRAME_INFO info{};
    ComPtr<IDXGIResource> resource;
    HRESULT hr = output.duplication->AcquireNextFrame(output.latest ? 0 : 100, &info, resource.GetAddressOf());
    if (hr == DXGI_ERROR_WAIT_TIMEOUT)
        return output.latest ? S_OK : hr; // Static content can reuse the last valid frame.
    if (FAILED(hr))
        return hr;
    // Every successful acquisition must be released, even if QI/allocation fails.
    ComPtr<ID3D11Texture2D> texture;
    hr = resource.As(&texture);
    if (SUCCEEDED(hr)) {
        D3D11_TEXTURE2D_DESC desc{};
        texture->GetDesc(&desc);
        bool rotated = output.desc.Rotation == DXGI_MODE_ROTATION_ROTATE90 ||
                       output.desc.Rotation == DXGI_MODE_ROTATION_ROTATE270;
        auto monitor = rect(output.desc.DesktopCoordinates);
        if (desc.Format != DXGI_FORMAT_B8G8R8A8_UNORM ||
            desc.Width != UINT(rotated ? monitor.height() : monitor.width()) ||
            desc.Height != UINT(rotated ? monitor.width() : monitor.height())) {
            hr = DXGI_ERROR_ACCESS_LOST;
        } else {
            if (!output.latest) {
                desc.Usage = D3D11_USAGE_DEFAULT;
                desc.BindFlags = desc.CPUAccessFlags = desc.MiscFlags = 0;
                hr = output.device->CreateTexture2D(&desc, nullptr, output.latest.GetAddressOf());
            }
            if (SUCCEEDED(hr))
                output.context->CopyResource(output.latest.Get(), texture.Get());
        }
    }
    HRESULT released = output.duplication->ReleaseFrame();
    return FAILED(hr) ? hr : released;
}
static HRESULT copyOutput(DesktopOutput &output, fdb::Rect request, unsigned char *rgba) {
    auto monitor = rect(output.desc.DesktopCoordinates);
    auto overlap = fdb::intersect(request, monitor);
    if (overlap.empty())
        return S_OK;
    HRESULT hr = updateFrame(output);
    if (FAILED(hr))
        return hr;
    D3D11_TEXTURE2D_DESC texture{};
    output.latest->GetDesc(&texture);
    fdb::Rect local{overlap.left - monitor.left, overlap.top - monitor.top,
                    overlap.right - monitor.left, overlap.bottom - monitor.top};
    auto source = fdb::textureRect(local, texture.Width, texture.Height, output.desc.Rotation);
    D3D11_TEXTURE2D_DESC staging{};
    if (output.staging)
        output.staging->GetDesc(&staging);
    if (!output.staging || staging.Width != UINT(source.width()) || staging.Height != UINT(source.height())) {
        output.staging.Reset();
        staging = {};
        staging.Width = source.width();
        staging.Height = source.height();
        staging.MipLevels = staging.ArraySize = staging.SampleDesc.Count = 1;
        staging.Format = DXGI_FORMAT_B8G8R8A8_UNORM;
        staging.Usage = D3D11_USAGE_STAGING;
        staging.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
        hr = output.device->CreateTexture2D(&staging, nullptr, output.staging.GetAddressOf());
        if (FAILED(hr))
            return hr;
    }
    D3D11_BOX box{UINT(source.left), UINT(source.top), 0, UINT(source.right), UINT(source.bottom), 1};
    output.context->CopySubresourceRegion(output.staging.Get(), 0, 0, 0, 0, output.latest.Get(), 0, &box);
    D3D11_MAPPED_SUBRESOURCE mapped{};
    hr = output.context->Map(output.staging.Get(), 0, D3D11_MAP_READ, 0, &mapped);
    if (FAILED(hr))
        return hr;
    fdb::copyDesktopCrop(rgba, request, overlap, monitor, static_cast<unsigned char *>(mapped.pData),
                         mapped.RowPitch, source, texture.Width, texture.Height, output.desc.Rotation);
    output.context->Unmap(output.staging.Get(), 0);
    return S_OK;
}
int fdb_dxgi_capture(fdb_dxgi *c, int x, int y, int width, int height, void *rgba,
                     char *error, size_t size) {
    RECT bounds{};
    if (!gameBounds(c->window, c->pid, &bounds)) {
        fdb_dxgi_reset(c);
        return fail(error, size, "Game window is closed, minimized, or unavailable");
    }
    if (!EqualRect(&bounds, &c->bounds) || (c->factory && !c->factory->IsCurrent())) {
        fdb_dxgi_reset(c);
        return fail(error, size, "Game window or display moved; reacquiring desktop capture");
    }
    auto game = rect(bounds);
    if (!rgba || x < 0 || y < 0 || width <= 0 || height <= 0 ||
        int64_t(x) + width > game.width() || int64_t(y) + height > game.height())
        return fail(error, size, "Capture rectangle is outside the game window");
    HRESULT hr = S_OK;
    if (c->outputs.empty())
        hr = openOutputs(c);
    if (FAILED(hr)) {
        fdb_dxgi_reset(c);
        return fail(error, size, "Start Windows 10 desktop capture; restore WoW on an unlocked desktop", hr);
    }
    auto dest = static_cast<unsigned char *>(rgba);
    // Off-screen portions remain opaque black, never pixels from another location.
    memset(dest, 0, size_t(width) * height * 4);
    for (size_t i = 3; i < size_t(width) * height * 4; i += 4)
        dest[i] = 255;
    fdb::Rect request{game.left + x, game.top + y, game.left + x + width, game.top + y + height};
    for (auto &output : c->outputs) {
        hr = copyOutput(*output, request, dest);
        if (FAILED(hr))
            break;
    }
    // Reject pixels if the game disappeared or moved during GPU readback.
    RECT after{};
    bool gameUnchanged = gameBounds(c->window, c->pid, &after) && EqualRect(&after, &bounds);
    if (FAILED(hr) || !gameUnchanged) {
        // A timeout before the first frame is transient; keep the session for retry.
        if (hr != DXGI_ERROR_WAIT_TIMEOUT || !gameUnchanged)
            fdb_dxgi_reset(c);
        return fail(error, size, "Desktop capture changed or has no frame yet; reacquiring WoW", hr);
    }
    return 0;
}
