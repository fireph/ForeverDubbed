#pragma once

#include <nlohmann/json.hpp>
#include <filesystem>
#include "pocket_tts.hpp"

// The pinned April export stores six attention layers in safetensors files.
// Keep its layout checks together so a model change cannot silently reuse an
// incompatible voice state. This adapter is the runtime's only friend class.
namespace pocket_tts {
struct ForeverDubbedAccess {
    static void voice(PocketTTS &tts, const std::string &filename) {
        std::ifstream input(std::filesystem::u8path(filename), std::ios::binary | std::ios::ate);
        if (!input)
            throw std::runtime_error("Cannot open voice: " + filename);
        auto length = input.tellg();
        if (length < 8 || length > 128 * 1024 * 1024)
            throw std::runtime_error("Invalid voice file size");
        input.seekg(0);
        uint64_t header_size = 0;
        input.read(reinterpret_cast<char *>(&header_size), 8);
        if (header_size > 1024 * 1024 || header_size + 8 > uint64_t(length))
            throw std::runtime_error("Invalid safetensors header");
        std::string header(header_size, '\0');
        input.read(header.data(), header.size());
        auto tensors = nlohmann::json::parse(header);
        auto read = [&](const std::string &key, const std::string &dtype,
                        const std::vector<int64_t> &shape, void *dest, size_t bytes) {
            const auto &tensor = tensors.at(key);
            auto offsets = tensor.at("data_offsets").get<std::vector<uint64_t>>();
            if (tensor.at("dtype") != dtype ||
                tensor.at("shape").get<std::vector<int64_t>>() != shape || offsets.size() != 2 ||
                offsets[1] < offsets[0] || offsets[1] - offsets[0] != bytes ||
                offsets[1] > uint64_t(length) - 8 - header_size)
                throw std::runtime_error("Incompatible voice tensor: " + key);
            input.seekg(8 + header_size + offsets[0]);
            input.read(static_cast<char *>(dest), bytes);
            if (!input)
                throw std::runtime_error("Truncated voice tensor: " + key);
        };
        tts.main_runner_->reinit();
        auto &state = tts.main_runner_->state();
        if (state.names.size() != 18)
            throw std::runtime_error("Expected April English six-layer model state");
        int64_t voice_length = -1;
        for (size_t layer = 0; layer < 6; ++layer) {
            std::string prefix = "transformer.layers." + std::to_string(layer) + ".self_attn/";
            int64_t offset = 0, pad = 0;
            read(prefix + "offset", "I64", {1}, &offset, 8);
            if (tensors.contains(prefix + "pad"))
                read(prefix + "pad", "I64", {1}, &pad, 8);
            if (offset < 1 || offset > 500 || pad != 0 ||
                (voice_length != -1 && offset != voice_length))
                throw std::runtime_error("Unsupported voice offset/padding");
            voice_length = offset;
            size_t i = layer * 3;
            if (state.init_shapes[i] != std::vector<int64_t>({2, 1, 1000, 16, 64}) ||
                state.types[i] != ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT ||
                state.types[i + 2] != ONNX_TENSOR_ELEMENT_DATA_TYPE_INT64)
                throw std::runtime_error("Incompatible ONNX state layout");
            auto shape = tensors.at(prefix + "cache").at("shape").get<std::vector<int64_t>>();
            if (shape.size() != 5 || shape[0] != 2 || shape[1] != 1 || shape[2] < offset ||
                shape[2] > 1000 || shape[3] != 16 || shape[4] != 64)
                throw std::runtime_error("Invalid voice cache shape");
            std::vector<float> cache(2 * shape[2] * 1024);
            read(prefix + "cache", "F32", shape, cache.data(), cache.size() * 4);
            for (int kv = 0; kv < 2; ++kv) {
                const float *begin = cache.data() + kv * shape[2] * 1024;
                if (!std::all_of(begin, begin + offset * 1024,
                                 [](float v) { return std::isfinite(v); }))
                    throw std::runtime_error("Non-finite voice state");
                std::copy(begin, begin + offset * 1024, state.f32[0][i].begin() + kv * 1000 * 1024);
            }
            // The exported current_end input is an unused, fixed empty tensor.
            state.shapes[i + 1] = {0};
            state.f32[0][i + 1].clear();
            state.i64[0][i + 2] = {offset};
        }
        tts.voice_kv_snap_ =
            std::make_unique<PocketTTS::VoiceKVSnapshot>(tts.main_runner_->take_snapshot());
        Tensor dummy({1, 1, 1024});
        tts.voice_kv_hash_ = tts.voice_hash(dummy);
    }
    static void fades(PocketTTS &tts, int fade_in_ms, int fade_out_ms) {
        if (fade_in_ms < 0 || fade_in_ms > 500 || fade_out_ms < 0 || fade_out_ms > 500)
            throw std::runtime_error("Fade durations must be 0..500 milliseconds");
        tts.cfg_.fade_in_ms = fade_in_ms;
        tts.cfg_.fade_out_ms = fade_out_ms;
    }
    static void steps(PocketTTS &tts, int steps) {
        if (steps < 1 || steps > 64)
            throw std::runtime_error("Decode steps must be 1..64");
        tts.cfg_.lsd_steps = steps;
        tts.dt_ = 1.0f / steps;
        tts.st_values_.clear();
        for (int i = 0; i < steps; ++i)
            tts.st_values_.emplace_back(float(i) / steps, float(i + 1) / steps);
    }
};
} // namespace pocket_tts
