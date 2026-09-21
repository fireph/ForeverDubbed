"""Loopback-only, offline Pocket TTS service; the Go process plays the audio."""
import argparse
from collections import OrderedDict
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import hashlib
from pathlib import Path
import json
import re
import threading
import time
import traceback

from runtime import MODEL, ROOT, load_model, load_profiles, voice_source


class Engine:
    def __init__(self, config_path):
        self.profiles = load_profiles(config_path)
        self.digest = hashlib.sha256(Path(config_path).read_bytes()).hexdigest()
        self.model = load_model()
        self.device = "cpu"
        self.lock = threading.Lock()
        self.state_lock = threading.Lock()
        self.events = OrderedDict()
        self.prompts = {}
        states = {}
        for name, profile in self.profiles.items():
            source = voice_source(config_path, profile)
            if source not in states:
                states[source] = self.model.get_state_for_audio_prompt(source)
            self.prompts[name] = states[source]

    def event(self, request_id):
        with self.state_lock:
            if request_id not in self.events:
                self.events[request_id] = threading.Event()
                while len(self.events) > 512:
                    self.events.popitem(last=False)
            return self.events[request_id]

    def synthesize(self, request_id, voice, text):
        import numpy as np
        import scipy.io.wavfile

        event = self.event(request_id)
        while not self.lock.acquire(timeout=0.1):
            if event.is_set():
                raise InterruptedError("cancelled")
        try:
            if event.is_set():
                raise InterruptedError("cancelled")
            started = time.monotonic()
            # Pocket 3.1.0 has no supported generation stop parameter. Drain the
            # current short chunk before releasing the lock, so its internal
            # threads cannot race the next request. Cancelled audio is discarded.
            audio = self.model.generate_audio(self.prompts[voice], text, copy_state=True)
            if event.is_set():
                raise InterruptedError("cancelled")
            stream = io.BytesIO()
            scipy.io.wavfile.write(stream, self.model.sample_rate,
                                  (audio.numpy().clip(-1, 1) * 32767).astype(np.int16))
            seconds = len(audio) / self.model.sample_rate
            print(f"{voice}: {seconds:.2f}s audio in {time.monotonic()-started:.2f}s", flush=True)
            return stream.getvalue()
        finally:
            self.lock.release()


class Handler(BaseHTTPRequestHandler):
    server_version = "ForeverDubbedTTS/0.3.1"

    def send(self, status, body, content_type="application/json"):
        if not isinstance(body, bytes):
            body = json.dumps(body).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        try:
            self.wfile.write(body)
        except (BrokenPipeError, ConnectionResetError, ConnectionAbortedError):
            pass

    def do_GET(self):
        if self.path == "/health":
            self.send(200, {"ready": True, "device": self.server.engine.device,
                            "model": MODEL, "version": "0.3.1", "engine": "pocket-tts",
                            "config_digest": self.server.engine.digest})
        else:
            self.send(404, {"error": "not found"})

    def do_POST(self):
        if self.headers.get("Origin"):
            self.send(403, {"error": "browser requests are not accepted"})
            return
        try:
            if self.headers.get_content_type() != "application/json":
                raise ValueError("expected JSON")
            length = int(self.headers.get("Content-Length", "0"))
            if not 0 < length <= 8192:
                raise ValueError("invalid request size")
            request = json.loads(self.rfile.read(length))
            request_id = request.get("id", "")
            if not isinstance(request_id, str) or not re.fullmatch(r"[a-zA-Z0-9-]{1,64}", request_id):
                raise ValueError("invalid request ID")
            engine = self.server.engine
            if self.path == "/cancel":
                engine.event(request_id).set()
                self.send(200, {"cancelled": True})
            elif self.path == "/synthesize":
                voice, text = request.get("voice"), request.get("text")
                if not isinstance(voice, str) or voice not in engine.profiles:
                    raise ValueError("unknown voice")
                if not isinstance(text, str) or not text.strip() or len(text) > 400:
                    raise ValueError("text must contain 1–400 characters")
                self.send(200, engine.synthesize(request_id, voice, text), "audio/wav")
            else:
                self.send(404, {"error": "not found"})
        except (ValueError, TypeError, AttributeError, json.JSONDecodeError) as exc:
            self.send(400, {"error": str(exc)})
        except InterruptedError:
            self.send(409, {"error": "cancelled"})
        except Exception:
            traceback.print_exc()
            self.send(500, {"error": "synthesis failed; see the local TTS log"})

    def log_message(self, format, *args):
        if self.path != "/health":
            super().log_message(format, *args)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", default=str(ROOT / "tts" / "voices.json"))
    parser.add_argument("--port", type=int, default=8765)
    args = parser.parse_args()
    if not 1024 <= args.port <= 65535:
        parser.error("port must be 1024–65535")
    print("Loading local speech model...", flush=True)
    engine = Engine(args.config)
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.daemon_threads = True
    server.engine = engine
    print(f"Ready: {MODEL} on {engine.device}, http://127.0.0.1:{args.port}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
