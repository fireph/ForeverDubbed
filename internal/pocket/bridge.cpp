//go:build pocket_native && cgo && (windows || linux || darwin)

// ForeverDubbed ABI around the pinned PocketTTS.cpp engine. No Python or HTTP.
#include <nlohmann/json.hpp>
#include <filesystem>
#include "pocket_tts.hpp"

#include "bridge.h"
#define FDB_EXPORT extern "C"

namespace pocket_tts {
struct ForeverDubbedAccess {
    static void voice(PocketTTS& tts, const std::string& filename) {
        std::ifstream input(std::filesystem::u8path(filename), std::ios::binary | std::ios::ate);
        if (!input) throw std::runtime_error("Cannot open voice: " + filename);
        auto length = input.tellg();
        if (length < 8 || length > 128*1024*1024) throw std::runtime_error("Invalid voice file size");
        input.seekg(0);
        uint64_t header_size = 0;
        input.read(reinterpret_cast<char*>(&header_size), 8);
        if (header_size > 1024*1024 || header_size + 8 > uint64_t(length)) throw std::runtime_error("Invalid safetensors header");
        std::string header(header_size, '\0');
        input.read(header.data(), header.size());
        auto tensors = nlohmann::json::parse(header);
        auto read = [&](const std::string& key, const std::string& dtype, const std::vector<int64_t>& shape, void* dest, size_t bytes) {
            const auto& tensor = tensors.at(key);
            auto offsets = tensor.at("data_offsets").get<std::vector<uint64_t>>();
            if (tensor.at("dtype") != dtype || tensor.at("shape").get<std::vector<int64_t>>() != shape || offsets.size()!=2 || offsets[1]<offsets[0] || offsets[1]-offsets[0]!=bytes || offsets[1]>uint64_t(length)-8-header_size)
                throw std::runtime_error("Incompatible voice tensor: " + key);
            input.seekg(8 + header_size + offsets[0]);
            input.read(static_cast<char*>(dest), bytes);
            if (!input) throw std::runtime_error("Truncated voice tensor: " + key);
        };
        tts.main_runner_->reinit();
        auto& state = tts.main_runner_->state();
        if (state.names.size()!=18) throw std::runtime_error("Expected April English six-layer model state");
        int64_t voice_length = -1;
        for (size_t layer=0; layer<6; ++layer) {
            std::string prefix = "transformer.layers." + std::to_string(layer) + ".self_attn/";
            int64_t offset=0, pad=0;
            read(prefix+"offset", "I64", {1}, &offset, 8);
            if (tensors.contains(prefix+"pad")) read(prefix+"pad", "I64", {1}, &pad, 8);
            if (offset<1 || offset>500 || pad!=0 || (voice_length!=-1 && offset!=voice_length)) throw std::runtime_error("Unsupported voice offset/padding");
            voice_length = offset;
            size_t i=layer*3;
            if (state.init_shapes[i] != std::vector<int64_t>({2,1,1000,16,64}) || state.types[i]!=ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT || state.types[i+2]!=ONNX_TENSOR_ELEMENT_DATA_TYPE_INT64)
                throw std::runtime_error("Incompatible ONNX state layout");
            auto shape=tensors.at(prefix+"cache").at("shape").get<std::vector<int64_t>>();
            if (shape.size()!=5 || shape[0]!=2 || shape[1]!=1 || shape[2]<offset || shape[2]>1000 || shape[3]!=16 || shape[4]!=64) throw std::runtime_error("Invalid voice cache shape");
            std::vector<float> cache(2*shape[2]*1024);
            read(prefix+"cache", "F32", shape, cache.data(), cache.size()*4);
            for (int kv=0; kv<2; ++kv) {
                const float* begin=cache.data()+kv*shape[2]*1024;
                if (!std::all_of(begin, begin+offset*1024, [](float v){return std::isfinite(v);})) throw std::runtime_error("Non-finite voice state");
                std::copy(begin, begin+offset*1024, state.f32[0][i].begin()+kv*1000*1024);
            }
            // The exported current_end input is an unused, fixed empty tensor.
            state.shapes[i+1]={0};
            state.f32[0][i+1].clear();
            state.i64[0][i+2]={offset};
        }
        tts.voice_kv_snap_=std::make_unique<PocketTTS::VoiceKVSnapshot>(tts.main_runner_->take_snapshot());
        Tensor dummy({1,1,1024});
        tts.voice_kv_hash_=tts.voice_hash(dummy);
    }
    static void steps(PocketTTS& tts, int steps) {
        if (steps<1 || steps>64) throw std::runtime_error("Decode steps must be 1..64");
        tts.cfg_.lsd_steps=steps;
        tts.dt_=1.0f/steps;
        tts.st_values_.clear();
        for(int i=0;i<steps;++i) tts.st_values_.emplace_back(float(i)/steps,float(i+1)/steps);
    }
};
}

