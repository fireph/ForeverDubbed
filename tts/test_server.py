"""Model-free tests for voice routing, cancellation and service validation."""
import hashlib
import http.client
import json
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import patch

from runtime import load_profiles, voice_source, synthesis_options
from server import Engine, Handler, ThreadingHTTPServer


class RuntimeTests(unittest.TestCase):
    def test_synthesis_options_validation(self):
        self.assertEqual(synthesis_options({'decode_steps':32, 'cpu_threads':1}),
                         {'decode_steps':32, 'cpu_threads':1})
        for key, values in (('decode_steps', (0, 65, True, 2.5)),
                            ('cpu_threads', (0, -1, True, '4'))):
            for value in values:
                with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                    synthesis_options({key:value})

    def test_per_voice_settings_restore_after_success_and_failure(self):
        import torch
        original_threads = torch.get_num_threads()
        class Model:
            sampler_decode_steps = 1
            sample_rate = 24000
            calls = []
            def get_state_for_audio_prompt(self, source):
                return source
            def generate_audio(self, state, text, **kwargs):
                self.calls.append((self.sampler_decode_steps, torch.get_num_threads()))
                if text == 'fail':
                    raise RuntimeError('synthesis error')
                return torch.ones(240) * 0.1
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / 'voices.json'
            p.write_text(json.dumps({'profiles':{
                'orc':{'voice':'javert','decode_steps':32,'cpu_threads':1},
                'human':{'voice':'alba'}}}))
            model = Model()
            with patch('server.load_model', return_value=model):
                engine = Engine(p)
            engine.synthesize('one', 'orc', 'Hello')
            self.assertEqual(model.calls[-1], (32, 1))
            engine.synthesize('two', 'human', 'Hello')
            self.assertEqual(model.calls[-1], (1, original_threads))
            with self.assertRaisesRegex(RuntimeError, 'synthesis error'):
                engine.synthesize('three', 'orc', 'fail')
            self.assertEqual(model.sampler_decode_steps, 1)
            self.assertEqual(torch.get_num_threads(), original_threads)
            self.assertTrue(engine.lock.acquire(blocking=False))
            engine.lock.release()

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

    def test_preload_and_precancel(self):
        class Model:
            def get_state_for_audio_prompt(self, source):
                return source
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp) / 'voices.json'
            p.write_text('{"profiles":{"human":{"voice":"alba"}}}')
            with patch('server.load_model', return_value=Model()):
                engine = Engine(p)
            self.assertEqual(engine.digest, hashlib.sha256(p.read_bytes()).hexdigest())
            engine.event('cancelled').set()
            with self.assertRaises(InterruptedError):
                engine.synthesize('cancelled', 'human', 'Do not speak.')
            self.assertTrue(engine.lock.acquire(blocking=False))
            engine.lock.release()

    def test_http_contract(self):
        class FakeEngine:
            device = 'cpu'
            digest = 'digest'
            profiles = {'human_male': {'voice': 'marius'}}
            stopped = threading.Event()
            def event(self, request_id):
                return self.stopped
            def synthesize(self, request_id, voice, text):
                return b'RIFF-test'
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        server.engine = FakeEngine()
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            conn = http.client.HTTPConnection(*server.server_address)
            conn.request('GET', '/health')
            resp = conn.getresponse()
            self.assertEqual(json.loads(resp.read())['engine'], 'pocket-tts')
            for value, want in [({'id':'x', 'voice':'human_male', 'text':'Hello'}, 200),
                                ({'id':'x', 'voice':'missing', 'text':'Hello'}, 400),
                                ({'id':'x', 'voice':'human_male', 'text':'x'*401}, 400)]:
                conn.request('POST', '/synthesize', json.dumps(value), {'Content-Type':'application/json'})
                resp = conn.getresponse()
                self.assertEqual(resp.status, want)
                resp.read()
            conn.request('POST', '/cancel', '{"id":"x"}', {'Content-Type':'application/json'})
            conn.getresponse().read()
            self.assertTrue(server.engine.stopped.is_set())
            conn.close()
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == '__main__':
    unittest.main()
