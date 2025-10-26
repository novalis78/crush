package heartbeat

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Load system prompt from file instead of embed for now
func loadSystemPrompt() string {
	content, err := os.ReadFile("internal/llm/prompt/mcp_heartbeat.md")
	if err != nil {
		// Fallback if file not found
		return "You are the MCP (Master Control Program) - an autonomous agent working toward goals."
	}
	return string(content)
}

var mcpSystemPrompt = loadSystemPrompt()

// PromptBuilder builds prompts for the MCP heartbeat cycles
type PromptBuilder struct{}

// BuildSystemPrompt returns the MCP system prompt
func (pb *PromptBuilder) BuildSystemPrompt() string {
	return mcpSystemPrompt
}

// BuildUserPrompt builds the dynamic user prompt from current state
func (pb *PromptBuilder) BuildUserPrompt(ctx *Context, goals *Goals, mission string, cycleNumber int) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# MCP Heartbeat Cycle %d\n\n", cycleNumber))
	b.WriteString(fmt.Sprintf("**Time**: %s\n\n", time.Now().Format("2006-01-02 15:04:05 MST")))

	// Mission (abbreviated)
	b.WriteString("## Mission\n\n")
	b.WriteString("Build companies and fortunes for Lennart and Tana. Pursue knowledge, prosperity, and the conquest of space.\n\n")

	// Active Goals
	b.WriteString("## Active Goals\n\n")
	if len(goals.Goals) == 0 {
		b.WriteString("*No active goals*\n\n")
	} else {
		for i, goal := range goals.Goals {
			if goal.Status != "active" {
				continue
			}
			b.WriteString(fmt.Sprintf("%d. **[%s] %s**\n", i+1, goal.Priority, goal.Title))
			b.WriteString(fmt.Sprintf("   %s\n", goal.Description))
			if len(goal.Progress) > 0 {
				latest := goal.Progress[len(goal.Progress)-1]
				b.WriteString(fmt.Sprintf("   *Latest: %s*\n", latest))
			}
			b.WriteString("\n")
		}
	}

	// Context Summary
	b.WriteString("## Your Knowledge\n\n")

	// Recent observations (last 5)
	if len(ctx.Observations) > 0 {
		b.WriteString("### Recent Observations\n\n")
		start := len(ctx.Observations) - 5
		if start < 0 {
			start = 0
		}
		for i := start; i < len(ctx.Observations); i++ {
			obs := ctx.Observations[i]
			b.WriteString(fmt.Sprintf("- [Cycle %d] %s\n", obs.Cycle, obs.Content))
		}
		b.WriteString("\n")
	}

	// All lessons
	if len(ctx.Lessons) > 0 {
		b.WriteString("### Lessons Learned\n\n")
		for _, lesson := range ctx.Lessons {
			confidence := ""
			if lesson.Confidence != "" {
				confidence = fmt.Sprintf(" [%s confidence]", lesson.Confidence)
			}
			b.WriteString(fmt.Sprintf("- %s%s\n", lesson.Content, confidence))
		}
		b.WriteString("\n")
	}

	// Active hypotheses
	if len(ctx.Hypotheses) > 0 {
		b.WriteString("### Hypotheses Being Tested\n\n")
		for _, hyp := range ctx.Hypotheses {
			if hyp.Status == "testing" {
				b.WriteString(fmt.Sprintf("- %s\n", hyp.Content))
			}
		}
		b.WriteString("\n")
	}

	// Strategies
	if len(ctx.Strategies) > 0 {
		b.WriteString("### Strategies\n\n")
		for _, strat := range ctx.Strategies {
			effectiveness := ""
			if strat.Effectiveness != "" {
				effectiveness = fmt.Sprintf(" - %s", strat.Effectiveness)
			}
			b.WriteString(fmt.Sprintf("- **%s**: %s%s\n", strat.Name, strat.Description, effectiveness))
		}
		b.WriteString("\n")
	}

	// This Cycle
	b.WriteString("## This Cycle\n\n")
	b.WriteString("What will you work on this cycle? Choose a goal, use tools to make progress, and update your memory.\n\n")
	b.WriteString("Remember:\n")
	b.WriteString("- Use websearch + fetch to research autonomously\n")
	b.WriteString("- Check your observations before re-researching\n")
	b.WriteString("- Update your memory with what you learn\n")
	b.WriteString("- Work incrementally toward goals\n\n")

	b.WriteString("Begin your autonomous work now.\n")

	return b.String()
}
