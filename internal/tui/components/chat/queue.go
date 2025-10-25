package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

func queuePill(queue int, queuedPrompts []string, t *styles.Theme) string {
	if queue <= 0 {
		return ""
	}

	// Build the content with just the queued prompts
	var content strings.Builder
	for i, prompt := range queuedPrompts {
		// Truncate long prompts
		displayPrompt := prompt
		if len(displayPrompt) > 60 {
			displayPrompt = displayPrompt[:57] + "..."
		}
		content.WriteString(fmt.Sprintf("%d. %s", i+1, displayPrompt))
		if i < len(queuedPrompts)-1 {
			content.WriteString("\n")
		}
	}

	return t.S().Base.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.BgOverlay).
		PaddingLeft(1).
		PaddingRight(1).
		Render(content.String())
}
