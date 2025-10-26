package heartbeat

import "time"

// Context represents the MCP's self-managed memory and learned knowledge
type Context struct {
	Observations []Observation `json:"observations"`
	Lessons      []Lesson      `json:"lessons"`
	Hypotheses   []Hypothesis  `json:"hypotheses"`
	Strategies   []Strategy    `json:"strategies"`
	Metadata     ContextMeta   `json:"metadata"`
}

// Observation is something the MCP has learned or noticed
type Observation struct {
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Cycle     int       `json:"cycle"`
}

// Lesson is a codified pattern or rule the MCP has discovered
type Lesson struct {
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Cycle     int       `json:"cycle"`
	Confidence string   `json:"confidence,omitempty"` // "high", "medium", "low"
}

// Hypothesis is a theory the MCP is testing
type Hypothesis struct {
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
	Cycle      int       `json:"cycle"`
	Status     string    `json:"status"` // "testing", "validated", "rejected"
	Evidence   []string  `json:"evidence,omitempty"`
}

// Strategy is an approach the MCP uses to accomplish goals
type Strategy struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Cycle       int       `json:"cycle"`
	Effectiveness string  `json:"effectiveness,omitempty"` // "works", "partial", "failed"
}

// ContextMeta holds metadata about the context
type ContextMeta struct {
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TotalCycles int       `json:"total_cycles"`
}

// Goals represents the active goals the MCP is working toward
type Goals struct {
	Goals    []Goal    `json:"goals"`
	Metadata GoalsMeta `json:"metadata"`
}

// Goal is a specific objective
type Goal struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    string    `json:"priority"` // "HIGH", "MEDIUM", "LOW"
	Status      string    `json:"status"`   // "active", "completed", "paused"
	CreatedAt   time.Time `json:"created_at"`
	Progress    []string  `json:"progress"` // Log of progress updates
}

// GoalsMeta holds metadata about goals
type GoalsMeta struct {
	NextID    int       `json:"next_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MemoryCommand is a command from the model to update its own context
type MemoryCommand struct {
	Command string                 `json:"command"` // "add_observation", "add_lesson", etc.
	Reason  string                 `json:"reason"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// CycleResult represents the output of a single heartbeat cycle
type CycleResult struct {
	CycleNumber    int
	StartTime      time.Time
	EndTime        time.Time
	Decision       string
	ToolsUsed      []string
	MemoryCommands []MemoryCommand
	Summary        string
	Success        bool
	Error          error
}
