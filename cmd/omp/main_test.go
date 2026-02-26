package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anomalyco/opencode-model-preferences/internal/config"
)

// withCommandRunner temporarily replaces config.CommandRunner for the test.
func withCommandRunner(t *testing.T, fn func(ctx context.Context, name string, args ...string) ([]byte, error)) {
	t.Helper()
	orig := config.CommandRunner
	config.CommandRunner = fn
	t.Cleanup(func() { config.CommandRunner = orig })
}

// withConfigDir sets OPENCODE_CONFIG_DIR to a temp dir containing opencode.json
// with the given content, and restores the env var on cleanup.
func withConfigDir(t *testing.T, jsonContent string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	t.Setenv("OPENCODE_CONFIG_DIR", dir)
	return dir
}

func TestRun_RefreshCalledBeforeLoad(t *testing.T) {
	var refreshCalled bool

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		refreshCalled = true
		return nil, nil
	})

	// Config with at least one model so run() doesn't fail on "no models"
	withConfigDir(t, `{
		"provider": {
			"anthropic": {
				"models": {
					"claude-sonnet-4": {"name": "Claude Sonnet 4"}
				}
			}
		}
	}`)

	// run() will fail when it tries to start the TUI (no TTY in tests), but
	// we only care that refresh was called before that point.
	var buf bytes.Buffer
	_ = run(&buf)

	if !refreshCalled {
		t.Error("expected opencode models --refresh to be called on startup")
	}
}

func TestRun_RefreshFailureBlocksLaunch(t *testing.T) {
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, context.DeadlineExceeded
	})

	withConfigDir(t, `{"provider": {"anthropic": {"models": {"claude-sonnet-4": {}}}}}`)

	var buf bytes.Buffer
	err := run(&buf)

	if err == nil {
		t.Fatal("expected error when refresh fails, got nil")
	}
	if !strings.Contains(err.Error(), "refreshing models") {
		t.Errorf("error should mention 'refreshing models', got: %v", err)
	}
}

func TestRun_RefreshCommandIsCorrect(t *testing.T) {
	var gotName string
	var gotArgs []string

	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = args
		return nil, nil
	})

	withConfigDir(t, `{"provider": {"anthropic": {"models": {"claude-sonnet-4": {}}}}}`)

	var buf bytes.Buffer
	_ = run(&buf)

	if gotName != "opencode" {
		t.Errorf("refresh command = %q, want opencode", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "models" || gotArgs[1] != "--refresh" {
		t.Errorf("refresh args = %v, want [models --refresh]", gotArgs)
	}
}

func TestRun_NoModelsAfterRefresh(t *testing.T) {
	withCommandRunner(t, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, nil // refresh succeeds
	})

	// Config with no provider models
	withConfigDir(t, `{}`)

	var buf bytes.Buffer
	err := run(&buf)

	if err == nil {
		t.Fatal("expected error when no models found, got nil")
	}
	if !strings.Contains(err.Error(), "no models found") {
		t.Errorf("error should mention 'no models found', got: %v", err)
	}
}
