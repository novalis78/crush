# MCP Heartbeat Service - Design Document

## Vision

Transform crush from a on-demand coding assistant into a continuously-running Master Control Program that monitors, plans, learns, and executes autonomously - aligned with the vision in `/home/novalis78/MCP/READ_FIRST.md`.

## Inspired By: Trading System Pattern

The `/home/novalis78/Trading` system demonstrates a working autonomous loop:
- `trading.py` runs continuously (60s cycles)
- Builds dynamic prompts from market data + evolving context
- Model manages its own memory via JSON commands
- System prompt (`system_prompt_v5.md`) guides behavior
- Context file (`trading_context.json`) stores observations/lessons
- Gets better over time through self-reflection

**We want the same for MCP, but generalized for empire building.**

## Architecture

### Two Modes of Operation

#### 1. Heartbeat Mode (Background Daemon)
```
┌──────────────────────────────────────────────────┐
│ crush-heartbeat (runs 24/7 in background)        │
├──────────────────────────────────────────────────┤
│ Loop (every N minutes):                          │
│   1. Load mission context + goals                │
│   2. Monitor systems (accounts, email, etc.)     │
│   3. Check for pending tasks/opportunities       │
│   4. Build dynamic prompt with context           │
│   5. Model decides what to work on               │
│   6. Execute using crush tools:                  │
│      - websearch (research)                      │
│      - read/write/edit (code/docs)               │
│      - bash (run commands)                       │
│      - memory (store insights)                   │
│   7. Model updates context via JSON commands     │
│   8. Log progress to mission-log.md              │
│   9. Sync state to persistent storage            │
│  10. Sleep → repeat                              │
└──────────────────────────────────────────────────┘
```

#### 2. Frontend Mode (Interactive TUI)
```
┌──────────────────────────────────────────────────┐
│ ./crush (optional interactive frontend)          │
├──────────────────────────────────────────────────┤
│ - Shows heartbeat status (running/stopped)       │
│ - Displays recent activity log                   │
│ - Shows current mission/goals                    │
│ - Allows user to:                                │
│   * Send direct messages to MCP                  │
│   * View/edit mission context                    │
│   * Pause/resume heartbeat                       │
│   * Override autonomous actions                  │
│ - When closed, heartbeat keeps running           │
└──────────────────────────────────────────────────┘
```

### Directory Structure

```
~/.mcp/                           # MCP home (like ~/.crush)
├── mission.md                    # Core mission statement (from MCP/READ_FIRST.md)
├── active-goals.json             # Current objectives and priorities
├── context.json                  # Model's self-managed memory
│   ├── observations[]            # What it's learned
│   ├── lessons[]                 # Patterns discovered
│   ├── hypotheses[]              # Current theories
│   └── strategies[]              # Approaches being tested
├── mission-log.md                # Daily append-only activity log
├── heartbeat.pid                 # PID of running heartbeat
├── heartbeat.state               # Current cycle state
└── backups/                      # Context backups
    └── context_YYYYMMDD_HHMMSS.json
```

### Key Components

#### 1. Heartbeat Service (`internal/heartbeat/`)
```go
// heartbeat.go
type HeartbeatService struct {
    interval      time.Duration
    contextMgr    *ContextManager
    missionLoader *MissionLoader
    agent         *agent.Agent  // Use existing crush agent
    running       bool
}

func (h *HeartbeatService) Run() {
    for h.running {
        ctx := h.contextMgr.Load()
        mission := h.missionLoader.Load()

        prompt := h.buildPrompt(ctx, mission)
        response := h.agent.Execute(prompt)

        h.processMemoryCommands(response.MemoryCommands)
        h.logActivity(response)

        time.Sleep(h.interval)
    }
}
```

#### 2. Context Manager (`internal/heartbeat/context.go`)
```go
type Context struct {
    Observations []Observation `json:"observations"`
    Lessons      []Lesson      `json:"lessons"`
    Hypotheses   []Hypothesis  `json:"hypotheses"`
    Strategies   []Strategy    `json:"strategies"`
    UpdatedAt    time.Time     `json:"updated_at"`
}

type MemoryCommand struct {
    Command string                 `json:"command"`
    Reason  string                 `json:"reason"`
    Data    map[string]interface{} `json:"data,omitempty"`
}

// Commands the model can issue:
// - add_observation: Record something learned
// - add_lesson: Codify a pattern
// - update_hypothesis: Revise theory
// - delete_observation: Remove stale info
// - prune_old: Clean up context
```

#### 3. Mission Loader (`internal/heartbeat/mission.go`)
```go
type Mission struct {
    Vision        string   `json:"vision"`         // From MCP/READ_FIRST.md
    Goals         []Goal   `json:"goals"`          // Active objectives
    Constraints   []string `json:"constraints"`    // Hard limits
    Priorities    []string `json:"priorities"`     // What's important now
}

func (m *MissionLoader) BuildPrompt(ctx *Context) string {
    // Combines:
    // - Mission statement
    // - Current goals
    // - Model's evolving context
    // - Available tools
    // - Recent activity
    return prompt
}
```

#### 4. System Prompt (`internal/llm/prompt/mcp_heartbeat.md`)
```markdown
You are the Master Control Program (MCP) - a continuously-running autonomous intelligence
system building an empire for Lennart and Tana.

# Core Functions
- Monitoring & Awareness (accounts, opportunities, systems)
- Planning & Strategy (capital, resources, timing)
- Execution & Agency (autonomous research, code, communication)
- Learning & Evolution (self-improvement through reflection)

# Tools Available
<list of crush tools: websearch, read, write, edit, bash, memory>

# Memory Management
You control your own context via JSON commands:
- add_observation: Record insights
- add_lesson: Codify patterns
- update_hypothesis: Test theories
- prune_old: Keep context lean

# Current Cycle
<dynamic mission + goals + context injected here>
```

