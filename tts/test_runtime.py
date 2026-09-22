"""Model-free tests for the TTS runtime."""
from pathlib import Path
import tempfile
import unittest

from runtime import load_profiles, voice_source, synthesis_options


class RuntimeTests(unittest.TestCase):
    def test_synthesis_options_validation(self):
        self.assertEqual(synthesis_options({'decode_steps':32, 'cpu_threads':1}),
                         {'decode_steps':32, 'cpu_threads':1})
        for key, values in (('decode_steps', (0, 65, True, 2.5)),
                            ('cpu_threads', (0, -1, True, '4'))):
            for value in values:
                with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                    synthesis_options({key:value})


    def test_presets_and_relative_custom_voice(self):
        self.assertEqual(voice_source('tts/voices.json', {'voice': 'alba'}), 'alba')
        self.assertEqual(voice_source('tts/voices.json', {'voice': 'custom/orc.wav'}),
                         Path('tts/custom/orc.wav').resolve())
        with self.assertRaises(ValueError):
            voice_source('tts/voices.json', {'voice': 'https://example.com/voice'})


    def test_profiles(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / 'voices.json'
            p.write_text('{"profiles":{"orc":{"voice":"javert"}}}')
            self.assertEqual(load_profiles(p)['orc']['voice'], 'javert')
            p.write_text('{"profiles":{"orc":{"seed":123}}}')
            with self.assertRaises(ValueError):
                load_profiles(p)


if __name__ == '__main__':
    unittest.main()
