//go:build windows

package platform

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

func powershell(ctx context.Context, script, input string) error {
	units := utf16.Encode([]rune(script))
	b := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(b[i*2:], u)
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Windows speech: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Text and voice names enter through JSON on stdin, never executable script.
func Speak(ctx context.Context, text, voice string, rate int) error {
	data, _ := json.Marshal(struct {
		Text, Voice string
		Rate        int
	}{text, voice, rate})
	return powershell(ctx, `$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = New-Object System.Text.UTF8Encoding($false)
$p = [Console]::In.ReadToEnd() | ConvertFrom-Json
Add-Type -AssemblyName System.Speech
$s = New-Object System.Speech.Synthesis.SpeechSynthesizer
try {
    $s.SetOutputToDefaultAudioDevice()
    if ($p.Voice) { $s.SelectVoice($p.Voice) }
    $s.Rate = [int]$p.Rate
    $s.Speak([string]$p.Text)
} finally { $s.Dispose() }`, string(data))
}

func Voices(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false); Add-Type -AssemblyName System.Speech; $s=New-Object System.Speech.Synthesis.SpeechSynthesizer; try { $s.GetInstalledVoices() | ForEach-Object { $_.VoiceInfo.Name } } finally { $s.Dispose() }`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	b, err := cmd.CombinedOutput()
	return string(b), err
}
