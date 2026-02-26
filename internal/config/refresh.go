package config

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// RefreshTimeout is the maximum time allowed for `opencode models --refresh`.
const RefreshTimeout = 30 * time.Second

// CommandRunner is the function used to run an external command.
// It is a package-level variable so tests can replace it without spawning
// real subprocesses.
//
// The function receives the command name and its arguments. It must return
// the combined stdout+stderr output and any error (including non-zero exit).
var CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error) = defaultCommandRunner

func defaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// RefreshModels runs `opencode models --refresh` to update the provider model
// registry before the config is loaded. This ensures the model picker always
// shows the latest available models.
//
// Returns an error if:
//   - the `opencode` binary is not found in PATH
//   - the command exits with a non-zero status
//   - the command exceeds RefreshTimeout
func RefreshModels() error {
	ctx, cancel := context.WithTimeout(context.Background(), RefreshTimeout)
	defer cancel()

	out, err := CommandRunner(ctx, "opencode", "models", "--refresh")
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded || err == context.DeadlineExceeded {
			return fmt.Errorf(
				"opencode models --refresh timed out after %s\n"+
					"  Check your network connection or API key configuration.",
				RefreshTimeout,
			)
		}
		if isNotFound(err) {
			return fmt.Errorf(
				"opencode binary not found in PATH\n" +
					"  Install opencode and ensure it is on your PATH, then retry.",
			)
		}
		return fmt.Errorf(
			"opencode models --refresh failed: %w\n  Output: %s",
			err, string(out),
		)
	}
	return nil
}

// isNotFound reports whether err indicates the binary was not found in PATH.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if err == exec.ErrNotFound {
		return true
	}
	// On some systems the error message describes the missing binary.
	msg := err.Error()
	return strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "no such file or directory")
}
