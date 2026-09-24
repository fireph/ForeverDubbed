//go:build windows && cgo

#define WIN32_LEAN_AND_MEAN
#define NOMINMAX
#include <windows.h>
#include <dwmapi.h>
#include <d3d11.h>
#include <dxgi.h>
#include <roapi.h>
#include <winstring.h>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <new>
#include "native_windows.h"
#include "wgc_abi_windows.h"

extern "C" HRESULT WINAPI CreateDirect3D11DeviceFromDXGIDevice(IDXGIDevice *, IInspectable **);

template<class T> struct ComPtr {
    T *p = nullptr;
    ComPtr() = default;
    ComPtr(const ComPtr &) = delete;
    ComPtr &operator=(const ComPtr &) = delete;
    ~ComPtr() { reset(); }
    void reset(T *value = nullptr) { if (p) p->Release(); p = value; }
    T **put() { reset(); return &p; }
    T *detach() { T *value = p; p = nullptr; return value; }
    T *operator->() const { return p; }
};
static void closeObject(IUnknown *object) {
    if (!object) return;
    ComPtr<FdbClosable> closable;
    if (SUCCEEDED(object->QueryInterface(iidClosable, reinterpret_cast<void **>(closable.put())))) closable->Close();
}
struct FramePtr : ComPtr<FdbFrame> {
    ~FramePtr() { closeObject(p); }
};
static int fail(char *error, size_t size, const char *message, HRESULT status = S_OK) {
    if (FAILED(status)) snprintf(error, size, "%s (HRESULT 0x%08lx)", message, static_cast<unsigned long>(status));
    else snprintf(error, size, "%s", message);
    return -1;
}
static HRESULT factory(const wchar_t *name, REFIID iid, void **result) {
    HSTRING text = nullptr;
    HRESULT hr = WindowsCreateString(name, static_cast<UINT32>(wcslen(name)), &text);
    if (SUCCEEDED(hr)) hr = RoGetActivationFactory(text, iid, result);
    if (text) WindowsDeleteString(text);
    return hr;
}

struct WindowList { fdb_win_window *head = nullptr; bool allocationFailed = false; };
static bool eligible(HWND window, DWORD *pid) {
    if (!IsWindow(window) || !IsWindowVisible(window) || IsIconic(window)) return false;
    DWORD cloaked = 0;
    if (SUCCEEDED(DwmGetWindowAttribute(window, DWMWA_CLOAKED, &cloaked, sizeof(cloaked))) && cloaked) return false;
    GetWindowThreadProcessId(window, pid);
    return *pid != 0;
}
static BOOL CALLBACK enumerateWindow(HWND window, LPARAM param) {
    auto &list = *reinterpret_cast<WindowList *>(param);
    DWORD pid = 0;
    if (!eligible(window, &pid)) return TRUE;
    RECT rect{};
    if (!GetWindowRect(window, &rect) || rect.right-rect.left < 100 || rect.bottom-rect.top < 100) return TRUE;
    HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
    if (!process) return TRUE;
    wchar_t path[32768];
    DWORD length = 32768;
    BOOL queried = QueryFullProcessImageNameW(process, 0, path, &length);
    CloseHandle(process);
    if (!queried) return TRUE;
    int bytes = WideCharToMultiByte(CP_UTF8, 0, path, length, nullptr, 0, nullptr, nullptr);
    if (bytes <= 0) return TRUE;
    auto node = static_cast<fdb_win_window *>(calloc(1, sizeof(fdb_win_window)));
    if (node) node->executable = static_cast<char *>(calloc(bytes+1, 1));
    if (!node || !node->executable) { free(node); list.allocationFailed = true; return FALSE; }
    WideCharToMultiByte(CP_UTF8, 0, path, length, node->executable, bytes, nullptr, nullptr);
    node->id = reinterpret_cast<uintptr_t>(window);
    node->pid = pid;
    node->width = rect.right-rect.left;
    node->height = rect.bottom-rect.top;
    node->next = list.head;
    list.head = node;
    return TRUE;
}
fdb_win_window *fdb_win_windows(char *error, size_t size) {
    error[0] = 0;
    WindowList list;
    if (!EnumWindows(enumerateWindow, reinterpret_cast<LPARAM>(&list))) {
        fdb_win_free_windows(list.head);
        fail(error, size, list.allocationFailed ? "Could not allocate game window metadata" : "Could not enumerate application windows");
        return nullptr;
    }
    return list.head;
}
void fdb_win_free_windows(fdb_win_window *node) {
    while (node) { auto next = node->next; free(node->executable); free(node); node = next; }
}

