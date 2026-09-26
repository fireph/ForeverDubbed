#include "pocket_tts.hpp"
#include <cassert>
#include <cstdlib>
#include <iostream>

static void punctuation() {
    const std::pair<const char *, const char *> cases[] = {
        {"", ""},
        {"hello world", "hello world."},
        {"hello world,", "hello world."},
        {"hello world;", "hello world."},
        {"hello world:", "hello world."},
        {"hello world-", "hello world."},
        {"hello world\u2013", "hello world."},
        {"hello world\u2014", "hello world."},
        {"hello world , --", "hello world."},
        {"he said \"go home,\"", "he said \"go home.\""},
        {"he said \"go home\"", "he said \"go home\"."},
        {"he said \"go home.\"", "he said \"go home.\""},
        {"he said \u201cgo home,\u201d", "he said \u201cgo home.\u201d"},
        {"he said \u2018go home\u2019", "he said \u2018go home\u2019."},
        {"he said \u2018go home.\u2019", "he said \u2018go home.\u2019"},
        {"he said \u00abgo home:\u00bb", "he said \u00abgo home.\u00bb"},
        {"(below)", "(below)."},
        {"[below,]", "[below.]"},
        {"50%", "50%."},
        {"is it (really)?", "is it (really)?"},
        {"wait\u2026", "wait\u2026"},
        {"wait!", "wait!"},
        {"(wait.)", "(wait.)"},
        {"hello,  ) ]", "hello.) ]"},
        {"\"') ]", "\"') ]"},
        {"caf\u00e9", "caf\u00e9."},
    };
    for (auto [input, expected] : cases) {
        auto actual = pocket_tts::ensure_terminal_punctuation(input);
        if (actual != expected) {
            std::cerr << "input: " << input << "\nexpected: " << expected << "\nactual: " << actual
                      << '\n';
            std::abort();
        }
        assert(pocket_tts::ensure_terminal_punctuation(actual) == actual);
    }
    // Exercise the actual preparation entry point, including existing cleanup,
    // capitalization, short-input padding and EOS tail overrides.
    assert(pocket_tts::prepare_text("  hello, \n", -1) ==
           std::make_pair(std::string("        Hello."), 5));
    assert(pocket_tts::prepare_text("[below,]", 7) ==
           std::make_pair(std::string("        [below.]"), 7));
    assert(pocket_tts::prepare_text("the door is open (below)", -1) ==
           std::make_pair(std::string("The door is open (below)."), 3));
    assert(pocket_tts::prepare_text("he said \"go home,\"", -1).first ==
           "        He said go home.");
    assert(pocket_tts::prepare_text("don't stop!", -1).first == "        Don't stop!");
    assert(pocket_tts::prepare_text(" \t\n", -1).first.empty());
}

static void generation_end() {
    // Continuous EOS predictions during the leading pause must not suppress a word.
    pocket_tts::GenerationEnd short_word;
    for (int frame = 0; frame < 11; ++frame)
        assert(!short_word.should_stop(frame, -3.0f, -4.0f, 5));
    assert(short_word.first_frame() == 6);
    assert(short_word.should_stop(11, -3.0f, -4.0f, 5));

    // A transient early prediction is discarded, not deferred until frame six.
    // Once a later EOS is accepted, changing logits must not reset its tail.
    pocket_tts::GenerationEnd sentence;
    for (int frame = 0; frame < 20; ++frame) {
        float logit = frame < 6 ? -3.0f : -5.0f;
        assert(!sentence.should_stop(frame, logit, -4.0f, 3));
        assert(sentence.first_frame() == -1);
    }
    assert(!sentence.should_stop(20, -4.0f, -4.0f, 3)); // Strict threshold.
    assert(!sentence.should_stop(21, -3.0f, -4.0f, 3));
    assert(!sentence.should_stop(22, -5.0f, -4.0f, 3));
    assert(!sentence.should_stop(23, -5.0f, -4.0f, 3));
    assert(sentence.should_stop(24, -5.0f, -4.0f, 3));
    assert(sentence.first_frame() == 21);

    pocket_tts::GenerationEnd no_tail;
    for (int frame = 0; frame < 6; ++frame)
        assert(!no_tail.should_stop(frame, 1.0f, 0.0f, 0));
    assert(no_tail.should_stop(6, 1.0f, 0.0f, 0));
}

int main() {
    punctuation();
    generation_end();
}
