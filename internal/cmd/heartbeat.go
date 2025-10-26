package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/crush/internal/heartbeat"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(heartbeatCmd)
	heartbeatCmd.AddCommand(heartbeatStartCmd)
	heartbeatCmd.AddCommand(heartbeatStopCmd)
	heartbeatCmd.AddCommand(heartbeatStatusCmd)

	heartbeatStartCmd.Flags().DurationP("interval", "i", 5*time.Minute, "Cycle interval (e.g., 5m, 10m, 1h)")
	heartbeatStartCmd.Flags().BoolP("background", "b", false, "Run in background (daemon mode)")
}

var heartbeatCmd = &cobra.Command{
	Use:   "heartbeat",
	Short: "Manage the MCP autonomous heartbeat service",
	Long: `The heartbeat service runs the Master Control Program (MCP) in autonomous mode.
It continuously monitors, plans, executes, and learns - operating 24/7 to work toward your goals.

The heartbeat service:
- Runs in the background as a daemon
- Uses the same agent and tools as interactive mode
- Manages its own memory and context through self-reflection
- Logs all activity to ~/.mcp/mission-log.md
- Can be monitored and interacted with via the crush TUI`,
	Example: `
# Start the heartbeat with 5-minute cycles
crush heartbeat start

# Start with 10-minute cycles
crush heartbeat start -i 10m

# Check status
crush heartbeat status

# Stop the heartbeat
crush heartbeat stop`,
}

var heartbeatStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the MCP heartbeat service",
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		background, _ := cmd.Flags().GetBool("background")

		// Check if already running
		running, pid, err := heartbeat.GetStatus()
		if err != nil {
			return fmt.Errorf("failed to check status: %w", err)
		}

		if running {
			return fmt.Errorf("heartbeat already running with PID %d\nUse 'crush heartbeat stop' to stop it first", pid)
		}

		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("🤖  Starting MCP Heartbeat Service")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("⏱️   Cycle Interval: %s\n", interval)
		fmt.Printf("📂  State Directory: ~/.mcp/\n")
		fmt.Printf("📝  Mission Log: ~/.mcp/mission-log.md\n")
		fmt.Println()

		if background {
			fmt.Println("⚠️  Background mode not yet implemented")
			fmt.Println("   For now, run in foreground (Ctrl+C to stop)")
			fmt.Println()
		}

		// Initialize crush app (same infrastructure as TUI)
		app, err := setupApp(cmd)
		if err != nil {
			return fmt.Errorf("failed to setup app: %w", err)
		}
		defer app.Shutdown()

		// Create adapter
		adapter := heartbeat.NewAppAdapter(app)

		// Create and start the service with the adapter
		service := heartbeat.NewService(interval, adapter)

		fmt.Println("✅ Heartbeat service starting...")
		fmt.Println("   Press Ctrl+C to stop")
		fmt.Println()

		if err := service.Start(); err != nil {
			return fmt.Errorf("heartbeat service failed: %w", err)
		}

		return nil
	},
}

var heartbeatStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the MCP heartbeat service",
	RunE: func(cmd *cobra.Command, args []string) error {
		running, pid, err := heartbeat.GetStatus()
		if err != nil {
			return fmt.Errorf("failed to check status: %w", err)
		}

		if !running {
			fmt.Println("❌ Heartbeat is not running")
			return nil
		}

		fmt.Printf("🛑 Stopping heartbeat service (PID %d)...\n", pid)

		// Send SIGTERM to the process
		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("failed to find process: %w", err)
		}

		if err := process.Signal(os.Interrupt); err != nil {
			return fmt.Errorf("failed to stop process: %w", err)
		}

		// Wait a moment and check if it stopped
		time.Sleep(500 * time.Millisecond)

		running, _, _ = heartbeat.GetStatus()
		if !running {
			fmt.Println("✅ Heartbeat service stopped")
		} else {
			fmt.Println("⚠️  Heartbeat may still be shutting down...")
		}

		return nil
	},
}

var heartbeatStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the MCP heartbeat service",
	RunE: func(cmd *cobra.Command, args []string) error {
		running, pid, err := heartbeat.GetStatus()
		if err != nil {
			return fmt.Errorf("failed to check status: %w", err)
		}

		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("🤖  MCP Heartbeat Status")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		if running {
			fmt.Printf("Status: ✅ RUNNING (PID %d)\n", pid)
			fmt.Println()

			// Try to load context to show stats
			mgr := heartbeat.NewContextManager()
			ctx, err := mgr.LoadContext()
			if err == nil {
				fmt.Println("📊 Context Statistics:")
				fmt.Printf("   Total Cycles: %d\n", ctx.Metadata.TotalCycles)
				fmt.Printf("   Observations: %d\n", len(ctx.Observations))
				fmt.Printf("   Lessons: %d\n", len(ctx.Lessons))
				fmt.Printf("   Hypotheses: %d\n", len(ctx.Hypotheses))
				fmt.Printf("   Strategies: %d\n", len(ctx.Strategies))
				fmt.Printf("   Last Updated: %s\n", ctx.Metadata.UpdatedAt.Format("2006-01-02 15:04:05 MST"))
			}

			goals, err := mgr.LoadGoals()
			if err == nil {
				activeGoals := 0
				for _, g := range goals.Goals {
					if g.Status == "active" {
						activeGoals++
					}
				}
				fmt.Println()
				fmt.Println("🎯 Active Goals:")
				fmt.Printf("   %d of %d goals active\n", activeGoals, len(goals.Goals))
			}

		} else {
			fmt.Println("Status: ❌ NOT RUNNING")
			fmt.Println()
			fmt.Println("Start the heartbeat with:")
			fmt.Println("   crush heartbeat start")
		}

		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		return nil
	},
}
