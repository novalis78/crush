package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

// AppInterface defines the minimal interface we need from crush App
type AppInterface interface {
	CoderAgentRun(ctx context.Context, sessionID, prompt string) (<-chan struct{}, error)
	CoderAgentGetLastResponse() string
	SessionsCreate(ctx context.Context, title string) (string, error)
	PermissionsAutoApprove(sessionID string)
}

// Service is the main MCP heartbeat service
type Service struct {
	ctx            context.Context
	cancel         context.CancelFunc
	app            AppInterface
	contextMgr     *ContextManager
	promptBuilder  *PromptBuilder
	interval       time.Duration
	cycleNumber    int
	running        bool
	pidFile        string
	logger         *slog.Logger
}

// NewService creates a new heartbeat service
func NewService(interval time.Duration, app AppInterface) *Service {
	homeDir, _ := os.UserHomeDir()
	pidFile := filepath.Join(homeDir, ".mcp", "heartbeat.pid")

	ctx, cancel := context.WithCancel(context.Background())

	return &Service{
		ctx:           ctx,
		cancel:        cancel,
		app:           app,
		contextMgr:    NewContextManager(),
		promptBuilder: &PromptBuilder{},
		interval:      interval,
		pidFile:       pidFile,
		logger:        slog.Default(),
	}
}

// Start starts the heartbeat service
func (s *Service) Start() error {
	// Check if already running
	if err := s.acquireLock(); err != nil {
		return err
	}
	defer s.releaseLock()

	s.running = true
	s.logger.Info("🤖 MCP Heartbeat Service starting",
		"interval", s.interval,
		"pid", os.Getpid())

	// Setup signal handlers
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Load initial state
	ctx, err := s.contextMgr.LoadContext()
	if err != nil {
		return fmt.Errorf("failed to load initial context: %w", err)
	}
	s.cycleNumber = ctx.Metadata.TotalCycles

	// Log startup
	s.contextMgr.AppendLog(fmt.Sprintf(
		"**Cycle %d - Heartbeat Started**\n\nHeartbeat service activated. Interval: %s\n",
		s.cycleNumber, s.interval))

	// Main loop
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("📡 Ticker started", "interval", s.interval)

	for s.running {
		s.logger.Info("⏳ Waiting for next tick...")
		select {
		case t := <-ticker.C:
			s.logger.Info("⏰ Ticker fired!", "time", t)
			if err := s.runCycle(); err != nil {
				s.logger.Error("Cycle failed", "error", err)
			}
			s.logger.Info("🔄 Cycle complete, back to select loop")

		case sig := <-sigChan:
			s.logger.Info("Received shutdown signal", "signal", sig)
			s.running = false

		case <-s.ctx.Done():
			s.running = false
		}
	}

	s.logger.Info("🛑 MCP Heartbeat Service stopped")
	s.contextMgr.AppendLog(fmt.Sprintf(
		"**Cycle %d - Heartbeat Stopped**\n\nHeartbeat service deactivated.\n",
		s.cycleNumber))

	return nil
}

// Stop stops the heartbeat service
func (s *Service) Stop() {
	s.running = false
	s.cancel()
}

// runCycle executes a single heartbeat cycle
func (s *Service) runCycle() error {
	s.cycleNumber++
	startTime := time.Now()

	s.logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	s.logger.Info("🔄 MCP Heartbeat Cycle", "number", s.cycleNumber)

	// Load current state
	ctx, err := s.contextMgr.LoadContext()
	if err != nil {
		return fmt.Errorf("failed to load context: %w", err)
	}

	goals, err := s.contextMgr.LoadGoals()
	if err != nil {
		return fmt.Errorf("failed to load goals: %w", err)
	}

	mission, err := s.contextMgr.LoadMission()
	if err != nil {
		return fmt.Errorf("failed to load mission: %w", err)
	}

	s.logger.Info("📋 State loaded",
		"observations", len(ctx.Observations),
		"lessons", len(ctx.Lessons),
		"active_goals", len(goals.Goals))

	// Build prompt
	userPrompt := s.promptBuilder.BuildUserPrompt(ctx, goals, mission, s.cycleNumber)

	// Create session for this cycle
	sessionTitle := fmt.Sprintf("MCP Heartbeat Cycle %d", s.cycleNumber)
	sessionID, err := s.app.SessionsCreate(s.ctx, sessionTitle)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Auto-approve all permissions for autonomous operation
	s.app.PermissionsAutoApprove(sessionID)

	s.logger.Info("🧠 Executing autonomous cycle...")

	// Run the agent
	done, err := s.app.CoderAgentRun(s.ctx, sessionID, userPrompt)
	if err != nil {
		return fmt.Errorf("failed to run agent: %w", err)
	}

	// Wait for completion
	<-done

	// Get the response
	response := s.app.CoderAgentGetLastResponse()

	s.logger.Info("✅ Agent execution complete")

	// Parse memory commands from response
	memoryCommands := parseMemoryCommands(response)

	if len(memoryCommands) > 0 {
		s.logger.Info("💾 Processing memory commands", "count", len(memoryCommands))

		// Backup before modifying
		s.contextMgr.BackupContext(fmt.Sprintf("cycle_%d", s.cycleNumber))

		// Execute memory commands
		for _, cmd := range memoryCommands {
			if err := s.contextMgr.ExecuteMemoryCommand(ctx, cmd, s.cycleNumber); err != nil {
				s.logger.Warn("Failed to execute memory command", "command", cmd.Command, "error", err)
			} else {
				s.logger.Info("  ✓ "+cmd.Command, "reason", cmd.Reason)
			}
		}
	}

	// Update context
	ctx.Metadata.TotalCycles = s.cycleNumber
	if err := s.contextMgr.SaveContext(ctx); err != nil {
		return fmt.Errorf("failed to save context: %w", err)
	}

	duration := time.Since(startTime)
	s.logger.Info("✅ Cycle complete", "duration", duration)

	// Extract brief summary from response (first line or first 200 chars)
	summary := extractSummary(response)

	// Log to mission log
	logEntry := fmt.Sprintf(
		"**Cycle %d**\n\n"+
		"Summary: %s\n"+
		"Memory Updates: %d commands\n"+
		"Duration: %s\n"+
		"Status: ✅ Success\n",
		s.cycleNumber, summary, len(memoryCommands), duration)

	if err := s.contextMgr.AppendLog(logEntry); err != nil {
		s.logger.Warn("Failed to append to log", "error", err)
	}

	s.logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	s.logger.Info("")

	return nil
}

