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
	"sort"
	"strings"
	"time"

	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/pocket"
	"foreverdubbed/internal/protocol"
	"foreverdubbed/internal/speech"
)

const version = "0.4.0"

func main() {
	log.SetFlags(log.Ltime)
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	var files, voice, snapshot string
	var backend, configPath, testText, testRace, testGender string
	var mute, list, showVersion bool
	var rate, threads int
	var nativeDir, modelsDir string
	var poll, scan time.Duration
	flag.StringVar(&nativeDir, "native-dir", pocket.DefaultDir(), "native model and preset directory")
	flag.StringVar(&modelsDir, "models-dir", "", "ONNX models directory (default: native-dir/models)")
	flag.IntVar(&threads, "cpu-threads", 1, "native inference CPU thread budget")
	flag.StringVar(&files, "image", "", "decode PNG file(s), comma-separated, without screen capture or TTS")
	flag.StringVar(&voice, "voice", "", "Pocket TTS profile override, or Windows voice name with -tts sapi")
	flag.StringVar(&snapshot, "snapshot", "", "save one desktop PNG after 3 seconds, report detection, then exit")
	flag.StringVar(&backend, "tts", "pocket", "speech backend: pocket (local CPU) or sapi")
	flag.StringVar(&configPath, "voice-config", speech.DefaultConfigPath(), "local race/voice mapping JSON")
	flag.StringVar(&testText, "speak-test", "", "speak this text once without screen capture")
	flag.StringVar(&testRace, "race", "Human", "race for -speak-test")
	flag.StringVar(&testGender, "gender", "male", "gender for -speak-test")
	flag.BoolVar(&showVersion, "version", false, "print version and optical format, then exit")
	flag.BoolVar(&mute, "mute", false, "print decoded JSON without speaking")
	flag.BoolVar(&list, "voices", false, "list configured voice profiles (Windows voices with -tts sapi)")
	flag.IntVar(&rate, "rate", 0, "SAPI speech rate, -10 through 10 (only with -tts sapi)")
	flag.DurationVar(&poll, "poll", 75*time.Millisecond, "capture interval while tracking the tile")
	flag.DurationVar(&scan, "scan", time.Second, "full desktop search interval when tile is missing")
	flag.Parse()
	if showVersion {
		fmt.Printf("ForeverDubbed %s, FDB4, 16 colors, %d bytes/page\n", version, protocol.PayloadBytes)
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
	if backend != "pocket" && backend != "local" && backend != "sapi" {
		return fmt.Errorf("-tts must be pocket or sapi")
	}
	if list && backend == "sapi" {
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
	var speak func(context.Context, protocol.Message) error
	if !mute && snapshot == "" || list || testText != "" {
		if backend != "sapi" {
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
				return local.Speak(ctx, m)
			}
		} else {
			speak = func(ctx context.Context, m protocol.Message) error {
				return platform.Speak(ctx, m.Speech(), voice, rate)
			}
		}
	}
	if testText != "" {
		return speak(ctx, protocol.Message{Text: testText, Race: testRace, Gender: testGender})
	}
	if err := platform.Init(); err != nil {
		return err
	}
	if snapshot != "" {
		return saveSnapshot(ctx, snapshot)
	}
	// A single worker serializes speech and cancels it when newer dialog arrives.
	speech := make(chan protocol.Message, 1)
	done := make(chan struct{})
	go func() { defer close(done); speakLoop(ctx, speech, speak) }()
	defer func() { stop(); <-done }()
	log.Printf("ForeverDubbed %s (FDB4, 16 colors). Searching; in WoW: /fdb unlock. Ctrl+C to quit.", version)
	var location *protocol.Location
	failures := 0
	var lastError time.Time
	for ctx.Err() == nil {
		delay := poll
		var p protocol.Packet
		var err error
		if location == nil {
			delay = scan
			im, captureErr := platform.Capture(platform.Desktop())
			if captureErr != nil {
				err = captureErr
			} else {
				l, packet, findErr := protocol.Find(im)
				if findErr == nil {
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
				p, err = protocol.Decode(im, *location)
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
		if err == nil {
			m, assemblyErr := assembler.Add(p, time.Now())
			if assemblyErr != nil {
				log.Printf("Discarding message: %v", assemblyErr)
			}
			if m != nil {
				if err := output.Encode(m); err != nil {
					return err
				}
				if !mute {
					select {
					case <-speech:
					default:
					}
					speech <- *m
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

func saveSnapshot(ctx context.Context, path string) error {
	log.Print("Capturing the desktop in 3 seconds. Keep the game and square visible.")
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
	log.Printf("Saved desktop PNG %s (%d × %d).", path, im.Bounds().Dx(), im.Bounds().Dy())
	l, p, err := protocol.Find(im)
	if err != nil {
		return err
	}
	log.Printf("Valid tile at (%d,%d), %dpx cells; page %d/%d.", l.X, l.Y, l.Cell, p.Index+1, p.Count)
	return nil
}

func speakLoop(ctx context.Context, requests <-chan protocol.Message, speak func(context.Context, protocol.Message) error) {
	var cancel context.CancelFunc
	var finished chan error
	stopCurrent := func() {
		if cancel != nil {
			cancel()
			<-finished
			cancel = nil
			finished = nil
		}
	}
	defer stopCurrent()
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-requests:
			stopCurrent()
			child, c := context.WithCancel(ctx)
			cancel = c
			result := make(chan error, 1)
			finished = result
			go func() { result <- speak(child, message) }()
		case err := <-finished:
			if err != nil && ctx.Err() == nil {
				log.Printf("TTS error: %v", err)
			}
			cancel()
			cancel = nil
			finished = nil
		}
	}
}
