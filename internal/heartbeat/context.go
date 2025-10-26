package heartbeat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultMCPDir = ".mcp"
	contextFile   = "context.json"
	goalsFile     = "active-goals.json"
	missionFile   = "mission.md"
	logFile       = "mission-log.md"
	backupDir     = "backups"
)

// ContextManager handles loading and saving MCP state
type ContextManager struct {
	mcpDir string
}

// NewContextManager creates a new context manager
func NewContextManager() *ContextManager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to relative path
		return &ContextManager{mcpDir: defaultMCPDir}
	}
	return &ContextManager{
		mcpDir: filepath.Join(homeDir, defaultMCPDir),
	}
}

// LoadContext loads the current context from disk
func (cm *ContextManager) LoadContext() (*Context, error) {
	path := filepath.Join(cm.mcpDir, contextFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty context if file doesn't exist
			return cm.createEmptyContext(), nil
		}
		return nil, fmt.Errorf("failed to read context file: %w", err)
	}

	var ctx Context
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to parse context JSON: %w", err)
	}

	return &ctx, nil
}

// SaveContext saves the context to disk
func (cm *ContextManager) SaveContext(ctx *Context) error {
	// Update timestamp
	ctx.Metadata.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	path := filepath.Join(cm.mcpDir, contextFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}

	return nil
}

// BackupContext creates a timestamped backup of the current context
func (cm *ContextManager) BackupContext(reason string) error {
	srcPath := filepath.Join(cm.mcpDir, contextFile)

	// Check if source exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil // Nothing to backup
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(cm.mcpDir, backupDir, fmt.Sprintf("context_%s_%s.json", timestamp, reason))

	// Ensure backup directory exists
	if err := os.MkdirAll(filepath.Join(cm.mcpDir, backupDir), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read context for backup: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}

// LoadGoals loads the active goals from disk
func (cm *ContextManager) LoadGoals() (*Goals, error) {
	path := filepath.Join(cm.mcpDir, goalsFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Goals{Goals: []Goal{}, Metadata: GoalsMeta{NextID: 1}}, nil
		}
		return nil, fmt.Errorf("failed to read goals file: %w", err)
	}

	var goals Goals
	if err := json.Unmarshal(data, &goals); err != nil {
		return nil, fmt.Errorf("failed to parse goals JSON: %w", err)
	}

	return &goals, nil
}

// SaveGoals saves the goals to disk
func (cm *ContextManager) SaveGoals(goals *Goals) error {
	goals.Metadata.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(goals, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal goals: %w", err)
	}

	path := filepath.Join(cm.mcpDir, goalsFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write goals file: %w", err)
	}

	return nil
}

// LoadMission loads the mission statement from disk
func (cm *ContextManager) LoadMission() (string, error) {
	path := filepath.Join(cm.mcpDir, missionFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read mission file: %w", err)
	}

	return string(data), nil
}

// AppendLog appends an entry to the mission log
func (cm *ContextManager) AppendLog(entry string) error {
	path := filepath.Join(cm.mcpDir, logFile)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05 MST")
	logEntry := fmt.Sprintf("\n### %s\n%s\n", timestamp, entry)

	if _, err := f.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
	}

	return nil
}

// ExecuteMemoryCommand executes a memory command from the model
func (cm *ContextManager) ExecuteMemoryCommand(ctx *Context, cmd MemoryCommand, cycle int) error {
	switch cmd.Command {
	case "add_observation":
		content, ok := cmd.Data["content"].(string)
		if !ok {
			return fmt.Errorf("observation content must be a string")
		}
		ctx.Observations = append(ctx.Observations, Observation{
			Content:   content,
			Timestamp: time.Now(),
			Cycle:     cycle,
		})

	case "add_lesson":
		content, ok := cmd.Data["content"].(string)
		if !ok {
			return fmt.Errorf("lesson content must be a string")
		}
		confidence, _ := cmd.Data["confidence"].(string)
		ctx.Lessons = append(ctx.Lessons, Lesson{
			Content:    content,
			Timestamp:  time.Now(),
			Cycle:      cycle,
			Confidence: confidence,
		})

	case "add_hypothesis":
		content, ok := cmd.Data["content"].(string)
		if !ok {
			return fmt.Errorf("hypothesis content must be a string")
		}
		ctx.Hypotheses = append(ctx.Hypotheses, Hypothesis{
			Content:   content,
			Timestamp: time.Now(),
			Cycle:     cycle,
			Status:    "testing",
		})

	case "add_strategy":
		name, okName := cmd.Data["name"].(string)
		desc, okDesc := cmd.Data["description"].(string)
		if !okName || !okDesc {
			return fmt.Errorf("strategy requires name and description strings")
		}
		ctx.Strategies = append(ctx.Strategies, Strategy{
			Name:        name,
			Description: desc,
			Timestamp:   time.Now(),
			Cycle:       cycle,
		})

	case "prune_old":
		// TODO: Implement pruning logic based on age/cycle count
		// For now, just log that we would prune
		return nil

	default:
		return fmt.Errorf("unknown memory command: %s", cmd.Command)
	}

	return nil
}

// createEmptyContext creates a new empty context
func (cm *ContextManager) createEmptyContext() *Context {
	now := time.Now()
	return &Context{
		Observations: []Observation{},
		Lessons:      []Lesson{},
		Hypotheses:   []Hypothesis{},
		Strategies:   []Strategy{},
		Metadata: ContextMeta{
			Version:     "1.0",
			CreatedAt:   now,
			UpdatedAt:   now,
			TotalCycles: 0,
		},
	}
}
