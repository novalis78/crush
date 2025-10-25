package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/memory"
)

//go:embed memory.md
var memoryDescription []byte

const MemoryToolName = "memory"

type MemoryParams struct {
	Action   string            `json:"action"`   // remember, recall, forget, list
	Key      string            `json:"key,omitempty"`
	Value    string            `json:"value,omitempty"`
	Scope    string            `json:"scope,omitempty"` // session, project, global (defaults to project)
	Metadata map[string]string `json:"metadata,omitempty"`
}

type memoryTool struct {
	basePath   string
	workingDir string
}

func NewMemoryTool(basePath, workingDir string) BaseTool {
	return &memoryTool{
		basePath:   basePath,
		workingDir: workingDir,
	}
}

func (m *memoryTool) Name() string {
	return MemoryToolName
}

func (m *memoryTool) Info() ToolInfo {
	return ToolInfo{
		Name:        MemoryToolName,
		Description: string(memoryDescription),
		Parameters: map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "The action to perform: remember, recall, forget, or list",
				"enum":        []string{"remember", "recall", "forget", "list"},
			},
			"key": map[string]any{
				"type":        "string",
				"description": "The key to store/retrieve the memory under (required for remember, recall, forget)",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "The value to store (required for remember action)",
			},
			"scope": map[string]any{
				"type":        "string",
				"description": "The scope of the memory: session (current session only), project (this project), global (all projects). Defaults to project.",
				"enum":        []string{"session", "project", "global"},
			},
			"metadata": map[string]any{
				"type":        "object",
				"description": "Optional metadata tags for the memory",
			},
		},
		Required: []string{"action"},
	}
}

func (m *memoryTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params MemoryParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	// Get session ID from context
	sessionID, _ := GetContextValues(ctx)
	if sessionID == "" {
		return NewTextErrorResponse("session ID not found in context"), nil
	}

	// Create memory store with current session ID
	store := memory.NewStore(m.basePath, m.workingDir, sessionID)

	// Default scope to project
	if params.Scope == "" {
		params.Scope = string(memory.ScopeProject)
	}

	scope := memory.Scope(params.Scope)

	// Validate scope
	if scope != memory.ScopeSession && scope != memory.ScopeProject && scope != memory.ScopeGlobal {
		return NewTextErrorResponse(fmt.Sprintf("invalid scope: %s (must be session, project, or global)", params.Scope)), nil
	}

	switch params.Action {
	case "remember":
		return m.handleRemember(store, params, scope)
	case "recall":
		return m.handleRecall(store, params, scope)
	case "forget":
		return m.handleForget(store, params, scope)
	case "list":
		return m.handleList(store, scope)
	default:
		return NewTextErrorResponse(fmt.Sprintf("unknown action: %s", params.Action)), nil
	}
}

func (m *memoryTool) handleRemember(store *memory.Store, params MemoryParams, scope memory.Scope) (ToolResponse, error) {
	if params.Key == "" {
		return NewTextErrorResponse("key is required for remember action"), nil
	}
	if params.Value == "" {
		return NewTextErrorResponse("value is required for remember action"), nil
	}

	err := store.Remember(params.Key, params.Value, scope, params.Metadata)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to remember: %w", err)
	}

	return NewTextResponse(fmt.Sprintf("✓ Remembered '%s' in %s scope", params.Key, scope)), nil
}

func (m *memoryTool) handleRecall(store *memory.Store, params MemoryParams, scope memory.Scope) (ToolResponse, error) {
	if params.Key == "" {
		return NewTextErrorResponse("key is required for recall action"), nil
	}

	mem, err := store.Recall(params.Key, scope)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Memory not found: %s", params.Key)), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# Memory: %s\n\n", mem.Key))
	output.WriteString(fmt.Sprintf("**Value:** %s\n\n", mem.Value))
	output.WriteString(fmt.Sprintf("**Scope:** %s\n", mem.Scope))
	output.WriteString(fmt.Sprintf("**Created:** %s\n", mem.CreatedAt.Format("2006-01-02 15:04:05")))
	output.WriteString(fmt.Sprintf("**Updated:** %s\n", mem.UpdatedAt.Format("2006-01-02 15:04:05")))

	if len(mem.Metadata) > 0 {
		output.WriteString("\n**Metadata:**\n")
		for k, v := range mem.Metadata {
			output.WriteString(fmt.Sprintf("  - %s: %s\n", k, v))
		}
	}

	return NewTextResponse(output.String()), nil
}

func (m *memoryTool) handleForget(store *memory.Store, params MemoryParams, scope memory.Scope) (ToolResponse, error) {
	if params.Key == "" {
		return NewTextErrorResponse("key is required for forget action"), nil
	}

	err := store.Forget(params.Key, scope)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to forget: %w", err)
	}

	return NewTextResponse(fmt.Sprintf("✓ Forgot '%s' from %s scope", params.Key, scope)), nil
}

func (m *memoryTool) handleList(store *memory.Store, scope memory.Scope) (ToolResponse, error) {
	memories, err := store.List(scope)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to list memories: %w", err)
	}

	if len(memories) == 0 {
		return NewTextResponse(fmt.Sprintf("No memories found in %s scope", scope)), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("# Memories (%s scope)\n\n", scope))
	output.WriteString(fmt.Sprintf("Found %d memories:\n\n", len(memories)))

	for i, mem := range memories {
		output.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, mem.Key))

		// Truncate long values for list view
		value := mem.Value
		if len(value) > 100 {
			value = value[:97] + "..."
		}
		output.WriteString(fmt.Sprintf("   Value: %s\n", value))
		output.WriteString(fmt.Sprintf("   Updated: %s\n", mem.UpdatedAt.Format("2006-01-02 15:04:05")))

		if len(mem.Metadata) > 0 {
			tags := make([]string, 0, len(mem.Metadata))
			for k, v := range mem.Metadata {
				tags = append(tags, fmt.Sprintf("%s:%s", k, v))
			}
			output.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(tags, ", ")))
		}
		output.WriteString("\n")
	}

	return NewTextResponse(output.String()), nil
}
