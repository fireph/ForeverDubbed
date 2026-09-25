#ifndef FDB_WGC_ABI_WINDOWS_H
#define FDB_WGC_ABI_WINDOWS_H
// Minimal Windows Runtime ABI declarations for MinGW, which lacks the projected
// Windows.Graphics.Capture headers. These are interface layouts, not a WinRT
// implementation. GUIDs/method order follow Microsoft's Windows metadata:
// https://github.com/microsoft/windows-rs/tree/master/crates/libs/windows/src/Windows/Graphics/Capture
// Interop: https://learn.microsoft.com/windows/win32/api/windows.graphics.capture.interop/
#include <inspectable.h>
#include <asyncinfo.h>

struct FdbSize {
    INT32 width, height;
};
struct FdbCaptureItem : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE DisplayName(HSTRING *) = 0;
    virtual HRESULT STDMETHODCALLTYPE Size(FdbSize *) = 0;
};
struct FdbItemInterop : IUnknown {
    virtual HRESULT STDMETHODCALLTYPE CreateForWindow(HWND, REFIID, void **) = 0;
    // CreateForMonitor is deliberately not exposed by this backend.
};
struct FdbSession : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE StartCapture() = 0;
};
struct FdbSession2 : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE CursorEnabled(boolean *) = 0;
    virtual HRESULT STDMETHODCALLTYPE SetCursorEnabled(boolean) = 0;
};
struct FdbSession3 : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE BorderRequired(boolean *) = 0;
    virtual HRESULT STDMETHODCALLTYPE SetBorderRequired(boolean) = 0;
};
// IAsyncOperation<AppCapabilityAccessStatus>; the enum result is a signed int32.
struct FdbAccessOperation : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE SetCompleted(IUnknown *) = 0;
    virtual HRESULT STDMETHODCALLTYPE Completed(IUnknown **) = 0;
    virtual HRESULT STDMETHODCALLTYPE GetResults(INT32 *) = 0;
};
struct FdbCaptureAccessStatics : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE RequestAccessAsync(INT32, FdbAccessOperation **) = 0;
};
static constexpr INT32 fdbAccessBorderless = 0;
static constexpr INT32 fdbAccessAllowed = 4;
struct FdbFrame : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE Surface(IInspectable **) = 0;
    virtual HRESULT STDMETHODCALLTYPE SystemRelativeTime(INT64 *) = 0;
    virtual HRESULT STDMETHODCALLTYPE ContentSize(FdbSize *) = 0;
};
struct FdbFramePool : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE Recreate(IInspectable *, INT32, INT32, FdbSize) = 0;
    virtual HRESULT STDMETHODCALLTYPE TryGetNextFrame(FdbFrame **) = 0;
    virtual HRESULT STDMETHODCALLTYPE FrameArrived(IUnknown *, INT64 *) = 0;
    virtual HRESULT STDMETHODCALLTYPE RemoveFrameArrived(INT64) = 0;
    virtual HRESULT STDMETHODCALLTYPE CreateCaptureSession(FdbCaptureItem *, FdbSession **) = 0;
};
struct FdbPoolStatics2 : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE CreateFreeThreaded(IInspectable *, INT32, INT32, FdbSize,
                                                         FdbFramePool **) = 0;
};
struct FdbClosable : IInspectable {
    virtual HRESULT STDMETHODCALLTYPE Close() = 0;
};
struct FdbDxgiAccess : IUnknown {
    virtual HRESULT STDMETHODCALLTYPE GetInterface(REFIID, void **) = 0;
};
static const GUID iidItem = {
    0x79c3f95b, 0x31f7, 0x4ec2, {0xa4, 0x64, 0x63, 0x2e, 0xf5, 0xd3, 0x07, 0x60}};
static const GUID iidItemInterop = {
    0x3628e81b, 0x3cac, 0x4c60, {0xb7, 0xf4, 0x23, 0xce, 0x0e, 0x0c, 0x33, 0x56}};
static const GUID iidPoolStatics2 = {
    0x589b103f, 0x6bbc, 0x5df5, {0xa9, 0x91, 0x02, 0xe2, 0x8b, 0x3b, 0x66, 0xd5}};
static const GUID iidSession2 = {
    0x2c39ae40, 0x7d2e, 0x5044, {0x80, 0x4e, 0x8b, 0x67, 0x99, 0xd4, 0xcf, 0x9e}};
static const GUID iidSession3 = {
    0xf2cdd966, 0x22ae, 0x5ea1, {0x95, 0x96, 0x3a, 0x28, 0x93, 0x44, 0xc3, 0xbe}};
static const GUID iidCaptureAccess = {
    0x743ed370, 0x06ec, 0x5040, {0xa5, 0x8a, 0x90, 0x1f, 0x0f, 0x75, 0x70, 0x95}};
static const GUID iidAsyncInfo = {
    0x00000036, 0x0000, 0x0000, {0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}};
static const GUID iidClosable = {
    0x30d5a829, 0x7fa4, 0x4026, {0x83, 0xbb, 0xd7, 0x5b, 0xae, 0x4e, 0xa9, 0x9e}};
static const GUID iidDxgiAccess = {
    0xa9b3d012, 0x3df2, 0x4ee3, {0xb8, 0xd1, 0x86, 0x95, 0xf4, 0x57, 0xd3, 0xc1}};
static const GUID iidRTDevice = {
    0xa37624ab, 0x8d5f, 0x4650, {0x9d, 0x3e, 0x9e, 0xae, 0x3d, 0x9b, 0xc6, 0x70}};
#endif
