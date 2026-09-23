//go:build darwin

package platform

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

func Speak(ctx context.Context, text, voice string, rate int) error {
	if rate < -10 || rate > 10 {
		return fmt.Errorf("speech rate must be -10..10")
	}
	// Map the shared relative rate to say's words per minute. Text goes over
	// stdin so dialogue beginning with a dash cannot become a command option.
	args := []string{"-r", strconv.Itoa(int(175 * math.Pow(2, float64(rate)/10))), "-f", "-"}
	if voice != "" {
		args = append(args, "-v", voice)
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/say", args...)
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("macOS speech: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Voices(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "/usr/bin/say", "-v", "?").CombinedOutput()
	return string(out), err
}