struct fdb_wgc {
    ComPtr<ID3D11Device> device;
    ComPtr<ID3D11DeviceContext> context;
    ComPtr<IInspectable> rtDevice;
    ComPtr<FdbItemInterop> itemFactory;
    ComPtr<FdbPoolStatics2> poolFactory;
    ComPtr<FdbCaptureItem> item;
    ComPtr<FdbFramePool> pool;
    ComPtr<FdbSession> session;
    ComPtr<FdbSession3> borderSession;
    ComPtr<FdbAccessOperation> borderAccess;
    ComPtr<IAsyncInfo> borderAccessInfo;
    bool borderRequested = false, borderAllowed = false;
    ComPtr<ID3D11Texture2D> latest, staging;
    HWND window = nullptr;
    DWORD pid = 0;
    FdbSize content{};
    ~fdb_wgc() {
        fdb_wgc_reset(this);
        if (borderAccessInfo.p) { borderAccessInfo->Cancel(); borderAccessInfo->Close(); }
    }
};
void fdb_wgc_reset(fdb_wgc *c) {
    c->borderSession.reset();
    closeObject(c->session.p); c->session.reset();
    closeObject(c->pool.p); c->pool.reset();
    c->item.reset(); c->latest.reset(); c->staging.reset();
    c->window = nullptr; c->pid = 0; c->content = {};
}
// Border suppression is optional. Never block frame acquisition on a consent
// dialog, or fail capture on older Windows versions / denied permission.
static void updateCaptureBorder(fdb_wgc *c) {
    if (!c->borderSession.p) return;
    if (!c->borderRequested) {
        c->borderRequested = true; // One request per reader, including window resets.
        ComPtr<FdbCaptureAccessStatics> access;
        HRESULT hr = factory(L"Windows.Graphics.Capture.GraphicsCaptureAccess", iidCaptureAccess, reinterpret_cast<void **>(access.put()));
        if (SUCCEEDED(hr)) hr = access->RequestAccessAsync(fdbAccessBorderless, c->borderAccess.put());
        if (SUCCEEDED(hr)) hr = c->borderAccess->QueryInterface(iidAsyncInfo, reinterpret_cast<void **>(c->borderAccessInfo.put()));
        if (FAILED(hr)) { c->borderAccess.reset(); c->borderAccessInfo.reset(); }
    }
    if (c->borderAccessInfo.p) {
        AsyncStatus status = Started;
        HRESULT hr = c->borderAccessInfo->get_Status(&status);
        if (SUCCEEDED(hr) && status == Started) return;
        INT32 result = 0;
        c->borderAllowed = SUCCEEDED(hr) && status == Completed &&
            SUCCEEDED(c->borderAccess->GetResults(&result)) && result == fdbAccessAllowed;
        if (FAILED(hr)) c->borderAccessInfo->Cancel();
        c->borderAccessInfo->Close();
        c->borderAccessInfo.reset(); c->borderAccess.reset();
    }
    if (c->borderAllowed) c->borderSession->SetBorderRequired(false);
    c->borderSession.reset(); // Apply once per session; a resize creates a new one.
}
fdb_wgc *fdb_wgc_open(char *error, size_t size) {
    HRESULT hr = RoInitialize(RO_INIT_MULTITHREADED);
    if (FAILED(hr)) { fail(error, size, "Initialize Windows Runtime", hr); return nullptr; }
    auto c = new(std::nothrow) fdb_wgc;
    if (!c) { RoUninitialize(); fail(error, size, "Could not allocate capture state"); return nullptr; }
    hr = factory(L"Windows.Graphics.Capture.GraphicsCaptureItem", iidItemInterop, reinterpret_cast<void **>(c->itemFactory.put()));
    if (SUCCEEDED(hr)) hr = factory(L"Windows.Graphics.Capture.Direct3D11CaptureFramePool", iidPoolStatics2, reinterpret_cast<void **>(c->poolFactory.put()));
    if (FAILED(hr)) { fail(error, size, "Window capture requires Windows 10 version 1903 or newer", hr); fdb_wgc_close(c); return nullptr; }
    hr = D3D11CreateDevice(nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr, D3D11_CREATE_DEVICE_BGRA_SUPPORT, nullptr, 0, D3D11_SDK_VERSION, c->device.put(), nullptr, c->context.put());
    if (FAILED(hr)) { fail(error, size, "Create Direct3D capture device", hr); fdb_wgc_close(c); return nullptr; }
    ComPtr<IDXGIDevice> dxgi;
    ComPtr<IInspectable> inspectable;
    hr = c->device->QueryInterface(IID_IDXGIDevice, reinterpret_cast<void **>(dxgi.put()));
    if (SUCCEEDED(hr)) hr = CreateDirect3D11DeviceFromDXGIDevice(dxgi.p, inspectable.put());
    if (SUCCEEDED(hr)) hr = inspectable->QueryInterface(iidRTDevice, reinterpret_cast<void **>(c->rtDevice.put()));
    if (FAILED(hr)) { fail(error, size, "Create WinRT capture device", hr); fdb_wgc_close(c); return nullptr; }
    return c;
}
void fdb_wgc_close(fdb_wgc *c) { delete c; RoUninitialize(); }

