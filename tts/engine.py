"""Offline model, voice cache, and serialized synthesis for streaming speech."""
from collections import OrderedDict
import hashlib
from pathlib import Path
import threading
import time

from runtime import load_model, load_profiles, voice_source, synthesis_settings


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

    def stream(self, request_id, voice, text, emit):
        """Emit PCM while decoding; always drain the model before unlocking."""
        event = self.event(request_id)
        while not self.lock.acquire(timeout=0.1):
            if event.is_set():
                raise InterruptedError("cancelled")
        try:
            if event.is_set():
                raise InterruptedError("cancelled")
            started = time.monotonic()
            samples = 0
            failure = None
            with synthesis_settings(self.model, self.profiles[voice]):
                for audio in self.model.generate_audio_stream(self.prompts[voice], text, copy_state=True):
                    # Closing this iterator early can leave Pocket's decoder
                    # running against state needed by the next request.
                    if event.is_set():
                        continue
                    try:
                        pcm = (audio.numpy().clip(-1, 1) * 32767).astype("<i2").tobytes()
                        if pcm:
                            emit(pcm)
                            if samples == 0:
                                print(f"{voice}: first audio in {time.monotonic()-started:.2f}s", flush=True)
                            samples += len(pcm) // 2
                    except Exception as exc:
                        failure = exc
                        event.set()
            if failure is not None:
                raise failure
            if event.is_set():
                raise InterruptedError("cancelled")
            seconds = samples / self.model.sample_rate
            print(f"{voice}: {seconds:.2f}s audio in {time.monotonic()-started:.2f}s", flush=True)
        finally:
            self.lock.release()
