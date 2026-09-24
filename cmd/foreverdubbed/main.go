package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"time"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/pocket"
	"foreverdubbed/internal/protocol"
	"foreverdubbed/internal/speech"
)

const version = "0.5.0"

func main() {
	prepareConsole()
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime)
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	var files, voice, snapshot, captureApp string
	var backend, configPath, testText, testRace, testGender string
	var mute, list, showVersion, headless bool
	var rate, threads int
	var nativeDir, modelsDir string
	var poll, scan time.Duration
	flag.StringVar(&nativeDir, "native-dir", pocket.DefaultDir(), "native model and preset directory")
	flag.StringVar(&modelsDir, "models-dir", "", "ONNX models directory (default: native-dir/models)")
	flag.IntVar(&threads, "cpu-threads", 1, "native inference CPU thread budget")
	flag.StringVar(&files, "image", "", "decode PNG file(s), comma-separated, without screen capture or TTS")
	flag.StringVar(&voice, "voice", "", "Pocket TTS profile override, or system voice name with -tts system")
	flag.StringVar(&snapshot, "snapshot", "", "save one game-window PNG after 3 seconds, then exit")
	if runtime.GOOS == "windows" {
		flag.StringVar(&captureApp, "capture-app", "WoWB.exe", "capture only this Windows executable (exact filename or full path)")
	} else if runtime.GOOS == "darwin" {
		flag.StringVar(&captureApp, "capture-app", "World of Warcraft Beta.app", "capture only this macOS app (bundle name, absolute path, application name, or bundle identifier)")
	}
	flag.StringVar(&backend, "tts", "pocket", "speech backend: pocket (local CPU) or system (OS voices; sapi is a Windows alias)")
	flag.StringVar(&configPath, "voice-config", speech.DefaultConfigPath(), "local race/voice mapping JSON")
	flag.StringVar(&testText, "speak-test", "", "speak this text once without screen capture")
	flag.StringVar(&testRace, "race", "Human", "race for -speak-test")
	flag.StringVar(&testGender, "gender", "male", "gender for -speak-test")
	flag.BoolVar(&showVersion, "version", false, "print version and optical format, then exit")
	flag.BoolVar(&headless, "headless", false, "run in the terminal without the desktop interface")
	flag.BoolVar(&mute, "mute", false, "print decoded JSON without speaking")
	flag.BoolVar(&list, "voices", false, "list configured voice profiles (OS voices with -tts system)")
	flag.IntVar(&rate, "rate", 0, "system speech rate, -10 through 10 (only with -tts system/sapi)")
	flag.DurationVar(&poll, "poll", 75*time.Millisecond, "capture interval while tracking the tile")
	flag.DurationVar(&scan, "scan", time.Second, "capture search interval when tile is missing")
	flag.Parse()
	if showVersion {
		fmt.Printf("ForeverDubbed %s, FDB5, 16 colors, %d bytes/page\n", version, protocol.PayloadBytes)
		return nil
	}
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flag.Args())
	}
	if rate < -10 || rate > 10 || poll < 20*time.Millisecond || scan < 100*time.Millisecond {
		return fmt.Errorf("rate must be -10..10; poll >=20ms; scan >=100ms")
	}
	if snapshot != "" && files != "" {
		return fmt.Errorf("use -snapshot or -image, not both")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if backend == "sapi" && runtime.GOOS != "windows" {
		return fmt.Errorf("-tts sapi requires Windows; use -tts system for macOS voices")
	}
	if backend == "sapi" {
		backend = "system"
	}
	if backend != "pocket" && backend != "local" && backend != "system" {
		return fmt.Errorf("-tts must be pocket or system (OS voices; sapi is a Windows alias)")
	}
	if list && backend == "system" {
		s, err := platform.Voices(ctx)
		fmt.Print(s)
		return err
	}
	var assembler protocol.Assembler
	output := json.NewEncoder(os.Stdout)
	if files != "" {
		completed := 0
		for _, name := range strings.Split(files, ",") {
			f, err := os.Open(strings.TrimSpace(name))
			if err != nil {
				return err
			}
			im, _, err := image.Decode(f)
			f.Close()
			if err != nil {
				return err
			}
			_, p, err := protocol.Find(im)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			m, err := assembler.Add(p, time.Now())
			if err != nil {
				return err
			}
			if m != nil {
				if err := output.Encode(m); err != nil {
					return err
				}
				completed++
			}
		}
		if completed == 0 {
			return fmt.Errorf("valid pages read, but no complete message; supply screenshots for every page")
		}
		return nil
	}
	guiMode := desktopEnabled && !headless && !list && testText == "" && snapshot == ""
	state := appstate.New(captureApp, backend, mute)
	work := func() error {
		var speak func(context.Context, protocol.Message) error
		if !mute && snapshot == "" || list || testText != "" {
			if backend != "system" {
				config, err := speech.Load(configPath)
				if err != nil {
					return fmt.Errorf("voice config: %w", err)
				}
				if list {
					ids := make([]string, 0, len(config.Profiles))
					for id := range config.Profiles {
						ids = append(ids, id)
					}
					sort.Strings(ids)
					for _, id := range ids {
						fmt.Printf("%s (%s)\n", id, config.Profiles[id].Voice)
					}
					return nil
				}
				local, err := speech.OpenLocal(config, voice, nativeDir, modelsDir, threads)
				if err != nil {
					return err
				}
				defer local.Close()
				log.Print("PocketTTS.cpp: native CPU streaming ready")
				speak = func(ctx context.Context, m protocol.Message) error {
					id, err := config.Voice(m, voice)
					if err != nil {
						return err
					}
					log.Printf("Voice %s (race=%q gender=%q NPC=%q)", id, m.Race, m.Gender, m.NPCID)
					state.Update(func(v *appstate.Snapshot) { v.Voice = id })
					ctx = platform.WithPlaybackObserver(ctx, func() { state.Audio("Playing audio") })
					ctx = platform.WithPlaybackProgress(ctx, func(played, total time.Duration, known bool) {
						state.Update(func(v *appstate.Snapshot) { v.Played, v.Duration, v.DurationKnown = played, total, known })
					})
					return local.Speak(ctx, m)
				}
			} else {
				speak = func(ctx context.Context, m protocol.Message) error {
					state.Audio("Speaking (system voice)")
					return platform.Speak(ctx, m.Speech(), voice, rate)
				}
			}
		}
		if testText != "" {
			return speak(ctx, protocol.Message{Text: testText, Race: testRace, Gender: testGender})
		}
		if err := platform.Init(captureApp); err != nil {
			return err
		}
		defer platform.CloseCapture()
		if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
			log.Printf("Capture is limited to windows owned by %q; waiting if the game is unavailable.", captureApp)
		}
		if snapshot != "" {
			return saveSnapshot(ctx, snapshot)
		}
		state.Update(func(v *appstate.Snapshot) {
			v.Ready = true
			if !mute {
				v.Audio = "Idle"
			}
		})
		// A single worker applies the selected queue/interrupt policy.
		speech := make(chan protocol.Message, 1)
		done := make(chan struct{})
		speechCtx, cancelSpeech := context.WithCancel(ctx)
		go func() { defer close(done); speakLoop(speechCtx, speech, speak, state) }()
		defer func() { cancelSpeech(); <-done }()
		log.Printf("ForeverDubbed %s (FDB5, 16 colors). Searching; in WoW: /fdb unlock. Ctrl+C to quit.", version)
		var location *protocol.Location
		failures := 0
		var lastError time.Time
		for ctx.Err() == nil {
			delay := poll
			var p protocol.Packet
			windowOK, tileOK := false, false
			var err error
			if location == nil {
				delay = scan
				im, captureErr := platform.Capture(platform.Desktop())
				if captureErr != nil {
					err = captureErr
				} else {
					windowOK = true
					l, packet, findErr := protocol.Find(im)
					if findErr == nil {
						tileOK = true
						location = &l
						p = packet
						failures = 0
						delay = poll
						log.Printf("Found tile at (%d, %d), %dpx cells, %d bytes/page.", l.X, l.Y, l.Cell, protocol.PayloadBytes)
					} else {
						err = findErr
					}
				}
			} else {
				im, captureErr := platform.Capture(location.Rect())
				if captureErr != nil {
					err = captureErr
				} else {
					windowOK = true
					p, err = protocol.Decode(im, *location)
					tileOK = err == nil
				}
				if err != nil {
					failures++
					if failures >= 4 {
						location = nil
						log.Print("Tile lost; searching again.")
					}
				} else {
					failures = 0
				}
			}
			state.Capture(windowOK, tileOK, err)
			if err == nil {
				m, assemblyErr := assembler.Add(p, time.Now())
				if assemblyErr != nil {
					log.Printf("Discarding message: %v", assemblyErr)
				}
				if m != nil {
					if !m.IsControl() {
						state.Received(*m)
					}
					if !guiMode {
						if err := output.Encode(m); err != nil {
							return err
						}
					}
					if !mute {
						select {
						case speech <- *m:
						case <-ctx.Done():
							return nil
						}
					}
				}
			} else if time.Since(lastError) > 15*time.Second {
				log.Printf("Waiting: %v", err)
				lastError = time.Now()
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
		return nil
	}
	if guiMode {
		return runDesktop(ctx, stop, state, work)
	}
	return work()
}

func saveSnapshot(ctx context.Context, path string) error {
	log.Print("Taking a game-window snapshot in 3 seconds. Keep the game and square visible.")
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	im, err := platform.Capture(platform.Desktop())
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	encodeErr := png.Encode(f, im)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	log.Printf("Saved capture PNG %s (%d × %d).", path, im.Bounds().Dx(), im.Bounds().Dy())
	l, p, err := protocol.Find(im)
	if err != nil {
		return err
	}
	log.Printf("Valid tile at (%d,%d), %dpx cells; page %d/%d.", l.X, l.Y, l.Cell, p.Index+1, p.Count)
	return nil
}

func speakLoop(ctx context.Context, requests <-chan protocol.Message, speak func(context.Context, protocol.Message) error, state *appstate.State) {
	var cancel context.CancelFunc
	var finished chan error
	var nextID uint64
	var activeKind byte
	var pending []protocol.Message
	updateQueue := func() { state.Update(func(v *appstate.Snapshot) { v.Queued = len(pending) }) }
	stopCurrent := func() {
		if cancel != nil {
			cancel()
			<-finished
			cancel = nil
			finished = nil
			state.ResetPlayback()
		}
	}
	defer func() { stopCurrent(); pending = nil; updateQueue() }()
	start := func(message protocol.Message) {
		activeKind = message.Kind
		nextID++
		state.Update(func(v *appstate.Snapshot) {
			v.Audio = "Preparing speech"
			v.SpeechError = ""
			v.PlaybackID = nextID
			v.PlayingSpeaker = message.Speaker
		})
		child, c := context.WithCancel(ctx)
		cancel = c
		result := make(chan error, 1)
		finished = result
		go func() { result <- speak(child, message) }()
	}
	for {
		if ctx.Err() != nil {
			return
		}
		// Remove disabled categories before starting any queued message.
		filters := state.Snapshot().Filters
		kept := pending[:0]
		for _, message := range pending {
			if filters.Allows(message.Kind) {
				kept = append(kept, message)
			}
		}
		clear(pending[len(kept):])
		if len(kept) != len(pending) {
			pending = kept
			updateQueue()
		}
		if finished != nil && !filters.Allows(activeKind) {
			stopCurrent()
		}
		// Switching back to interrupt mode keeps only the newest waiting message.
		if !state.Snapshot().QueueSpeech && len(pending) > 0 {
			newest := pending[len(pending)-1]
			pending = nil
			updateQueue()
			stopCurrent()
			start(newest)
		}
		if finished == nil && len(pending) > 0 {
			next := pending[0]
			pending[0] = protocol.Message{}
			pending = pending[1:]
			updateQueue()
			start(next)
		}
		select {
		case <-ctx.Done():
			return
		case <-state.SpeechChanges():
			// Apply queue mode and category changes at the top of the loop.
		case id := <-state.AudioStops():
			if id == state.Snapshot().PlaybackID {
				stopCurrent()
			}
		case message, ok := <-requests:
			if !ok {
				requests = nil
				continue
			}
			if message.IsControl() {
				// Controls bypass queue mode. Cancelling the current utterance
				// advances any pending dialogue, just like the desktop buttons.
				stopCurrent()
				continue
			}
			if !state.Snapshot().Filters.Allows(message.Kind) {
				continue
			}
			if state.Snapshot().QueueSpeech && finished != nil {
				pending = append(pending, message)
				updateQueue()
			} else {
				pending = nil
				updateQueue()
				stopCurrent()
				start(message)
			}
		case err := <-finished:
			state.ResetPlayback()
			if err != nil && ctx.Err() == nil {
				log.Printf("TTS error: %v", err)
				state.Update(func(v *appstate.Snapshot) { v.SpeechError = err.Error() })
			}
			cancel()
			cancel = nil
			finished = nil
		}
	}
}
