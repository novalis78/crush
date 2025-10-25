package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Scope defines where a memory is stored
type Scope string

const (
	ScopeSession Scope = "session"
	ScopeProject Scope = "project"
	ScopeGlobal  Scope = "global"
)

// Memory represents a stored piece of information
type Memory struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Scope     Scope     `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Store manages persistent memory storage
type Store struct {
	basePath   string
	workingDir string
	sessionID  string
}

// NewStore creates a new memory store
func NewStore(basePath, workingDir, sessionID string) *Store {
	return &Store{
		basePath:   basePath,
		workingDir: workingDir,
		sessionID:  sessionID,
	}
}

// Remember stores a new memory or updates an existing one
func (s *Store) Remember(key, value string, scope Scope, metadata map[string]string) error {
	memory := Memory{
		Key:       key,
		Value:     value,
		Scope:     scope,
		UpdatedAt: time.Now(),
		Metadata:  metadata,
	}

	// Check if memory exists to preserve CreatedAt
	existing, err := s.Recall(key, scope)
	if err == nil && existing != nil {
		memory.CreatedAt = existing.CreatedAt
	} else {
		memory.CreatedAt = time.Now()
	}

	path := s.getMemoryPath(scope)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Load existing memories
	memories, err := s.loadMemories(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load existing memories: %w", err)
	}

	// Update or append
	found := false
	for i, m := range memories {
		if m.Key == key {
			memories[i] = memory
			found = true
			break
		}
	}
	if !found {
		memories = append(memories, memory)
	}

	// Save back to file
	return s.saveMemories(path, memories)
}

// Recall retrieves a memory by key and scope
func (s *Store) Recall(key string, scope Scope) (*Memory, error) {
	path := s.getMemoryPath(scope)
	memories, err := s.loadMemories(path)
	if err != nil {
		return nil, err
	}

	for _, m := range memories {
		if m.Key == key {
			return &m, nil
		}
	}

	return nil, fmt.Errorf("memory not found: %s", key)
}

// Forget removes a memory
func (s *Store) Forget(key string, scope Scope) error {
	path := s.getMemoryPath(scope)
	memories, err := s.loadMemories(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to forget
		}
		return err
	}

	// Filter out the memory
	filtered := make([]Memory, 0, len(memories))
	for _, m := range memories {
		if m.Key != key {
			filtered = append(filtered, m)
		}
	}

	return s.saveMemories(path, filtered)
}

// List returns all memories for a scope
func (s *Store) List(scope Scope) ([]Memory, error) {
	path := s.getMemoryPath(scope)
	return s.loadMemories(path)
}

// getMemoryPath returns the file path for a given scope
func (s *Store) getMemoryPath(scope Scope) string {
	switch scope {
	case ScopeSession:
		return filepath.Join(s.basePath, "sessions", s.sessionID, "memory.json")
	case ScopeProject:
		projectHash := s.hashWorkingDir()
		return filepath.Join(s.basePath, "projects", projectHash, "memory.json")
	case ScopeGlobal:
		return filepath.Join(s.basePath, "global", "memory.json")
	default:
		return filepath.Join(s.basePath, "global", "memory.json")
	}
}

// hashWorkingDir creates a unique hash for the working directory
func (s *Store) hashWorkingDir() string {
	hash := sha256.Sum256([]byte(s.workingDir))
	return hex.EncodeToString(hash[:])[:16] // First 16 chars
}

// loadMemories loads memories from a file
func (s *Store) loadMemories(path string) ([]Memory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Memory{}, nil
		}
		return nil, err
	}

	var memories []Memory
	if err := json.Unmarshal(data, &memories); err != nil {
		return nil, fmt.Errorf("failed to parse memories: %w", err)
	}

	return memories, nil
}

// saveMemories saves memories to a file
func (s *Store) saveMemories(path string, memories []Memory) error {
	data, err := json.MarshalIndent(memories, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal memories: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write memories: %w", err)
	}

	return nil
}
