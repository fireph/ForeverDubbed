package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"time"

	"foreverdubbed/internal/appstate"
	"foreverdubbed/internal/buildinfo"
	"foreverdubbed/internal/identity"
	"foreverdubbed/internal/platform"
	"foreverdubbed/internal/pocket"
	"foreverdubbed/internal/protocol"
	"foreverdubbed/internal/speech"
)

var version = buildinfo.Version

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
	var backend, configPath, raceConfigPath, testText, testRace, testGender string
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
	flag.StringVar(&raceConfigPath, "race-config", identity.DefaultCustomPath(), "custom NPC/model race JSON (default: bundled data/custom-races.json)")
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
	identities, err := identity.Load(raceConfigPath)
	if err != nil {
		return fmt.Errorf("race config: %w", err)
	}
	if files != "" {
		return decodeImages(files, identities, os.Stdout)
	}
	guiMode := desktopEnabled && !headless && !list && testText == "" && snapshot == ""
	// The desktop Voices tab lists races from the voice config, so GUI mode
	// needs it loaded before the window opens.
	var config *speech.Config
	var configErr error
	if guiMode {
		config, configErr = speech.Load(configPath)
	}
	state := appstate.New(captureApp, backend, mute)
	work := func() error {
		// Report configuration failures inside the window, like other startup
		// errors. Capture-only mode can still run without a voice configuration.
		if configErr != nil && !mute && backend != "system" {
			return fmt.Errorf("voice config: %w", configErr)
		}
		var speak func(context.Context, protocol.Message) error
		if !mute && snapshot == "" || list || testText != "" {
			if backend != "system" {
				if config == nil {
					loaded, err := speech.Load(configPath)
					if err != nil {
						return fmt.Errorf("voice config: %w", err)
					}
					config = loaded
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
				local.ChoiceSource = state.VoiceChoices
				local.OnVoice = func(m protocol.Message, id string) {
					log.Printf("Voice %s (race=%q gender=%q NPC=%q)", id, m.Race, m.Gender, m.NPCID)
					state.Update(func(v *appstate.Snapshot) { v.Voice = id })
				}
				log.Print("PocketTTS.cpp: native CPU streaming ready")
				speak = func(ctx context.Context, m protocol.Message) error {
					ctx = platform.WithPlaybackObserver(ctx, func() { state.Audio("Playing audio") })
					ctx = platform.WithPlaybackProgress(ctx, func(played, total time.Duration, known bool) {
						state.Update(func(v *appstate.Snapshot) { v.Played, v.Duration, v.DurationKnown = played, total, known })
					})
					return local.Speak(ctx, m)
				}
			} else {
				speak = func(ctx context.Context, m protocol.Message) error {
					if speech.VoiceMuted(state.VoiceChoices(), m.Race, m.Gender) {
						return nil
					}
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
		return captureLoop(ctx, captureSettings{poll: poll, scan: scan, emitJSON: !guiMode, mute: mute}, state, identities, speak)
	}
	if guiMode {
		var races []string
		if config != nil {
			for race := range config.Races {
				races = append(races, race)
			}
			sort.Strings(races)
		}
		return runDesktop(ctx, stop, state, races, work)
	}
	return work()
}