// parseMemoryCommands extracts memory commands from the agent's response
func parseMemoryCommands(response string) []MemoryCommand {
	// Look for MEMORY_COMMANDS: section with JSON array
	re := regexp.MustCompile(`(?s)MEMORY_COMMANDS:\s*(\[.*?\](?:\s*\n|$))`)
	matches := re.FindStringSubmatch(response)

	if len(matches) < 2 {
		return nil
	}

	var commands []MemoryCommand
	if err := json.Unmarshal([]byte(matches[1]), &commands); err != nil {
		slog.Warn("Failed to parse memory commands", "error", err)
		return nil
	}

	return commands
}

// extractSummary extracts a brief summary from the response
func extractSummary(response string) string {
	// Try to get the first line of content
	lines := regexp.MustCompile(`\r?\n`).Split(response, -1)
	for _, line := range lines {
		trimmed := regexp.MustCompile(`^[#\s*-]+`).ReplaceAllString(line, "")
		if len(trimmed) > 0 {
			if len(trimmed) > 150 {
				return trimmed[:147] + "..."
			}
			return trimmed
		}
	}

	// Fallback to first 150 chars
	if len(response) > 150 {
		return response[:147] + "..."
	}
	return response
}

// acquireLock creates a PID file to prevent multiple instances
func (s *Service) acquireLock() error {
	// Check if PID file exists
	if data, err := os.ReadFile(s.pidFile); err == nil {
		// File exists, check if process is running
		var oldPID int
		fmt.Sscanf(string(data), "%d", &oldPID)

		// Check if process exists
		if process, err := os.FindProcess(oldPID); err == nil {
			// On Unix, FindProcess always succeeds. Try to signal it to check if alive
			if err := process.Signal(syscall.Signal(0)); err == nil {
				return fmt.Errorf("heartbeat already running with PID %d", oldPID)
			}
		}

		// Stale PID file, remove it
		s.logger.Warn("Removing stale PID file", "old_pid", oldPID)
		os.Remove(s.pidFile)
	}

	// Create new PID file
	pid := os.Getpid()
	if err := os.WriteFile(s.pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("failed to create PID file: %w", err)
	}

	return nil
}

// releaseLock removes the PID file
func (s *Service) releaseLock() {
	// Only remove if it contains our PID (safety check)
	if data, err := os.ReadFile(s.pidFile); err == nil {
		var filePID int
		fmt.Sscanf(string(data), "%d", &filePID)
		if filePID == os.Getpid() {
			os.Remove(s.pidFile)
		}
	}
}

// GetStatus returns the current status of the heartbeat service
func GetStatus() (bool, int, error) {
	homeDir, _ := os.UserHomeDir()
	pidFile := filepath.Join(homeDir, ".mcp", "heartbeat.pid")

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil // Not running
		}
		return false, 0, err
	}

	var pid int
	fmt.Sscanf(string(data), "%d", &pid)

	// Check if process is actually running
	if process, err := os.FindProcess(pid); err == nil {
		if err := process.Signal(syscall.Signal(0)); err == nil {
			return true, pid, nil // Running
		}
	}

	// Stale PID file
	return false, 0, nil
}
