#include <stddef.h>
#include <stdint.h>
int fdb_screen_init(char *error, size_t size);
int fdb_window_list(char **json, char *error, size_t size);
int fdb_window_select(uint32_t window_id, int32_t pid, int *width, int *height, char *error, size_t size);
int fdb_window_capture(int x, int y, int width, int height, void *pixels, char *error, size_t size);
typedef struct fdb_audio fdb_audio;
fdb_audio *fdb_audio_open(int rate, int *status);
int fdb_audio_queue(fdb_audio *device, const void *data, int size);
int fdb_audio_pending(fdb_audio *device);
int fdb_audio_close(fdb_audio *device);

int fdb_audio_drain(fdb_audio *device);
int fdb_audio_running(fdb_audio *device, unsigned int *running);
