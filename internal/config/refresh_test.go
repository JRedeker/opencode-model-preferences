package config

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// withCommandRunner temporarily replaces CommandRunner for the duration of the
// test and restores it on cleanup.
func withCommandRunner(t *testing.T, fn func(ctx context.Context, name string, args ...string) ([]byte, error)) {
	t.Helper()
	orig := CommandRunner
	CommandRunner = fn
	t.Cleanup(func() { CommandRunner = orig })
}

func TestRefreshModels_Success(t *testing.T) {
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "opencode" || len(args) < 2 || args[0] != "models" || args[1] != "--refresh" {
			t.Errorf("unexpected command: %s %v", name, args)
		}
		return []byte("refreshed 42 models"), nil
	})

	if err := RefreshModels(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRefreshModels_NonZeroExit(t *testing.T) {
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("API error: unauthorized"), &exec.ExitError{}
	})

	err := RefreshModels()
	if err == nil {
		t.Fatal("expected error on non-zero exit, got nil")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("error should mention 'failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "API error: unauthorized") {
		t.Errorf("error should include command output, got: %v", err)
	}
}

func TestRefreshModels_BinaryNotFound(t *testing.T) {
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, exec.ErrNotFound
	})

	err := RefreshModels()
	if err == nil {
		t.Fatal("expected error when binary not found, got nil")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error should mention PATH, got: %v", err)
	}
}

func TestRefreshModels_BinaryNotFoundByMessage(t *testing.T) {
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("executable file not found in $PATH")
	})

	err := RefreshModels()
	if err == nil {
		t.Fatal("expected error when binary not found, got nil")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error should mention PATH, got: %v", err)
	}
}

func TestRefreshModels_Timeout(t *testing.T) {
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Simulate the context being cancelled (deadline exceeded)
		<-ctx.Done()
		return nil, ctx.Err()
	})

	// Override timeout to something very short so the test doesn't take 30s
	orig := RefreshTimeout
	// We can't change the const, but we can test the timeout path by making
	// the runner return a deadline-exceeded error directly.
	_ = orig

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, context.DeadlineExceeded
	})

	err := RefreshModels()
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should mention timeout, got: %v", err)
	}
}

func TestRefreshModels_CommandArgs(t *testing.T) {
	var gotName string
	var gotArgs []string

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = args
		return nil, nil
	})

	_ = RefreshModels()

	if gotName != "opencode" {
		t.Errorf("command name = %q, want opencode", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "models" || gotArgs[1] != "--refresh" {
		t.Errorf("args = %v, want [models --refresh]", gotArgs)
	}
}

func TestRefreshModels_ContextHasTimeout(t *testing.T) {
	var deadline time.Time
	var hasDeadline bool

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		deadline, hasDeadline = ctx.Deadline()
		return nil, nil
	})

	before := time.Now()
	_ = RefreshModels()

	if !hasDeadline {
		t.Error("context should have a deadline set")
	}
	if deadline.Before(before) {
		t.Error("deadline should be in the future")
	}
	if deadline.After(before.Add(RefreshTimeout + time.Second)) {
		t.Errorf("deadline %v is too far in the future (expected ~%s)", deadline, RefreshTimeout)
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"exec.ErrNotFound", exec.ErrNotFound, true},
		{"executable file not found", errors.New("executable file not found in $PATH"), true},
		{"no such file or directory", errors.New("fork/exec /usr/bin/opencode: no such file or directory"), true},
		{"exit error", &exec.ExitError{}, false},
		{"generic error", errors.New("some other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNotFound(tt.err)
			if got != tt.want {
				t.Errorf("isNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
