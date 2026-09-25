#pragma once

#include <algorithm>
#include <cmath>
#include <cstddef>
#include <vector>

namespace pocket_tts {

// Retain only the fade-out tail. Decoder batch boundaries do not restart the
// envelope; each generated sentence gets its own instance.
class SentenceFade {
    size_t fade_in_, fade_out_, seen_ = 0;
    std::vector<float> tail_;

public:
    SentenceFade(size_t fade_in, size_t fade_out)
        : fade_in_(fade_in), fade_out_(fade_out) {}

    template<class Callback>
    bool push(const float* data, size_t count, Callback&& emit) {
        if (count == 0) return true;
        if (fade_in_ == 0 && fade_out_ == 0) return emit(data, count);
        const size_t offset = tail_.size();
        tail_.insert(tail_.end(), data, data + count);
        for (size_t i = 0; i < count && seen_ + i < fade_in_; ++i) {
            const float gain = fade_in_ <= 1 ? 0.0f :
                0.5f * (1.0f - std::cos(3.14159265358979323846 * (seen_ + i) / (fade_in_ - 1)));
            tail_[offset + i] *= gain;
        }
        seen_ += count;
        if (tail_.size() > fade_out_) {
            const size_t ready = tail_.size() - fade_out_;
            if (!emit(tail_.data(), ready)) return false;
            tail_.erase(tail_.begin(), tail_.begin() + ready);
        }
        return true;
    }

    template<class Callback>
    bool finish(Callback&& emit) {
        if (tail_.empty()) return true;
        for (size_t i = 0; i < tail_.size(); ++i) {
            const float gain = tail_.size() <= 1 ? 0.0f :
                0.5f * (1.0f + std::cos(3.14159265358979323846 * i / (tail_.size() - 1)));
            tail_[i] *= gain;
        }
        const bool ok = emit(tail_.data(), tail_.size());
        tail_.clear();
        return ok;
    }
};
}
