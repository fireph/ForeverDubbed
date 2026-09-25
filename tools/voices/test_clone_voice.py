"""Reference preparation checks; no model download or synthesis required."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import numpy as np
import soundfile as sf
from scipy.io import wavfile

from clone_voice import excerpt, read_audio


class ReferenceTests(unittest.TestCase):
    def test_real_mp3_decode_and_reference_conversion(self):
        rate = 44100
        signal = 0.3 * np.sin(2*np.pi*220*np.arange(rate*5)/rate)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "voice with spaces.MP3"
            sf.write(path, np.column_stack((signal, signal)), rate, format="MP3")
            actual_rate, audio, _ = read_audio(path)
        self.assertEqual(actual_rate, rate)
        self.assertAlmostEqual(len(audio)/rate, 5, delta=0.1)
        self.assertGreater(np.sqrt(np.mean(audio**2)), 0.1)
        clip, first, last = excerpt(audio, rate, 3, start=1)
        self.assertEqual((first, last), (1, 4))
        self.assertEqual(len(clip), 72000)
        self.assertTrue(np.isfinite(clip).all())

    def test_bad_audio_reports_file_and_unsupported_extension(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "broken.mp3"
            path.write_bytes(b"not an audio file")
            with self.assertRaisesRegex(ValueError, "Cannot decode audio reference"):
                read_audio(path)
            with self.assertRaisesRegex(ValueError, "use WAV or MP3"):
                read_audio(path.with_suffix(".txt"))

    def test_stereo_polarity_and_resampling(self):
        rate = 48000
        tone = (0.3 * np.sin(2*np.pi*220*np.arange(rate*5)/rate))
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "stereo.wav"
            wavfile.write(path, rate, (np.column_stack((tone, -tone))*32767).astype(np.int16))
            actual_rate, audio, clipped = read_audio(path)
        self.assertEqual(actual_rate, rate)
        self.assertEqual(clipped, 0)
        self.assertGreater(np.max(np.abs(audio)), 0.29)
        clip, first, last = excerpt(audio, rate, 3, start=1)
        self.assertEqual((first, last), (1, 4))
        self.assertEqual(len(clip), 72000)
        self.assertTrue(np.isfinite(clip).all())
        self.assertLessEqual(np.max(np.abs(clip)), 1)

    def test_automatic_selection_prefers_active_audio(self):
        rate = 1000
        audio = np.zeros(60*rate)
        audio[30*rate:55*rate] = 0.3*np.sin(np.arange(25*rate))
        clip, first, last = excerpt(audio, rate, 20)
        self.assertGreaterEqual(first, 29)
        self.assertLessEqual(last, 56)
        self.assertLessEqual(last-first, 30)
        self.assertGreater(np.sqrt(np.mean(clip**2)), 0.1)

    def test_prepare_only_preserves_existing_voice_and_config(self):
        # Exercise the real CLI from outside the repo: source paths are relative
        # to the caller, while outputs follow the selected configuration file.
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / "voices.json"
            original = json.dumps({"profiles": {"test": {"voice": "custom/test.safetensors"}}})
            config.write_text(original, encoding="utf-8")
            custom = root / "custom"
            custom.mkdir()
            voice = custom / "test.safetensors"
            voice.write_bytes(b"existing exported voice")
            rate = 24000
            samples = 0.3 * np.sin(2 * np.pi * 220 * np.arange(rate * 5) / rate)
            sf.write(root / "source with spaces.wav", samples, rate)
            script = Path(__file__).with_name("clone_voice.py").resolve()
            result = subprocess.run(
                [sys.executable, str(script), "--config", str(config),
                 "--voice", "test=source with spaces.wav", "--seconds", "3",
                 "--start", "1", "--prepare-only"],
                cwd=root, capture_output=True, text=True, timeout=30,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(config.read_text(encoding="utf-8"), original)
            self.assertEqual(voice.read_bytes(), b"existing exported voice")
            metadata = json.loads((custom / "test.reference.json").read_text())
            self.assertEqual(metadata["source"], str((root / "source with spaces.wav").resolve()))
            actual_rate, audio = wavfile.read(custom / "test.reference.wav")
            self.assertEqual(actual_rate, 24000)
            self.assertEqual(len(audio), 72000)
            self.assertFalse((custom / "test.preview.wav").exists())

    def test_invalid_or_silent_reference(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "invalid.wav"
            for audio in (np.zeros(48000, dtype=np.int16),
                          np.array([float("nan")], dtype=np.float32)):
                wavfile.write(path, 24000, audio)
                with self.assertRaises(ValueError):
                    read_audio(path)
        with self.assertRaises(ValueError):
            excerpt(np.zeros(24000*5), 24000, 3, start=0)
        with self.assertRaises(ValueError):
            excerpt(np.ones(24000*5), 24000, 3, start=6)


if __name__ == "__main__":
    unittest.main()
