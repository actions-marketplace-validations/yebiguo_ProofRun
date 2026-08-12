package runner

import (
	"bytes"
	"context"
	"runtime"
	"testing"
	"time"
)

func echoCommand(msg string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/C", "echo", msg}
	}
	return []string{"echo", msg}
}

func exitCommand(code int) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/C", "exit", "/B", itoa(code)}
	}
	return []string{"sh", "-c", "exit " + itoa(code)}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestRun_Success(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result, err := Run(context.Background(), t.TempDir(), echoCommand("hello"), 0, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.TimedOut {
		t.Fatal("TimedOut = true for a command that completed normally")
	}
	if result.Duration <= 0 {
		t.Fatal("Duration should be positive")
	}
}

func TestRun_NonZeroExit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result, err := Run(context.Background(), t.TempDir(), exitCommand(3), 0, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() returned an error for a normal non-zero exit: %v", err)
	}
	if result.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3", result.ExitCode)
	}
}

func TestRun_CommandNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	_, err := Run(context.Background(), t.TempDir(), []string{"proofrun-definitely-not-a-real-binary"}, 0, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run() error = nil, want an error when the binary does not exist")
	}
}

func TestRun_EmptyCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	_, err := Run(context.Background(), t.TempDir(), nil, 0, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run() error = nil, want an error for an empty command")
	}
}

func TestRun_Timeout(t *testing.T) {
	var sleepCmd []string
	if runtime.GOOS == "windows" {
		sleepCmd = []string{"cmd", "/C", "ping", "-n", "10", "127.0.0.1"}
	} else {
		sleepCmd = []string{"sleep", "5"}
	}

	var stdout, stderr bytes.Buffer
	result, err := Run(context.Background(), t.TempDir(), sleepCmd, 200*time.Millisecond, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !result.TimedOut {
		t.Fatal("TimedOut = false for a command that should have been killed")
	}
}