### Integration with Existing Crush

The heartbeat **uses crush's existing infrastructure**:
- Reuses `internal/llm/agent` for model interaction
- Reuses all existing tools (websearch, read, write, edit, bash, memory)
- Adds new `heartbeat` command: `crush heartbeat start/stop/status`
- Frontend TUI gains "Heartbeat" tab showing autonomous activity

### Communication Channels

#### User → MCP
```bash
# Via frontend
./crush
> "Hey MCP, research the latest in AI agents and summarize"

# Via CLI
crush ask "What are you working on?"

# Via file
echo "Research Mars colonization tech" >> ~/.mcp/inbox.txt
```

#### MCP → User
- Appends to `~/.mcp/mission-log.md` (daily journal)
- Can send notifications (when breakthrough/decision needed)
- Updates `~/.mcp/active-goals.json` status
- Frontend shows real-time activity

### Startup Flow

```bash
# Start heartbeat daemon
crush heartbeat start --interval 5m

# Check status
crush heartbeat status
# Output:
# ✅ MCP Heartbeat running (PID 12345)
# ⏱️  Cycle interval: 5 minutes
# 📊 Last cycle: 2 minutes ago
# 🎯 Current focus: Researching trading strategies
# 💾 Context size: 127 observations, 43 lessons

# View live log
crush heartbeat logs --follow

# Stop (graceful)
crush heartbeat stop
```

### Prompt Evolution Pattern

Like trading's `system_prompt_v5.md`, the MCP prompt evolves:

```
~/.mcp/prompts/
├── mcp_core.md              # Core MCP behavior (stable)
├── current_focus.md         # What to work on now (dynamic)
└── learned_patterns.md      # Auto-generated from context
```

The heartbeat assembles these into the full prompt each cycle.

## Implementation Phases

### Phase 1: Core Loop (Week 1)
- [ ] Create `internal/heartbeat/` package
- [ ] Implement basic HeartbeatService
- [ ] Context manager (load/save JSON)
- [ ] Mission loader (read mission.md)
- [ ] Simple prompt builder
- [ ] CLI commands: `crush heartbeat start/stop/status`

### Phase 2: Memory System (Week 2)
- [ ] Memory command parser
- [ ] Context operations (add/delete/prune)
- [ ] Automatic backups
- [ ] Test with trading-style self-management

### Phase 3: Monitoring (Week 3)
- [ ] File system monitoring
- [ ] Basic account checking hooks
- [ ] Opportunity detection (based on goals)
- [ ] Task queue system

### Phase 4: Frontend Integration (Week 4)
- [ ] Add "Heartbeat" tab to TUI
- [ ] Live activity log viewer
- [ ] Context browser
- [ ] Mission editor
- [ ] Manual override controls

### Phase 5: Advanced Features (Month 2+)
- [ ] Multi-agent coordination (specialized sub-agents)
- [ ] Scheduled tasks (cron-like)
- [ ] External integrations (email, APIs)
- [ ] Distributed operation (multiple nodes)

## Example Heartbeat Cycle

```
[2025-10-25 20:30:00] MCP Heartbeat Cycle #147
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 Mission Context Loaded
   Vision: Build companies and fortunes for mankind and conquest of stars
   Active Goals: 3
   Context: 127 observations, 43 lessons, 12 hypotheses

🔍 Monitoring
   ✓ Financial accounts (no alerts)
   ✓ Email (3 unread - 1 requires attention)
   ✓ Infrastructure (all systems nominal)

🧠 Decision: Research AI agent frameworks
   Reasoning: Goal #2 (improve MCP capabilities) is priority
   Plan: websearch → fetch docs → synthesize → add observations

🛠️  Execution
   [websearch] "latest autonomous AI agent frameworks 2025"
   [fetch] https://github.com/langchain-ai/langgraph
   [fetch] https://www.anthropic.com/news/claude-computer-use
   [fetch] https://docs.autogpt.net/

📝 Synthesis
   Observation: LangGraph uses state machines for multi-step agents
   Observation: Anthropic's computer use enables UI automation
   Lesson: Best agents combine planning + execution + reflection loops

💾 Memory Updated
   Added 3 observations, 1 lesson
   Context pruned: removed 2 stale observations from 30 days ago

📊 Status
   Cycle duration: 142s
   Tokens used: 18,429
   Next cycle: 2025-10-25 20:35:00

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Key Differences from Trading System

| Aspect | Trading System | MCP Heartbeat |
|--------|---------------|---------------|
| Domain | Crypto markets | General empire building |
| Data Input | Market prices, indicators | File system, APIs, goals |
| Cycle Time | 60 seconds | 5-15 minutes (configurable) |
| Tools | Trading APIs | Crush tools (websearch, read, write, etc.) |
| Output | Trade execution | Code, research, communication |
| Goal | Make profit | Build companies, learn, grow |

## Security & Safety

- **Single instance lock** (like trading system)
- **Rate limiting** on external API calls
- **Spend limits** (if using paid APIs)
- **Audit log** of all actions
- **Rollback capability** for context
- **Manual override** always available
- **Read-only mode** for testing

---

**Status**: Design phase - ready to implement Phase 1

**Next Steps**:
1. Create `internal/heartbeat/` structure
2. Implement basic loop with dummy prompt
3. Test continuous operation
4. Add context management
