"""Model-free tests for the TTS server."""
import http.client
import json
import struct
import threading
import unittest
from unittest.mock import patch

from server import Handler, ThreadingHTTPServer


class ServerTests(unittest.TestCase):
    def test_http_stream_flush_and_failure(self):
        release = threading.Event()
        class FakeEngine:
            profiles = {'human': {'voice': 'alba'}}
            class model:
                sample_rate = 24000
            def event(self, request_id):
                return threading.Event()
            def stream(self, request_id, voice, text, emit):
                if text == 'cancel':
                    raise InterruptedError('cancelled')
                emit(b'\x01\x00\x02\x00')
                if text == 'fail':
                    raise RuntimeError('generation failed')
                if not release.wait(2):
                    raise RuntimeError('client did not receive first frame')
                emit(b'\x03\x00')
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        server.engine = FakeEngine()
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            for text in ('Hello', 'fail', 'cancel'):
                with self.subTest(text=text):
                    conn = http.client.HTTPConnection(*server.server_address, timeout=3)
                    with patch('server.traceback.print_exc'):
                        conn.request('POST', '/stream', json.dumps({'id':'x','voice':'human','text':text}),
                                     {'Content-Type':'application/json'})
                        response = conn.getresponse()
                        if text == 'cancel':
                            self.assertEqual(response.status, 409)
                            response.read()
                        else:
                            self.assertEqual(response.status, 200)
                            self.assertEqual(response.getheader('X-Sample-Rate'), '24000')
                            self.assertEqual(response.read(8), struct.pack('<Ihh', 4, 1, 2))
                            release.set()
                            tail = response.read()
                            self.assertEqual(tail, b'' if text == 'fail' else struct.pack('<IhI', 2, 3, 0))
                    conn.close()
        finally:
            release.set()
            server.shutdown()
            server.server_close()
            thread.join()


    def test_http_contract(self):
        class FakeEngine:
            device = 'cpu'
            digest = 'digest'
            profiles = {'human_male': {'voice': 'marius'}}
            stopped = threading.Event()
            def event(self, request_id):
                return self.stopped
            class model:
                sample_rate = 24000
            def stream(self, request_id, voice, text, emit):
                emit(b'\x01\x00')
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
                conn.request('POST', '/stream', json.dumps(value), {'Content-Type':'application/json'})
                resp = conn.getresponse()
                self.assertEqual(resp.status, want)
                resp.read()
            # The speech service exposes only streaming synthesis.
            conn.request('POST', '/synthesize', json.dumps({'id':'x', 'voice':'human_male', 'text':'Hello'}),
                         {'Content-Type':'application/json'})
            response = conn.getresponse()
            self.assertEqual(response.status, 404)
            response.read()
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