struct Engine {
    std::unique_ptr<pocket_tts::PocketTTS> tts;
    std::thread worker;
    std::mutex mutex;
    std::condition_variable cv;
    std::deque<std::vector<int16_t>> queue;
    std::string error, last_voice;
    bool done=true, cancelled=false;
    void stop() {
        { std::lock_guard<std::mutex> lock(mutex); cancelled=true; }
        cv.notify_all();
        if(worker.joinable()) worker.join();
        queue.clear();
    }
    ~Engine(){stop();}
};

FDB_EXPORT void* fdb_create(const char* models, int threads, char* error, int capacity) {
    try {
        pocket_tts::Config config;
        config.models_dir=models;
        config.tokenizer_path=std::string(models)+"/tokenizer.model";
        config.num_threads=threads;
        config.temperature=0.3f;
        config.voice_cache=false;
        // Decode 15 latent frames (1.2 seconds of audio) together, including
        // the first batch. The callback below still splits PCM into <=100 ms
        // buffers for bounded playback queues and responsive cancellation.
        config.first_chunk_frames=15;
        config.max_chunk_frames=15;
        auto engine=std::make_unique<Engine>();
        engine->tts=std::make_unique<pocket_tts::PocketTTS>(config);
        return engine.release();
    } catch(const std::exception& e) {
        if(capacity>0) std::snprintf(error,capacity,"%s",e.what());
        return nullptr;
    }
}
FDB_EXPORT int fdb_start(void* handle, const char* text, const char* voice, int steps) {
    auto& e=*static_cast<Engine*>(handle);
    e.stop(); e.cancelled=false; e.done=false; e.error.clear();
    try {
        std::string prompt(text), filename(voice);
        e.worker=std::thread([&e,prompt,filename,steps] {
            try {
                pocket_tts::ForeverDubbedAccess::steps(*e.tts,steps);
                if(e.last_voice!=filename) {
                    pocket_tts::ForeverDubbedAccess::voice(*e.tts,filename);
                    e.last_voice=filename;
                }
                pocket_tts::Tensor dummy({1,1,1024});
                e.tts->stream(prompt,dummy,[&e](const float* data,size_t count) {
                    for(size_t pos=0;pos<count;) {
                        size_t n=std::min(size_t(2400),count-pos);
                        std::vector<int16_t> pcm(n);
                        for(size_t i=0;i<n;++i) {
                            float value=data[pos+i];
                            if(!std::isfinite(value)) throw std::runtime_error("Non-finite generated audio");
                            pcm[i]=int16_t(std::clamp(value,-1.0f,1.0f)*32767);
                        }
                        std::unique_lock<std::mutex> lock(e.mutex);
                        e.cv.wait(lock,[&e]{return e.cancelled || e.queue.size()<4;});
                        if(e.cancelled) return false;
                        e.queue.push_back(std::move(pcm));
                        pos+=n;
                    }
                    return true;
                },450);
            } catch(const std::exception& ex) { std::lock_guard<std::mutex> lock(e.mutex);e.error=ex.what(); }
            {std::lock_guard<std::mutex> lock(e.mutex);e.done=true;}
        });
        return 0;
    } catch(const std::exception& ex) { e.error=ex.what();e.done=true;return -1; }
}
// Nonblocking polling allows Go cancellation while the model is computing.
FDB_EXPORT int fdb_read(void* handle, int16_t* output, int capacity) {
    auto& e=*static_cast<Engine*>(handle);
    std::lock_guard<std::mutex> lock(e.mutex);
    if(!e.error.empty()) return -2;
    if(e.queue.empty()) return e.done ? -1 : 0;
    auto& pcm=e.queue.front();
    if(capacity<int(pcm.size())) {e.error="PCM buffer too small";return -2;}
    int n=int(pcm.size());std::copy(pcm.begin(),pcm.end(),output);
    e.queue.pop_front();e.cv.notify_all();return n;
}
FDB_EXPORT void fdb_stop(void* handle) { static_cast<Engine*>(handle)->stop(); }
FDB_EXPORT void fdb_error(void* handle,char* output,int size) { auto& e=*static_cast<Engine*>(handle);std::lock_guard<std::mutex> lock(e.mutex);if(size>0) std::snprintf(output,size,"%s",e.error.c_str()); }
FDB_EXPORT void fdb_destroy(void* handle) { delete static_cast<Engine*>(handle); }
