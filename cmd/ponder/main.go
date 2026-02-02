package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ldi/ponder/internal/db"
	"github.com/ldi/ponder/internal/mcp"
	"github.com/ldi/ponder/internal/orchestrator"
	"github.com/ldi/ponder/internal/server"
	"github.com/ldi/ponder/internal/ui"
	"github.com/ldi/ponder/pkg/models"
)

var (
	dbPath       string
	snapshotPath string
	verbose      bool
)

func main() {
	flag.StringVar(&dbPath, "db-path", ".ponder/ponder.db", "Path to database file")
	flag.StringVar(&snapshotPath, "snapshot-path", ".ponder/snapshot.jsonl", "Path to snapshot file")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	var command string
	var args []string

	if flag.NArg() == 0 {
		selected, err := ui.RunMenu()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running menu: %v\n", err)
			os.Exit(1)
		}
		if selected == "" {
			os.Exit(0)
		}
		command = selected
		args = []string{}
	} else {
		command = flag.Arg(0)
		args = flag.Args()[1:]
	}

	var err error
	switch command {
	case "init":
		err = runInit(args)
	case "mcp":
		err = runMCP(args)
	case "list-features":
		err = runListFeatures(args)
	case "list-tasks":
		err = runListTasks(args)
	case "status":
		err = runStatus(args)
	case "web":
		err = runWeb(args)
	case "work":
		err = runWork(args)
	case "orchestrate":
		err = runOrchestrate(args)
	case "db":
		err = runDB(args)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runInit(args []string) error {
	targetDir := "."
	if len(args) > 0 {
		targetDir = args[0]
	}

	ponderDir := filepath.Join(targetDir, ".ponder")
	if err := os.MkdirAll(ponderDir, 0755); err != nil {
		return fmt.Errorf("failed to create .ponder directory: %w", err)
	}
	fmt.Println("✓ Created .ponder/ directory")

	gitignorePath := filepath.Join(ponderDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("ponder.db*\n"), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}
	fmt.Println("✓ Created .ponder/.gitignore")

	// Default paths if not overridden by flags
	finalDbPath := dbPath
	if dbPath == ".ponder/ponder.db" {
		finalDbPath = filepath.Join(ponderDir, "ponder.db")
	}

	finalSnapshotPath := snapshotPath
	if snapshotPath == ".ponder/snapshot.jsonl" {
		finalSnapshotPath = filepath.Join(ponderDir, "snapshot.jsonl")
	}

	database, err := db.Open(finalDbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	fmt.Printf("✓ Initialized database at %s\n", finalDbPath)

	// Check if snapshot exists and import it
	if _, err := os.Stat(finalSnapshotPath); err == nil {
		if err := database.ImportSnapshot(ctx, finalSnapshotPath); err != nil {
			return fmt.Errorf("failed to import snapshot: %w", err)
		}
		fmt.Printf("✓ Imported snapshot from %s\n", finalSnapshotPath)
	} else {
		// If no snapshot, seed default "misc" feature
		feature := &models.Feature{
			Name:        "misc",
			Description: "Miscellaneous tasks",
		}
		// Check if it already exists
		existing, err := database.GetFeatureByName(ctx, "misc")
		if err != nil {
			return fmt.Errorf("failed to check for existing misc feature: %w", err)
		}
		if existing == nil {
			if err := database.CreateFeature(ctx, feature); err != nil {
				return fmt.Errorf("failed to seed misc feature: %w", err)
			}
			fmt.Println("✓ Seeded default 'misc' feature")
		}
	}

	fmt.Println("✓ Ponder initialized successfully")
	return nil
}

func runMCP(args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.Init(ctx); err != nil {
		return err
	}

	// Set auto-snapshotting
	database.SetOnChange(func(ctx context.Context) {
		if err := database.ExportSnapshot(ctx, snapshotPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting snapshot: %v\n", err)
		}
	})

	s := mcp.NewServer(database)
	return mcp.Serve(s)
}

func runWeb(args []string) error {
	webFlags := flag.NewFlagSet("web", flag.ContinueOnError)
	port := webFlags.String("port", "8000", "Port to listen on")
	if err := webFlags.Parse(args); err != nil {
		return err
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.Init(ctx); err != nil {
		return err
	}

	srv := server.NewServer(database)
	return srv.Start(fmt.Sprintf(":%s", *port))
}

func runDB(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: ponder db <command> [arguments]")
		fmt.Println("\nCommands:")
		fmt.Println("  status    Show database status")
		return nil
	}

	command := args[0]
	subArgs := args[1:]

	switch command {
	case "status":
		return runStatus(subArgs)
	default:
		return fmt.Errorf("unknown db command: %s", command)
	}
}

func runListFeatures(args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	features, err := database.ListFeatures(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("%-20s %-30s\n", "NAME", "DESCRIPTION")
	fmt.Println("------------------------------------------------------------")
	for _, f := range features {
		fmt.Printf("%-20s %-30s\n", f.Name, f.Description)
	}
	return nil
}

func runListTasks(args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	// Parse flags for filtering
	taskFlags := flag.NewFlagSet("list-tasks", flag.ContinueOnError)
	statusFilter := taskFlags.String("status", "", "Filter by status (pending, in_progress, completed, blocked)")
	featureFilter := taskFlags.String("feature", "", "Filter by feature name")
	if err := taskFlags.Parse(args); err != nil {
		return err
	}

	var status *models.TaskStatus
	if *statusFilter != "" {
		s := models.TaskStatus(*statusFilter)
		status = &s
	}

	var featureName *string
	if *featureFilter != "" {
		featureName = featureFilter
	}

	ctx := context.Background()
	tasks, err := database.ListTasks(ctx, status, featureName)
	if err != nil {
		return err
	}

	fmt.Printf("%-30s %-15s %-10s %-15s\n", "NAME", "FEATURE", "PRIORITY", "STATUS")
	fmt.Println("----------------------------------------------------------------------")
	for _, t := range tasks {
		fmt.Printf("%-30s %-15s %-10d %-15s\n", t.Name, t.FeatureName, t.Priority, t.Status)
	}
	return nil
}

func runStatus(args []string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	features, err := database.ListFeatures(ctx)
	if err != nil {
		return err
	}

	tasks, err := database.ListTasks(ctx, nil, nil)
	if err != nil {
		return err
	}

	available, err := database.GetAvailableTasks(ctx)
	if err != nil {
		return err
	}

	fmt.Println("Ponder Project Status")
	fmt.Println("=====================")
	fmt.Printf("Features:        %d\n", len(features))
	fmt.Printf("Total Tasks:     %d\n", len(tasks))
	fmt.Printf("Available Tasks: %d\n", len(available))

	// Count by status
	statusCounts := make(map[models.TaskStatus]int)
	for _, t := range tasks {
		statusCounts[t.Status]++
	}

	fmt.Println("\nTask Breakdown:")
	fmt.Printf("  Pending:     %d\n", statusCounts[models.TaskStatusPending])
	fmt.Printf("  In Progress: %d\n", statusCounts[models.TaskStatusInProgress])
	fmt.Printf("  Completed:   %d\n", statusCounts[models.TaskStatusCompleted])
	fmt.Printf("  Blocked:     %d\n", statusCounts[models.TaskStatusBlocked])

	if len(available) > 0 {
		fmt.Println("\nNext Available Tasks:")
		for i, t := range available {
			if i >= 5 {
				break
			}
			fmt.Printf("  - %s (priority: %d)\n", t.Name, t.Priority)
		}
	}

	return nil
}

func runOrchestrate(args []string) error {
	orchFlags := flag.NewFlagSet("orchestrate", flag.ContinueOnError)
	maxWorkers := orchFlags.Int("workers", 3, "Maximum number of concurrent workers")
	model := orchFlags.String("model", "opencode/gemini-3-flash", "Model to use for workers")
	interval := orchFlags.Duration("interval", 5*time.Second, "Polling interval when idle (0 to exit)")
	enableWeb := orchFlags.Bool("web", true, "Enable web UI")
	webPort := orchFlags.String("port", "8000", "Port for web UI")
	if err := orchFlags.Parse(args); err != nil {
		return err
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := database.Init(ctx); err != nil {
		return err
	}

	// Set auto-snapshotting for the orchestrator too
	database.SetOnChange(func(ctx context.Context) {
		if err := database.ExportSnapshot(ctx, snapshotPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting snapshot: %v\n", err)
		}
	})

	orch := orchestrator.NewOrchestrator(database, *maxWorkers, *model)
	orch.PollingInterval = *interval

	// Start web server if enabled
	var webURL string
	if *enableWeb {
		srv := server.NewServer(database)
		webURL = fmt.Sprintf("http://localhost:%s", *webPort)
		orch.WebURL = webURL

		go func() {
			if err := srv.Start(fmt.Sprintf(":%s", *webPort)); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
			}
		}()

		// Ensure graceful shutdown
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(shutdownCtx)
		}()
	}

	return orchestrator.Run(ctx, orch)
}

func runWork(args []string) error {
	workFlags := flag.NewFlagSet("work", flag.ContinueOnError)
	concurrency := workFlags.Int("concurrency", 3, "Maximum number of concurrent workers")
	model := workFlags.String("model", "opencode/gemini-3-flash", "Model to use for workers")
	interval := workFlags.Duration("interval", 5*time.Second, "Polling interval when idle (0 to exit)")
	enableWeb := workFlags.Bool("web", true, "Enable web UI")
	webPort := workFlags.String("port", "8000", "Port for web UI")
	if err := workFlags.Parse(args); err != nil {
		return err
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := database.Init(ctx); err != nil {
		return err
	}

	// Set auto-snapshotting for the orchestrator too
	database.SetOnChange(func(ctx context.Context) {
		if err := database.ExportSnapshot(ctx, snapshotPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting snapshot: %v\n", err)
		}
	})

	orch := orchestrator.NewOrchestrator(database, *concurrency, *model)
	orch.PollingInterval = *interval

	// Start web server if enabled
	var webURL string
	if *enableWeb {
		srv := server.NewServer(database)
		webURL = fmt.Sprintf("http://localhost:%s", *webPort)
		orch.WebURL = webURL

		go func() {
			if err := srv.Start(fmt.Sprintf(":%s", *webPort)); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
			}
		}()

		// Ensure graceful shutdown
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(shutdownCtx)
		}()
	}

	return orchestrator.Run(ctx, orch)
}
