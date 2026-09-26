#include "pocket_tts.hpp"
#include <cassert>
#include <stdexcept>

using pocket_tts::SentenceFade;

static std::vector<float> render(size_t in, size_t out, size_t length, size_t batch) {
    SentenceFade fade(in, out);
    std::vector<float> source(length, 1.0f), result;
    auto emit = [&](const float* data, size_t n) {
        result.insert(result.end(), data, data + n);
        return true;
    };
    for (size_t pos = 0; pos < length; pos += batch) {
        assert(fade.push(source.data() + pos, std::min(batch, length - pos), emit));
    }
    assert(fade.finish(emit));
    assert(fade.finish(emit)); // Flushing twice must not duplicate the tail.
    assert(result.size() == length);
    for (float sample : source) assert(sample == 1.0f);
    for (float sample : result) assert(std::isfinite(sample) && sample >= 0 && sample <= 1);
    return result;
}

int main() {
    const auto reference = render(1200, 2400, 12000, 12000);
    assert(reference.front() == 0 && reference.back() == 0);
    for (size_t i = 1; i < 1200; ++i) assert(reference[i] >= reference[i-1]);
    for (size_t i = 1200; i < 9600; ++i) assert(reference[i] == 1);
    for (size_t i = 9601; i < 12000; ++i) assert(reference[i] <= reference[i-1]);
    for (size_t batch : {1, 37, 2400, 3800, 10000}) {
        assert(render(1200, 2400, 12000, batch) == reference);
        for (float v : render(0, 0, 12000, batch)) assert(v == 1);
    }
    assert(render(0, 2400, 12000, 1000).front() == 1);
    assert(render(1200, 0, 12000, 1000).back() == 1);
    for (size_t n : {0, 1, 17, 2400}) {
        const auto short_audio = render(1200, 2400, n, 3);
        if (n) assert(short_audio.front() == 0 && short_audio.back() == 0);
    }
    float samples[8] = {1,1,1,1,1,1,1,1};
    auto stop = [](const float*, size_t) { return false; };
    auto accept = [](const float*, size_t) { return true; };
    SentenceFade cancelled(2, 4);
    assert(!cancelled.push(samples, 8, stop));
    SentenceFade tail_cancelled(2, 4);
    assert(tail_cancelled.push(samples, 4, accept));
    assert(!tail_cancelled.finish(stop));
}
