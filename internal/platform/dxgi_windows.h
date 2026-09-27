#ifndef FDB_DXGI_WINDOWS_H
#define FDB_DXGI_WINDOWS_H
#include <windows.h>
#include <cstddef>
struct fdb_dxgi;
fdb_dxgi *fdb_dxgi_open();
void fdb_dxgi_close(fdb_dxgi *capture);
void fdb_dxgi_reset(fdb_dxgi *capture);
void fdb_dxgi_poll(fdb_dxgi *capture);
int fdb_dxgi_select(fdb_dxgi *capture, HWND window, DWORD pid, int *width, int *height,
                    char *error, size_t size);
int fdb_dxgi_capture(fdb_dxgi *capture, int x, int y, int width, int height, void *rgba,
                     char *error, size_t size);
#endif
