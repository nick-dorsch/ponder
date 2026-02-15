package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nick-dorsch/ponder/internal/db"
	"github.com/nick-dorsch/ponder/internal/mcp"
	"github.com/nick-dorsch/ponder/internal/orchestrator"
	"github.com/nick-dorsch/ponder/internal/server"
	"github.com/nick-dorsch/ponder/pkg/models"
)

var (
	dbPath       string
	snapshotPath string
	verbose      bool
)

const (
	defaultWorkMaxConcurrency = 4
	defaultWorkModel          = "opencode/gemini-3-flash"
)

type workConfig struct {
	Model           *string  `json:"model"`
	MaxConcurrency  *int     `json:"max_concurrency"`
	AvailableModels []string `json:"available_models"`
}

type workDefaults struct {
	Model           string
	MaxConcurrency  int
	AvailableModels []string
}

var runOrchestrator = runOrchestratorCommon

func main() {
	err := execute(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func execute(args []string, stderr io.Writer) error {
	rootFlags := flag.NewFlagSet("ponder", flag.ContinueOnError)
	rootFlags.SetOutput(stderr)
	rootFlags.StringVar(&dbPath, "db-path", ".ponder/ponder.db", "Path to database file")
	rootFlags.StringVar(&snapshotPath, "snapshot-path", ".ponder/snapshot.jsonl", "Path to snapshot file")
	rootFlags.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	maxConcurrency := rootFlags.Int("max_concurrency", defaultWorkMaxConcurrency, "Maximum number of concurrent workers")
	model := rootFlags.String("model", defaultWorkModel, "Model to use for workers")
	interval := rootFlags.Duration("interval", 5*time.Second, "Polling interval when idle (0 to exit)")
	enableWeb := rootFlags.Bool("web", true, "Enable web UI")
	webPort := rootFlags.String("port", "8000", "Port for web UI")
	rootFlags.Usage = func() {
		printRootUsage(stderr, rootFlags)
	}

	if err := rootFlags.Parse(args); err != nil {
		return err
	}

	defaults, err := loadWorkDefaults()
	if err != nil {
		return err
	}

	if !flagProvided(rootFlags, "max_concurrency") {
		*maxConcurrency = defaults.MaxConcurrency
	}
	if !flagProvided(rootFlags, "model") {
		*model = defaults.Model
	}

	if rootFlags.NArg() == 0 {
		return runOrchestrator(*maxConcurrency, 0, *model, defaults.AvailableModels, *interval, *enableWeb, *webPort)
	}

	command := rootFlags.Arg(0)
	commandArgs := rootFlags.Args()[1:]

	switch command {
	case "init":
		return runInit(commandArgs)
	case "mcp":
		return runMCP(commandArgs)
	case "list-features":
		return runListFeatures(commandArgs)
	case "list-tasks":
		return runListTasks(commandArgs)
	case "status":
		return runStatus(commandArgs)
	case "web":
		return runWeb(commandArgs)
	case "db":
		return runDB(commandArgs)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func flagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}

func printRootUsage(w io.Writer, rootFlags *flag.FlagSet) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ponder [flags]")
	fmt.Fprintln(w, "  ponder <command> [args]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Running `ponder` with no command launches the Work TUI.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  init          Initialize Ponder in a directory")
	fmt.Fprintln(w, "  mcp           Start MCP server")
	fmt.Fprintln(w, "  list-features List all features")
	fmt.Fprintln(w, "  list-tasks    List all tasks")
	fmt.Fprintln(w, "  status        Show project status")
	fmt.Fprintln(w, "  web           Start web server")
	fmt.Fprintln(w, "  db            Database commands")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	rootFlags.PrintDefaults()
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

	configPath := filepath.Join(ponderDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := writeDefaultConfig(configPath); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
		fmt.Println("✓ Created .ponder/config.json")
	}

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

	if _, err := os.Stat(finalSnapshotPath); err == nil {
		if err := database.ImportSnapshot(ctx, finalSnapshotPath); err != nil {
			return fmt.Errorf("failed to import snapshot: %w", err)
		}
		fmt.Printf("✓ Imported snapshot from %s\n", finalSnapshotPath)
	} else {
		feature := &models.Feature{
			Name:        "misc",
			Description: "Miscellaneous tasks",
		}
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

func loadWorkDefaults() (workDefaults, error) {
	defaults := workDefaults{
		Model:           defaultWorkModel,
		MaxConcurrency:  defaultWorkMaxConcurrency,
		AvailableModels: []string{defaultWorkModel},
	}

	configPath := filepath.Join(filepath.Dir(dbPath), "config.json")
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return defaults, nil
	}
	if err != nil {
		return defaults, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var cfg workConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaults, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	if cfg.Model != nil && *cfg.Model != "" {
		defaults.Model = *cfg.Model
	}
	if cfg.MaxConcurrency != nil {
		if *cfg.MaxConcurrency < 1 {
			return defaults, fmt.Errorf("invalid max_concurrency in %s: must be >= 1", configPath)
		}
		defaults.MaxConcurrency = *cfg.MaxConcurrency
	}
	if len(cfg.AvailableModels) > 0 {
		defaults.AvailableModels = cfg.AvailableModels
	}

	foundModel := false
	for _, model := range defaults.AvailableModels {
		if model == defaults.Model {
			foundModel = true
			break
		}
	}
	if !foundModel {
		defaults.AvailableModels = append(defaults.AvailableModels, defaults.Model)
	}

	return defaults, nil
}

func writeDefaultConfig(configPath string) error {
	model := defaultWorkModel
	maxConcurrency := defaultWorkMaxConcurrency

	cfg := workConfig{
		Model:           &model,
		MaxConcurrency:  &maxConcurrency,
		AvailableModels: []string{defaultWorkModel},
	}

	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}
	content = append(content, '\n')

	if err := os.WriteFile(configPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}

	return nil
}

func runOrchestratorCommon(maxConcurrency int, initialWorkers int, model string, availableModels []string, interval time.Duration, enableWeb bool, webPort string) error {
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

	database.SetOnChange(func(ctx context.Context) {
		if err := database.ExportSnapshot(ctx, snapshotPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting snapshot: %v\n", err)
		}
	})

	orch := orchestrator.NewOrchestrator(database, maxConcurrency, model)
	orch.SetAvailableModels(availableModels)
	orch.SetTargetWorkers(initialWorkers)
	orch.PollingInterval = interval

	if enableWeb {
		srv := server.NewServer(database)
		orch.WebURL = fmt.Sprintf("http://localhost:%s", webPort)

		go func() {
			if err := srv.Start(fmt.Sprintf(":%s", webPort)); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
			}
		}()

		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(shutdownCtx)
		}()
	}

	return orchestrator.Run(ctx, orch)
}
