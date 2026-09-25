//go:build darwin && cgo

#import <AudioToolbox/AudioToolbox.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>
#include "native_darwin.h"

struct fdb_audio {
    AudioQueueRef queue;
    atomic_int pending;
    int started;
};
static void audio_done(void *user, AudioQueueRef queue, AudioQueueBufferRef buffer) {
    fdb_audio *device = user;
    AudioQueueFreeBuffer(queue, buffer);
    atomic_fetch_sub(&device->pending, 1);
}
fdb_audio *fdb_audio_open(int rate, int *status) {
    fdb_audio *device = calloc(1, sizeof(*device));
    if (!device) {
        *status = -108;
        return NULL;
    }
    atomic_init(&device->pending, 0);
    AudioStreamBasicDescription format = {0};
    format.mSampleRate = rate;
    format.mFormatID = kAudioFormatLinearPCM;
    format.mFormatFlags = kLinearPCMFormatFlagIsSignedInteger | kLinearPCMFormatFlagIsPacked;
    format.mBytesPerPacket = format.mBytesPerFrame = 2;
    format.mFramesPerPacket = format.mChannelsPerFrame = 1;
    format.mBitsPerChannel = 16;
    *status = AudioQueueNewOutput(&format, audio_done, device, NULL, NULL, 0, &device->queue);
    if (*status) {
        free(device);
        return NULL;
    }
    return device;
}
int fdb_audio_queue(fdb_audio *device, const void *data, int size) {
    AudioQueueBufferRef buffer;
    OSStatus status = AudioQueueAllocateBuffer(device->queue, size, &buffer);
    if (status)
        return status;
    memcpy(buffer->mAudioData, data, size);
    buffer->mAudioDataByteSize = size;
    atomic_fetch_add(&device->pending, 1);
    status = AudioQueueEnqueueBuffer(device->queue, buffer, 0, NULL);
    if (status) {
        atomic_fetch_sub(&device->pending, 1);
        AudioQueueFreeBuffer(device->queue, buffer);
        return status;
    }
    if (!device->started) {
        status = AudioQueueStart(device->queue, NULL);
        if (!status)
            device->started = 1;
    }
    return status;
}
int fdb_audio_pending(fdb_audio *device) {
    return atomic_load(&device->pending);
}
int fdb_audio_close(fdb_audio *device) {
    // Immediate disposal stops playback, waits for callbacks and releases all
    // buffers, including those still queued when speech was interrupted.
    OSStatus status = AudioQueueDispose(device->queue, true);
    if (!status)
        free(device); // Never free callback state if disposal failed.
    return status;
}

int fdb_audio_drain(fdb_audio *device) {
    return AudioQueueStop(device->queue, false);
}
int fdb_audio_running(fdb_audio *device, unsigned int *running) {
    UInt32 size = sizeof(*running);
    return AudioQueueGetProperty(device->queue, kAudioQueueProperty_IsRunning, running, &size);
}
