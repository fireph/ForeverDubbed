//go:build pocket_native && cgo && (windows || linux || darwin)

// ForeverDubbed ABI around the pinned PocketTTS.cpp engine. No Python or HTTP.
#include "voice_state.hpp"

#include "bridge.h"
#define FDB_EXPORT extern "C"

namespace {
// Match the Go playback queue: at most four buffers of 100 ms at 24 kHz.
constexpr size_t max_pcm_samples = 2400;
constexpr size_t max_queued_buffers = 4;

struct Engine {
    std::unique_ptr<pocket_tts::PocketTTS> tts;
    std::thread worker;
    std::mutex mutex;
    std::condition_variable cv;
    std::deque<std::vector<int16_t>> queue;
    std::string error, last_voice;
    bool done = true, cancelled = false;
    // Cancellation wakes a blocked producer before stop() joins its worker.
    bool enqueue(const float *data, size_t count) {
        for (size_t pos = 0; pos < count;) {
            size_t n = std::min(max_pcm_samples, count - pos);
            std::vector<int16_t> pcm(n);
            for (size_t i = 0; i < n; ++i) {
                float value = data[pos + i];
                if (!std::isfinite(value))
                    throw std::runtime_error("Non-finite generated audio");
                pcm[i] = int16_t(std::clamp(value, -1.0f, 1.0f) * 32767);
            }
            std::unique_lock<std::mutex> lock(mutex);
            cv.wait(lock, [this] { return cancelled || queue.size() < max_queued_buffers; });
            if (cancelled)
                return false;
            queue.push_back(std::move(pcm));
            pos += n;
        }
        return true;
    }

    void stop() {
        {
            std::lock_guard<std::mutex> lock(mutex);
            cancelled = true;
        }
        cv.notify_all();
        if (worker.joinable())
            worker.join();
        queue.clear();
    }
    ~Engine() {
        stop();
    }
};
} // namespace

FDB_EXPORT void *fdb_create(const char *models, int threads, char *error, int capacity) {
    try {
        pocket_tts::Config config;
        config.models_dir = models;
        config.tokenizer_path = std::string(models) + "/tokenizer.model";
        config.num_threads = threads;
        config.temperature = 0.3f;
        config.voice_cache = false;
        // Decode 15 latent frames (1.2 seconds of audio) together, including
        // the first batch. The callback below still splits PCM into <=100 ms
        // buffers for bounded playback queues and responsive cancellation.
        config.first_chunk_frames = 15;
        config.max_chunk_frames = 15;
        auto engine = std::make_unique<Engine>();
        engine->tts = std::make_unique<pocket_tts::PocketTTS>(config);
        return engine.release();
    } catch (const std::exception &e) {
        if (capacity > 0)
            std::snprintf(error, capacity, "%s", e.what());
        return nullptr;
    }
}
FDB_EXPORT int fdb_start(void *handle, const char *text, const char *voice, int steps,
                         int fade_in_ms, int fade_out_ms) {
    auto &e = *static_cast<Engine *>(handle);
    e.stop();
    e.cancelled = false;
    e.done = false;
    e.error.clear();
    try {
        std::string prompt(text), filename(voice);
        e.worker = std::thread([&e, prompt, filename, steps, fade_in_ms, fade_out_ms] {
            try {
                pocket_tts::ForeverDubbedAccess::steps(*e.tts, steps);
                pocket_tts::ForeverDubbedAccess::fades(*e.tts, fade_in_ms, fade_out_ms);
                if (e.last_voice != filename) {
                    pocket_tts::ForeverDubbedAccess::voice(*e.tts, filename);
                    e.last_voice = filename;
                }
                pocket_tts::Tensor dummy({1, 1, 1024});
                e.tts->stream(
                    prompt, dummy,
                    [&e](const float *data, size_t count) { return e.enqueue(data, count); }, 450);
            } catch (const std::exception &ex) {
                std::lock_guard<std::mutex> lock(e.mutex);
                e.error = ex.what();
            }
            {
                std::lock_guard<std::mutex> lock(e.mutex);
                e.done = true;
            }
        });
        return 0;
    } catch (const std::exception &ex) {
        e.error = ex.what();
        e.done = true;
        return -1;
    }
}
// Nonblocking polling allows Go cancellation while the model is computing.
FDB_EXPORT int fdb_read(void *handle, int16_t *output, int capacity) {
    auto &e = *static_cast<Engine *>(handle);
    std::lock_guard<std::mutex> lock(e.mutex);
    if (!e.error.empty())
        return -2;
    if (e.queue.empty())
        return e.done ? -1 : 0;
    auto &pcm = e.queue.front();
    if (capacity < int(pcm.size())) {
        e.error = "PCM buffer too small";
        return -2;
    }
    int n = int(pcm.size());
    std::copy(pcm.begin(), pcm.end(), output);
    e.queue.pop_front();
    e.cv.notify_all();
    return n;
}
FDB_EXPORT void fdb_stop(void *handle) {
    static_cast<Engine *>(handle)->stop();
}
FDB_EXPORT void fdb_error(void *handle, char *output, int size) {
    auto &e = *static_cast<Engine *>(handle);
    std::lock_guard<std::mutex> lock(e.mutex);
    if (size > 0)
        std::snprintf(output, size, "%s", e.error.c_str());
}
FDB_EXPORT void fdb_destroy(void *handle) {
    delete static_cast<Engine *>(handle);
}
