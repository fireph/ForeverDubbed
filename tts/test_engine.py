"""Model-free tests for the TTS engine."""
import hashlib
import json
from pathlib import Path
import struct
import tempfile
import unittest
from unittest.mock import patch

from engine import Engine


class EngineTests(unittest.TestCase):
    def test_per_voice_settings_restore_after_success_and_failure(self):
        import torch
        original_threads = torch.get_num_threads()
        class Model:
            sampler_decode_steps = 1
            sample_rate = 24000
            calls = []
            def get_state_for_audio_prompt(self, source):
                return source
            def generate_audio_stream(self, state, text, **kwargs):
                self.calls.append((self.sampler_decode_steps, torch.get_num_threads()))
                if text == 'fail':
                    raise RuntimeError('synthesis error')
                yield torch.ones(240) * 0.1
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / 'voices.json'
            p.write_text(json.dumps({'profiles':{
                'orc':{'voice':'javert','decode_steps':32,'cpu_threads':1},
                'human':{'voice':'alba'}}}))
            model = Model()
            with patch('engine.load_model', return_value=model):
                engine = Engine(p)
            engine.stream('one', 'orc', 'Hello', lambda pcm: None)
            self.assertEqual(model.calls[-1], (32, 1))
            engine.stream('two', 'human', 'Hello', lambda pcm: None)
            self.assertEqual(model.calls[-1], (1, original_threads))
            with self.assertRaisesRegex(RuntimeError, 'synthesis error'):
                engine.stream('three', 'orc', 'fail', lambda pcm: None)
            self.assertEqual(model.sampler_decode_steps, 1)
            self.assertEqual(torch.get_num_threads(), original_threads)
            self.assertTrue(engine.lock.acquire(blocking=False))
            engine.lock.release()


    def test_preload_and_precancel(self):
        class Model:
            def get_state_for_audio_prompt(self, source):
                return source
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / 'voices.json'
            p.write_text('{"profiles":{"human":{"voice":"alba"}}}')
            with patch('engine.load_model', return_value=Model()):
                engine = Engine(p)
            self.assertEqual(engine.digest, hashlib.sha256(p.read_bytes()).hexdigest())
            engine.event('cancelled').set()
            with self.assertRaises(InterruptedError):
                engine.stream('cancelled', 'human', 'Do not speak.', lambda pcm: None)
            self.assertTrue(engine.lock.acquire(blocking=False))
            engine.lock.release()


    def test_stream_drains_after_cancel_or_disconnect(self):
        import numpy as np
        class Audio:
            def numpy(self):
                return np.array([-2., 0., 2.], dtype=np.float32)
        class Model:
            sample_rate = 24000
            def get_state_for_audio_prompt(self, source):
                return source
            def generate_audio_stream(self, state, text, **kwargs):
                for i in range(3):
                    self.assert_locked()
                    yield Audio()
                    self.consumed += 1
                self.finished = True
        with tempfile.TemporaryDirectory() as tmp:
            config = Path(tmp) / 'voices.json'
            config.write_text('{"profiles":{"human":{"voice":"alba"}}}')
            model = Model()
            with patch('engine.load_model', return_value=model):
                engine = Engine(config)
            model.assert_locked = lambda: self.assertTrue(engine.lock.locked())
            for disconnected in (False, True):
                with self.subTest(disconnected=disconnected):
                    model.finished, model.consumed = False, 0
                    frames = []
                    request_id = str(disconnected)
                    def emit(pcm):
                        frames.append(pcm)
                        self.assertFalse(model.finished)
                        if disconnected:
                            raise BrokenPipeError('client disconnected')
                        engine.event(request_id).set()
                    expected = BrokenPipeError if disconnected else InterruptedError
                    with self.assertRaises(expected):
                        engine.stream(request_id, 'human', 'Hello', emit)
                    self.assertEqual(frames, [struct.pack('<hhh', -32767, 0, 32767)])
                    self.assertTrue(model.finished)
                    self.assertEqual(model.consumed, 3)
                    self.assertFalse(engine.lock.locked())
            # A later request can use the same model after it has drained.
            frames = []
            engine.stream('next', 'human', 'Hello again', frames.append)
            self.assertEqual(frames, [struct.pack('<hhh', -32767, 0, 32767)] * 3)


if __name__ == '__main__':
    unittest.main()
