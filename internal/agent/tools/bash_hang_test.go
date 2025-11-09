package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestGitPushHangDetection(t *testing.T) {
	t.Parallel()

	// Create a simulated git push that produces no output for 11 seconds.
	bgManager := shell.GetBackgroundShellManager()
	bgManager.Cleanup()

	// Start a command that simulates git push hanging.
	command := "git push origin main & sleep 11"
	bgShell, err := bgManager.Start(context.Background(), t.TempDir(), nil, command, "test hang")
	require.NoError(t, err)

	// Simulate the hang detection logic from bash.go.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(1 * time.Minute)

	var stdout, stderr string
	var done bool
	var prevOutputLen int
	var noOutputDuration time.Duration
	lastOutputCheck := time.Now()
	isGitCommand := strings.Contains(command, "git push")

	hangDetected := false

waitLoop:
	for {
		select {
		case <-ticker.C:
			stdout, stderr, done, _ = bgShell.GetOutput()
			if done {
				break waitLoop
			}

			// Detect SSH/git hangs: no new output for 10 seconds on git operations.
			if isGitCommand {
				currentOutputLen := len(stdout) + len(stderr)
				timeSinceLastCheck := time.Since(lastOutputCheck)

				if currentOutputLen == prevOutputLen {
					noOutputDuration += timeSinceLastCheck
					if noOutputDuration > 10*time.Second {
						// Hang detected.
						hangDetected = true
						bgManager.Kill(bgShell.ID)
						break waitLoop
					}
				} else {
					noOutputDuration = 0
				}

				prevOutputLen = currentOutputLen
				lastOutputCheck = time.Now()
			}
		case <-timeout:
			break waitLoop
		}
	}

	require.True(t, hangDetected, "Expected hang detection to trigger")
}
