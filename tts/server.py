"""Loopback-only, offline Pocket TTS service; the Go process plays the audio."""
import argparse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import re
import struct
import traceback

from engine import Engine
from runtime import MODEL, ROOT

VERSION = "0.3.2"
STREAM_FORMAT = "pcm-s16le-v1"
STREAM_CONTENT_TYPE = "application/x-foreverdubbed-pcm"


class Handler(BaseHTTPRequestHandler):
    server_version = f"ForeverDubbedTTS/{VERSION}"

    def send_json(self, status, body):
        body = json.dumps(body).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        try:
            self.wfile.write(body)
        except (BrokenPipeError, ConnectionResetError, ConnectionAbortedError):
            pass

    def do_GET(self):
        if self.path == "/health":
            self.send_json(200, {"ready": True, "device": self.server.engine.device,
                            "model": MODEL, "version": VERSION, "engine": "pocket-tts",
                            "config_digest": self.server.engine.digest,
                            "stream_format": STREAM_FORMAT})
        else:
            self.send_json(404, {"error": "not found"})

    def do_POST(self):
        if self.headers.get("Origin"):
            self.send_json(403, {"error": "browser requests are not accepted"})
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
                self.send_json(200, {"cancelled": True})
            elif self.path == "/stream":
                voice, text = request.get("voice"), request.get("text")
                if not isinstance(voice, str) or voice not in engine.profiles:
                    raise ValueError("unknown voice")
                if not isinstance(text, str) or not text.strip() or len(text) > 400:
                    raise ValueError("text must contain 1–400 characters")
                self.stream_audio(request_id, voice, text)
            else:
                self.send_json(404, {"error": "not found"})
        except (ValueError, TypeError, AttributeError) as exc:
            self.send_json(400, {"error": str(exc)})
        except InterruptedError:
            self.send_json(409, {"error": "cancelled"})
        except Exception:
            traceback.print_exc()
            self.send_json(500, {"error": "synthesis failed; see the local TTS log"})

    def stream_audio(self, request_id, voice, text):
        # HTTP/1.0 close-delimited response. Explicit frame lengths and a zero
        # terminator distinguish successful completion from a broken stream.
        started = False
        previous_timeout = self.connection.gettimeout()
        self.connection.settimeout(10)
        self.close_connection = True

        def emit(pcm):
            nonlocal started
            if not started:
                self.send_response(200)
                self.send_header("Content-Type", STREAM_CONTENT_TYPE)
                self.send_header("X-Sample-Rate", str(self.server.engine.model.sample_rate))
                self.send_header("Cache-Control", "no-store")
                self.send_header("Connection", "close")
                started = True
                self.end_headers()
            self.wfile.write(struct.pack("<I", len(pcm)))
            self.wfile.write(pcm)
            self.wfile.flush()

        try:
            self.server.engine.stream(request_id, voice, text, emit)
            if not started:
                raise ValueError("model returned no audio")
            emit(b"")
        except Exception:
            if not started:
                raise
            # After headers, close without a terminator so clients detect failure.
            if not self.server.engine.event(request_id).is_set():
                traceback.print_exc()
        finally:
            self.connection.settimeout(previous_timeout)

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
