package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
)

// CompactSession performs intelligent conversation compaction
// It removes stale content while preserving critical context
func (a *agent) CompactSession(ctx context.Context, sessionID string) error {
	if a.summarizeProvider == nil {
		return fmt.Errorf("summarize provider not available")
	}

	// Check if session is busy
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	slog.Info("Starting automatic compaction", "session_id", sessionID)

	// Create a new context with cancellation
	compactCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Store the cancel function
	a.activeRequests.Set(sessionID+"-compact", cancel)
	defer a.activeRequests.Del(sessionID + "-compact")

	// Publish start event
	event := AgentEvent{
		Type:     AgentEventTypeSummarize,
		Progress: "Auto-compacting conversation at 85% context...",
	}
	a.Publish(pubsub.CreatedEvent, event)

	// Get all messages from the session
	msgs, err := a.messages.List(compactCtx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	if len(msgs) == 0 {
		return fmt.Errorf("no messages to compact")
	}

	// Categorize messages: keep recent, summarize old
	keepCount := 10 // Keep last 10 message pairs (20 messages)
	if len(msgs) <= keepCount {
		return fmt.Errorf("not enough messages to compact (need more than %d)", keepCount)
	}

	// Split: messages to summarize vs messages to keep
	splitIndex := len(msgs) - keepCount
	toSummarize := msgs[:splitIndex]
	toKeep := msgs[splitIndex:]

	event = AgentEvent{
		Type:     AgentEventTypeSummarize,
		Progress: fmt.Sprintf("Analyzing %d messages for compaction...", len(toSummarize)),
	}
	a.Publish(pubsub.CreatedEvent, event)

	// Filter out stale tool results from messages to summarize
	filteredForSummary := a.filterStaleToolResults(toSummarize)

	// Create summary of old messages
	event = AgentEvent{
		Type:     AgentEventTypeSummarize,
		Progress: "Generating compact summary...",
	}
	a.Publish(pubsub.CreatedEvent, event)

	compactCtx = context.WithValue(compactCtx, tools.SessionIDContextKey, sessionID)

	summarizePrompt := `Provide a concise summary of our conversation above. Focus on:
- Key decisions and architectural choices
- Important code changes and their locations
- Active goals and next steps
- Critical errors or blockers encountered
- Any patterns or learnings discovered

Keep the summary compact but preserve ALL critical information needed to continue the conversation seamlessly.`

	promptMsg := message.Message{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: summarizePrompt}},
	}

	msgsWithPrompt := append(filteredForSummary, promptMsg)

	// Generate summary
	response := a.summarizeProvider.StreamResponse(
		compactCtx,
		msgsWithPrompt,
		nil,
	)

	var finalResponse *provider.ProviderResponse
	for r := range response {
		if r.Error != nil {
			return fmt.Errorf("failed to generate summary: %w", r.Error)
		}
		finalResponse = r.Response
	}

	summary := strings.TrimSpace(finalResponse.Content)
	if summary == "" {
		return fmt.Errorf("empty summary returned")
	}

	// Add current shell working directory
	shellCwd := shell.GetPersistentShell(config.Get().WorkingDir()).GetWorkingDir()
	summary += "\n\n**Current working directory of persistent shell:** " + shellCwd

	event = AgentEvent{
		Type:     AgentEventTypeSummarize,
		Progress: "Compacting conversation history...",
	}
	a.Publish(pubsub.CreatedEvent, event)

	// Create the summary message
	summaryMsg, err := a.messages.Create(compactCtx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "# Conversation Summary\n\n" + summary},
			message.Finish{
				Reason: message.FinishReasonEndTurn,
				Time:   time.Now().Unix(),
			},
		},
		Model:    a.summarizeProvider.Model().ID,
		Provider: a.summarizeProviderID,
	})
	if err != nil {
		return fmt.Errorf("failed to create summary message: %w", err)
	}

	// Delete old messages (but keep the summary)
	deletedCount := 0
	for _, msg := range toSummarize {
		if err := a.messages.Delete(compactCtx, msg.ID); err != nil {
			slog.Warn("Failed to delete message during compaction", "msg_id", msg.ID, "error", err)
		} else {
			deletedCount++
		}
	}

	// Update session with summary reference and reset token count
	sess, err := a.sessions.Get(compactCtx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	sess.SummaryMessageID = summaryMsg.ID
	// Reset tokens - they'll be recalculated on next interaction
	sess.CompletionTokens = finalResponse.Usage.OutputTokens
	sess.PromptTokens = 0

	// Add summary generation cost
	model := a.summarizeProvider.Model()
	usage := finalResponse.Usage
	cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		model.CostPer1MIn/1e6*float64(usage.InputTokens) +
		model.CostPer1MOut/1e6*float64(usage.OutputTokens)
	sess.Cost += cost

	_, err = a.sessions.Save(compactCtx, sess)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	slog.Info("Compaction complete",
		"session_id", sessionID,
		"deleted_messages", deletedCount,
		"kept_messages", len(toKeep)+1, // +1 for summary
		"summary_tokens", finalResponse.Usage.OutputTokens,
	)

	event = AgentEvent{
		Type:      AgentEventTypeSummarize,
		SessionID: sessionID,
		Progress:  fmt.Sprintf("✓ Compacted: %d messages → summary + %d recent messages", deletedCount, len(toKeep)),
		Done:      true,
	}
	a.Publish(pubsub.CreatedEvent, event)

	return nil
}

// filterStaleToolResults removes verbose tool outputs from old messages
// while preserving the tool calls themselves for context
func (a *agent) filterStaleToolResults(msgs []message.Message) []message.Message {
	filtered := make([]message.Message, 0, len(msgs))

	for _, msg := range msgs {
		// Keep user messages as-is
		if msg.Role == message.User {
			filtered = append(filtered, msg)
			continue
		}

		// For assistant messages, filter tool results
		if msg.Role == message.Assistant {
			newParts := make([]message.ContentPart, 0, len(msg.Parts))

			for _, part := range msg.Parts {
				switch p := part.(type) {
				case message.ToolResult:
					// Keep tool result but truncate if too verbose
					if len(p.Content) > 1000 {
						p.Content = p.Content[:1000] + "\n...[truncated for compaction]"
					}
					newParts = append(newParts, p)

				case message.ToolCall:
					// Always keep tool calls (they're small and important for context)
					newParts = append(newParts, p)

				case message.TextContent:
					// Keep text content
					newParts = append(newParts, p)

				case message.Finish:
					// Keep finish markers
					newParts = append(newParts, p)

				default:
					// Keep other parts
					newParts = append(newParts, p)
				}
			}

			msg.Parts = newParts
			filtered = append(filtered, msg)
		}
	}

	return filtered
}