int fdb_wgc_select(fdb_wgc *c, uintptr_t id, uint32_t pid, int *width, int *height, char *error, size_t size) {
    HWND window = reinterpret_cast<HWND>(id);
    DWORD owner = 0;
    if (!eligible(window, &owner) || owner != pid) { fdb_wgc_reset(c); return fail(error, size, "Game window is no longer available"); }
    ComPtr<FdbCaptureItem> item;
    // The only source constructor in this backend takes a specific HWND.
    HRESULT hr = c->itemFactory->CreateForWindow(window, iidItem, reinterpret_cast<void **>(item.put()));
    FdbSize dimensions{};
    if (SUCCEEDED(hr)) hr = item->Size(&dimensions);
    if (FAILED(hr)) { fdb_wgc_reset(c); return fail(error, size, "Select game window", hr); }
    if (dimensions.width <= 0 || dimensions.height <= 0 || int64_t(dimensions.width)*dimensions.height > 100000000) {
        fdb_wgc_reset(c); return fail(error, size, "Invalid game window dimensions");
    }
    *width = dimensions.width; *height = dimensions.height;
    if (c->window == window && c->pid == pid && c->content.width == dimensions.width && c->content.height == dimensions.height) return 0;
    fdb_wgc_reset(c);
    c->item.reset(item.detach());
    c->content = dimensions;
    hr = c->poolFactory->CreateFreeThreaded(c->rtDevice.p, DXGI_FORMAT_B8G8R8A8_UNORM, 2, dimensions, c->pool.put());
    if (SUCCEEDED(hr)) hr = c->pool->CreateCaptureSession(c->item.p, c->session.put());
    if (SUCCEEDED(hr)) {
        ComPtr<FdbSession2> cursor;
        if (SUCCEEDED(c->session->QueryInterface(iidSession2, reinterpret_cast<void **>(cursor.put())))) cursor->SetCursorEnabled(false);
        c->session->QueryInterface(iidSession3, reinterpret_cast<void **>(c->borderSession.put()));
        updateCaptureBorder(c);
        hr = c->session->StartCapture();
    }
    if (FAILED(hr)) { fdb_wgc_reset(c); return fail(error, size, "Start game window capture", hr); }
    c->window = window; c->pid = pid;
    return 0;
}

