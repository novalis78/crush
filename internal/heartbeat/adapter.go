package heartbeat

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/llm/agent"
)

// AppAdapter adapts app.App to implement AppInterface
type AppAdapter struct {
	app          *app.App
	lastResponse string
	logger       *slog.Logger
}

// NewAppAdapter creates a new adapter
func NewAppAdapter(a *app.App) *AppAdapter {
	return &AppAdapter{
		app:    a,
		logger: slog.Default(),
	}
}

// CoderAgentRun runs the agent and waits for completion
func (a *AppAdapter) CoderAgentRun(ctx context.Context, sessionID, prompt string) (<-chan struct{}, error) {
	// Run the agent
	events, err := a.app.CoderAgent.Run(ctx, sessionID, prompt)
	if err != nil {
		return nil, err
	}

	// Create done channel
	done := make(chan struct{})

	// Process events in background
	go func() {
		defer close(done)
		for event := range events {
			// Capture responses
			if event.Type == agent.AgentEventTypeResponse {
				a.lastResponse += event.Message.Content().Text
			}
			// Log completion
			if event.Done {
				a.logger.Debug("Agent completed")
			}
			// Log errors
			if event.Error != nil {
				a.logger.Error("Agent error", "error", event.Error)
			}
		}
	}()

	return done, nil
}

// CoderAgentGetLastResponse gets the last response
func (a *AppAdapter) CoderAgentGetLastResponse() string {
	response := a.lastResponse
	a.lastResponse = "" // Clear for next run
	return response
}

// SessionsCreate creates a new session
func (a *AppAdapter) SessionsCreate(ctx context.Context, title string) (string, error) {
	sess, err := a.app.Sessions.Create(ctx, title)
	if err != nil {
		return "", err
	}
	return sess.ID, nil
}

// PermissionsAutoApprove auto-approves permissions for a session
func (a *AppAdapter) PermissionsAutoApprove(sessionID string) {
	a.app.Permissions.AutoApproveSession(sessionID)
}
