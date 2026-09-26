#pragma once
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
void *fdb_create(const char *models, int threads, char *error, int capacity);
int fdb_start(void *handle, const char *text, const char *voice, int steps, int fade_in_ms,
              int fade_out_ms);
int fdb_read(void *handle, int16_t *output, int capacity);
void fdb_stop(void *handle);
void fdb_error(void *handle, char *output, int size);
void fdb_destroy(void *handle);
#ifdef __cplusplus
}
#endif