static HRESULT cacheFrame(fdb_wgc *c, FdbFrame *frame) {
    FdbSize dimensions{};
    HRESULT hr = frame->ContentSize(&dimensions);
    if (FAILED(hr)) return hr;
    if (dimensions.width != c->content.width || dimensions.height != c->content.height) return HRESULT_FROM_WIN32(ERROR_RETRY);
    ComPtr<IInspectable> surface;
    ComPtr<FdbDxgiAccess> access;
    ComPtr<ID3D11Texture2D> texture;
    hr = frame->Surface(surface.put());
    if (SUCCEEDED(hr)) hr = surface->QueryInterface(iidDxgiAccess, reinterpret_cast<void **>(access.put()));
    if (SUCCEEDED(hr)) hr = access->GetInterface(IID_ID3D11Texture2D, reinterpret_cast<void **>(texture.put()));
    if (FAILED(hr)) return hr;
    D3D11_TEXTURE2D_DESC desc{};
    texture->GetDesc(&desc);
    if (desc.Format != DXGI_FORMAT_B8G8R8A8_UNORM || desc.Width < UINT(dimensions.width) || desc.Height < UINT(dimensions.height)) return E_FAIL;
    D3D11_TEXTURE2D_DESC previous{};
    if (c->latest.p) c->latest->GetDesc(&previous);
    if (!c->latest.p || previous.Width != desc.Width || previous.Height != desc.Height) {
        desc.Usage = D3D11_USAGE_DEFAULT;
        desc.BindFlags = desc.CPUAccessFlags = desc.MiscFlags = 0;
        hr = c->device->CreateTexture2D(&desc, nullptr, c->latest.put());
        if (FAILED(hr)) return hr;
    }
    c->context->CopyResource(c->latest.p, texture.p);
    return S_OK;
}
int fdb_wgc_capture(fdb_wgc *c, int x, int y, int width, int height, void *rgba, char *error, size_t size) {
    DWORD owner = 0;
    if (!c->window || !eligible(c->window, &owner) || owner != c->pid) { fdb_wgc_reset(c); return fail(error, size, "Game window is closed, minimized, or unavailable"); }
    if (x < 0 || y < 0 || width <= 0 || height <= 0 || int64_t(x)+width > c->content.width || int64_t(y)+height > c->content.height) return fail(error, size, "Capture rectangle is outside the game window");
    updateCaptureBorder(c);
    ULONGLONG deadline = GetTickCount64()+1000;
    int received = 0;
    do {
        FramePtr frame;
        HRESULT hr = c->pool->TryGetNextFrame(frame.put());
        if (FAILED(hr)) { fdb_wgc_reset(c); return fail(error, size, "Read game window frame", hr); }
        if (frame.p) {
            hr = cacheFrame(c, frame.p);
            // Return the frame to the pool before resetting a failed session.
            closeObject(frame.p); frame.reset();
            if (FAILED(hr)) { fdb_wgc_reset(c); return fail(error, size, "Game frame changed or graphics device failed; reacquiring window", hr); }
            if (++received < 4) continue;
        }
        if (c->latest.p) break; // Static game content need not generate new frames.
        Sleep(5);
    } while (GetTickCount64() < deadline);
    if (!c->latest.p) return fail(error, size, "No game frames yet; restore WoW and use windowed or borderless mode");
    D3D11_TEXTURE2D_DESC desc{};
    if (c->staging.p) c->staging->GetDesc(&desc);
    HRESULT hr = S_OK;
    if (!c->staging.p || desc.Width != UINT(width) || desc.Height != UINT(height)) {
        desc = {};
        desc.Width = width; desc.Height = height; desc.MipLevels = desc.ArraySize = 1;
        desc.Format = DXGI_FORMAT_B8G8R8A8_UNORM; desc.SampleDesc.Count = 1;
        desc.Usage = D3D11_USAGE_STAGING; desc.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
        hr = c->device->CreateTexture2D(&desc, nullptr, c->staging.put());
    }
    if (FAILED(hr)) return fail(error, size, "Allocate game frame readback", hr);
    D3D11_BOX box{UINT(x), UINT(y), 0, UINT(x+width), UINT(y+height), 1};
    c->context->CopySubresourceRegion(c->staging.p, 0, 0, 0, 0, c->latest.p, 0, &box);
    D3D11_MAPPED_SUBRESOURCE mapped{};
    hr = c->context->Map(c->staging.p, 0, D3D11_MAP_READ, 0, &mapped);
    if (FAILED(hr)) { fdb_wgc_reset(c); return fail(error, size, "Read game pixels", hr); }
    auto dest = static_cast<unsigned char *>(rgba);
    for (int row = 0; row < height; ++row) {
        auto src = static_cast<unsigned char *>(mapped.pData)+size_t(row)*mapped.RowPitch;
        for (int col = 0; col < width; ++col) {
            *dest++ = src[2]; *dest++ = src[1]; *dest++ = src[0]; *dest++ = 255;
            src += 4;
        }
    }
    c->context->Unmap(c->staging.p, 0);
    return 0;
}
