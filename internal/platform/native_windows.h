#ifndef FDB_NATIVE_WINDOWS_H
#define FDB_NATIVE_WINDOWS_H
#include <stdint.h>
#include <stddef.h>
#ifdef __cplusplus
extern "C" {
#endif

typedef struct fdb_win_window {
    struct fdb_win_window *next;
    uintptr_t id;
    uint32_t pid;
    int width, height;
    char *executable;
} fdb_win_window;
fdb_win_window *fdb_win_windows(char *error, size_t size);
void fdb_win_free_windows(fdb_win_window *windows);

typedef struct fdb_wgc fdb_wgc;
// All WGC functions run on the same dedicated OS thread (COM MTA).
fdb_wgc *fdb_wgc_open(char *error, size_t size);
void fdb_wgc_close(fdb_wgc *capture);
void fdb_wgc_reset(fdb_wgc *capture);
int fdb_wgc_select(fdb_wgc *capture, uintptr_t window, uint32_t pid, int *width, int *height, char *error, size_t size);
int fdb_wgc_capture(fdb_wgc *capture, int x, int y, int width, int height, void *rgba, char *error, size_t size);
#ifdef __cplusplus
}
#endif
#endif
