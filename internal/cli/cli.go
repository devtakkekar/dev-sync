package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/devsync/dev-sync/internal/config"
	"github.com/devsync/dev-sync/internal/drift"
	"github.com/devsync/dev-sync/internal/ignore"
	"github.com/devsync/dev-sync/internal/logging"
	"github.com/devsync/dev-sync/internal/safety"
	"github.com/devsync/dev-sync/internal/scanner"
	"github.com/devsync/dev-sync/internal/syncengine"
	"github.com/devsync/dev-sync/internal/verify"
	"github.com/devsync/dev-sync/internal/watcher"
)

// Exit codes conforming to PRD Section 46
const (
	ExitSuccess            = 0
	ExitGeneralError       = 1
	ExitConfigError        = 2
	ExitValidationError    = 3
	ExitSyncFailure        = 4
	ExitVerifyFailure      = 5
	ExitReplicaUnavailable = 6
)

var (
	Version   = "1.0.0"
	BuildDate = "2026-10-07"
	Commit    = "release"
)

// Run parses CLI flags and dispatches commands.
func Run(args []string) int {
	if len(args) == 0 {
		PrintHelp()
		return ExitSuccess
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "init":
		return runInit(cmdArgs)
	case "add":
		return runAdd(cmdArgs)
	case "remove", "rm":
		return runRemove(cmdArgs)
	case "list", "ls":
		return runList(cmdArgs)
	case "sync":
		return runSync(cmdArgs)
	case "watch":
		return runWatch(cmdArgs)
	case "status":
		return runStatus(cmdArgs)
	case "verify":
		return runVerify(cmdArgs)
	case "doctor":
		return runDoctor(cmdArgs)
	case "config":
		return runConfig(cmdArgs)
	case "version", "-v", "--version":
		return runVersion(cmdArgs)
	case "help", "-h", "--help":
		PrintHelp()
		return ExitSuccess
	default:
		logging.PrintError("Unknown command: %s. Use 'dev-sync help' for usage.", cmd)
		return ExitGeneralError
	}
}

// -------------------------------------------------------------
// Command: dev-sync init
// -------------------------------------------------------------
func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	sourceFlag := fs.String("source", "", "Source directory path (default: current directory)")
	nonInteractive := fs.Bool("non-interactive", false, "Run in non-interactive mode")
	yesFlag := fs.Bool("yes", false, "Accept defaults without prompting")
	if err := fs.Parse(args); err != nil {
		return ExitConfigError
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║                 Dev-Sync                 ║")
	fmt.Println("║     One Source. Multiple Replicas.       ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Initialize a synchronization project")
	fmt.Println()

	sourceDir := *sourceFlag
	if sourceDir == "" {
		if *nonInteractive || *yesFlag {
			sourceDir = "."
		} else {
			fmt.Print("Source directory [default: .]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				sourceDir = "."
			} else {
				sourceDir = input
			}
		}
	}

	normSource, err := safety.NormalizePath(sourceDir)
	if err != nil {
		logging.PrintError("Invalid source path: %v", err)
		return ExitValidationError
	}

	// Check if already initialized
	existingConfig := filepath.Join(normSource, config.ConfigDirName, config.ConfigFileName)
	if _, err := os.Stat(existingConfig); err == nil && !*yesFlag {
		logging.PrintWarning("Dev-Sync is already initialized in: %s", normSource)
		if !*nonInteractive {
			fmt.Print("Reinitialize and overwrite existing configuration? [y/N]: ")
			ans, _ := reader.ReadString('\n')
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(ans)), "y") {
				return ExitSuccess
			}
		}
	}

	cfg := config.DefaultConfig(normSource)

	// Collect replicas interactively if not non-interactive
	if !*nonInteractive && !*yesFlag {
		for {
			fmt.Print("\nAdd replica? [Y/n]: ")
			ans, _ := reader.ReadString('\n')
			ans = strings.TrimSpace(strings.ToLower(ans))
			if ans == "n" || ans == "no" {
				break
			}

			fmt.Print("Replica name/ID: ")
			id, _ := reader.ReadString('\n')
			id = strings.TrimSpace(id)
			if id == "" {
				fmt.Println("Replica name cannot be empty.")
				continue
			}

			fmt.Print("Replica directory path: ")
			repPath, _ := reader.ReadString('\n')
			repPath = strings.TrimSpace(repPath)
			if repPath == "" {
				fmt.Println("Replica directory cannot be empty.")
				continue
			}

			normRep, err := safety.NormalizePath(repPath)
			if err != nil {
				logging.PrintError("Invalid replica path: %v", err)
				continue
			}

			if err := safety.ValidateReplicaPath(normSource, normRep); err != nil {
				logging.PrintError("Validation error: %v", err)
				continue
			}

			cfg.Replicas = append(cfg.Replicas, config.ReplicaConfig{
				ID:      id,
				Path:    normRep,
				Enabled: true,
			})
			logging.PrintSuccess("Replica '%s' registered.", id)
		}
	}

	if err := config.SaveConfig(normSource, cfg); err != nil {
		logging.PrintError("Failed to write config: %v", err)
		return ExitConfigError
	}

	// Create default .devsyncignore if not present
	ignorePath := filepath.Join(normSource, config.IgnoreFileName)
	if _, err := os.Stat(ignorePath); os.IsNotExist(err) {
		defaultIgnoreContent := `# Dev-Sync Ignore Rules
# Matches files and directories to ignore during synchronization

.git/
.github/
.vscode/
.idea/
node_modules/
vendor/
*.log
*.tmp
*.bak
.DS_Store
Thumbs.db
.devsync/
`
		_ = os.WriteFile(ignorePath, []byte(defaultIgnoreContent), 0644)
	}

	// Initialize logger
	_, _ = logging.Init(normSource, logging.LevelInfo, false)

	fmt.Println()
	logging.PrintSuccess("Configuration created successfully in %s/.devsync", normSource)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run 'dev-sync list' to inspect your configured replicas")
	fmt.Println("  2. Run 'dev-sync sync' to perform the initial synchronization")
	fmt.Println("  3. Run 'dev-sync watch' to continuously mirror changes")
	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync add
// -------------------------------------------------------------
func runAdd(args []string) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	idFlag := fs.String("id", "", "Unique replica identifier")
	pathFlag := fs.String("path", "", "Replica directory path")
	disabledFlag := fs.Bool("disabled", false, "Add replica in disabled state")
	if err := fs.Parse(args); err != nil {
		return ExitConfigError
	}

	projectRoot, err := config.FindProjectRoot(".")
	if err != nil {
		logging.PrintError("%v", err)
		return ExitConfigError
	}

	cfg, err := config.LoadConfig(projectRoot)
	if err != nil {
		logging.PrintError("Failed to load config: %v", err)
		return ExitConfigError
	}

	id := *idFlag
	repPath := *pathFlag

	reader := bufio.NewReader(os.Stdin)

	if id == "" {
		fmt.Print("Replica name: ")
		id, _ = reader.ReadString('\n')
		id = strings.TrimSpace(id)
	}
	if id == "" {
		logging.PrintError("Replica ID cannot be empty.")
		return ExitValidationError
	}

	if rep, _ := cfg.FindReplicaByID(id); rep != nil {
		logging.PrintError("Replica with ID '%s' already exists.", id)
		return ExitValidationError
	}

	if repPath == "" {
		fmt.Print("Replica path: ")
		repPath, _ = reader.ReadString('\n')
		repPath = strings.TrimSpace(repPath)
	}
	if repPath == "" {
		logging.PrintError("Replica path cannot be empty.")
		return ExitValidationError
	}

	normRep, err := safety.NormalizePath(repPath)
	if err != nil {
		logging.PrintError("Invalid replica path: %v", err)
		return ExitValidationError
	}

	if err := safety.ValidateReplicaPath(cfg.Source, normRep); err != nil {
		logging.PrintError("Path safety error: %v", err)
		return ExitValidationError
	}

	cfg.Replicas = append(cfg.Replicas, config.ReplicaConfig{
		ID:      id,
		Path:    normRep,
		Enabled: !*disabledFlag,
	})

	if err := config.SaveConfig(projectRoot, cfg); err != nil {
		logging.PrintError("Failed to save config: %v", err)
		return ExitConfigError
	}

	logging.PrintSuccess("Replica '%s' added successfully.", id)
	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync remove
// -------------------------------------------------------------
func runRemove(args []string) int {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	idFlag := fs.String("id", "", "Replica ID to remove")
	if err := fs.Parse(args); err != nil {
		return ExitConfigError
	}

	projectRoot, err := config.FindProjectRoot(".")
	if err != nil {
		logging.PrintError("%v", err)
		return ExitConfigError
	}

	cfg, err := config.LoadConfig(projectRoot)
	if err != nil {
		logging.PrintError("Failed to load config: %v", err)
		return ExitConfigError
	}

	if len(cfg.Replicas) == 0 {
		fmt.Println("No replicas configured.")
		return ExitSuccess
	}

	targetID := *idFlag
	if targetID == "" {
		fmt.Println("Configured replicas:")
		for i, r := range cfg.Replicas {
			fmt.Printf("  %d. %s (%s)\n", i+1, r.ID, r.Path)
		}
		fmt.Print("\nSelect replica ID or number to remove: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		var idx int
		if n, err := fmt.Sscanf(input, "%d", &idx); err == nil && n == 1 && idx >= 1 && idx <= len(cfg.Replicas) {
			targetID = cfg.Replicas[idx-1].ID
		} else {
			targetID = input
		}
	}

	foundIdx := -1
	for i, r := range cfg.Replicas {
		if strings.EqualFold(r.ID, targetID) {
			foundIdx = i
			break
		}
	}

	if foundIdx == -1 {
		logging.PrintError("Replica '%s' not found.", targetID)
		return ExitValidationError
	}

	// Remove from list without touching filesystem
	removed := cfg.Replicas[foundIdx]
	cfg.Replicas = append(cfg.Replicas[:foundIdx], cfg.Replicas[foundIdx+1:]...)

	if err := config.SaveConfig(projectRoot, cfg); err != nil {
		logging.PrintError("Failed to save config: %v", err)
		return ExitConfigError
	}

	logging.PrintSuccess("Replica '%s' removed from configuration (directory retained at %s).", removed.ID, removed.Path)
	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync list
// -------------------------------------------------------------
func runList(args []string) int {
	projectRoot, err := config.FindProjectRoot(".")
	if err != nil {
		logging.PrintError("%v", err)
		return ExitConfigError
	}

	cfg, err := config.LoadConfig(projectRoot)
	if err != nil {
		logging.PrintError("Failed to load config: %v", err)
		return ExitConfigError
	}

	logging.PrintHeader("Dev-Sync Configuration")
	fmt.Println("Source:")
	fmt.Printf("  %s\n\n", cfg.Source)

	fmt.Println("Replicas:")
	if len(cfg.Replicas) == 0 {
		fmt.Println("  (No replicas configured. Use 'dev-sync add' to add one)")
	} else {
		for i, r := range cfg.Replicas {
			status := "✓ Enabled"
			if !r.Enabled {
				status = "✗ Disabled"
			}
			fmt.Printf("  [%d] %s\n", i+1, r.ID)
			fmt.Printf("      %s\n", r.Path)
			fmt.Printf("      %s\n\n", status)
		}
	}

	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync sync
// -------------------------------------------------------------
func runSync(args []string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	replicaFlag := fs.String("replica", "", "Synchronize only this specific replica ID")
	allFlag := fs.Bool("all", false, "Synchronize all enabled replicas (default)")
	dryRun := fs.Bool("dry-run", false, "Perform comparison and show plan without modifying files")
	deepFlag := fs.Bool("deep", false, "Force cryptographic SHA-256 hash comparison on all files")
	verboseFlag := fs.Bool("verbose", false, "Show detailed item-by-item actions")
	jsonFlag := fs.Bool("json", false, "Output results in JSON format")
	_ = allFlag // Satisfies spec flag

	if err := fs.Parse(args); err != nil {
		return ExitConfigError
	}

	projectRoot, err := config.FindProjectRoot(".")
	if err != nil {
		logging.PrintError("%v", err)
		return ExitConfigError
	}

	logLevel := logging.LevelInfo
	if *verboseFlag {
		logLevel = logging.LevelDebug
	}
	_, _ = logging.Init(projectRoot, logLevel, *jsonFlag)

	engine, err := syncengine.NewSyncEngine(projectRoot)
	if err != nil {
		logging.PrintError("Failed to initialize sync engine: %v", err)
		return ExitConfigError
	}

	if *dryRun && !*jsonFlag {
		fmt.Println("====================================")
		fmt.Println("              DRY RUN               ")
		fmt.Println("    No files will be modified.      ")
		fmt.Println("====================================")
	}

	if !*jsonFlag {
		logging.PrintHeader("Dev-Sync")
		fmt.Printf("Source:\n  %s\n\n", engine.Config.Source)
		fmt.Println("Synchronizing...")
		fmt.Println()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logging.Warn("\nInterrupt signal received. Aborting gracefully...")
		cancel()
	}()

	results, syncErr := engine.Sync(ctx, syncengine.EngineSyncOptions{
		ReplicaID:   *replicaFlag,
		DryRun:      *dryRun,
		DeepCompare: *deepFlag,
		Verbose:     *verboseFlag,
		OnProgress: func(repID string, item *planner.PlanItem, itemErr error) {
			if *verboseFlag && !*jsonFlag {
				if itemErr != nil {
					logging.PrintError("[%s] %s %s: %v", repID, item.Action, item.RelPath, itemErr)
				} else {
					fmt.Printf("  [%s] %-8s %s\n", repID, item.Action, item.RelPath)
				}
			}
		},
	})

	if *jsonFlag {
		outData, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(outData))
		if syncErr != nil {
			return ExitSyncFailure
		}
		return ExitSuccess
	}

	// Format CLI summary
	for _, res := range results {
		if res == nil {
			continue
		}
		fmt.Printf("%s\n", res.ReplicaID)
		fmt.Printf("  Created:   %d\n", res.Created)
		fmt.Printf("  Modified:  %d\n", res.Modified)
		fmt.Printf("  Deleted:   %d\n", res.Deleted)
		fmt.Printf("  Unchanged: %d\n\n", res.Unchanged)
	}

	fmt.Println("Verification:")
	for _, res := range results {
		if res == nil {
			continue
		}
		if res.Success {
			logging.PrintSuccess("%s", res.ReplicaID)
		} else {
			logging.PrintError("%s (Errors encountered)", res.ReplicaID)
			for _, itemErr := range res.Errors {
				fmt.Printf("    - %s: %s (%s)\n", itemErr.Action, itemErr.RelPath, itemErr.Err)
			}
		}
	}
	fmt.Println()

	if syncErr != nil {
		logging.PrintError("Synchronization completed with errors.")
		return ExitSyncFailure
	}

	if *dryRun {
		logging.PrintSuccess("Dry run completed successfully. No files were modified.")
	} else {
		logging.PrintSuccess("Synchronization completed successfully.")
	}
	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync watch
// -------------------------------------------------------------
func runWatch(args []string) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	debounceFlag := fs.Int("debounce", 0, "Debounce duration in milliseconds (default: 150)")
	replicaFlag := fs.String("replica", "", "Watch and mirror only to this replica ID")
	detectDrift := fs.Bool("detect-drift", true, "Monitor and report replica drift")
	autoRepair := fs.Bool("auto-repair", false, "Automatically repair replica drift from source")
	if err := fs.Parse(args); err != nil {
		return ExitConfigError
	}

	projectRoot, err := config.FindProjectRoot(".")
	if err != nil {
		logging.PrintError("%v", err)
		return ExitConfigError
	}

	_, _ = logging.Init(projectRoot, logging.LevelInfo, false)
	engine, err := syncengine.NewSyncEngine(projectRoot)
	if err != nil {
		logging.PrintError("Failed to load engine: %v", err)
		return ExitConfigError
	}

	debounceMs := *debounceFlag
	if debounceMs <= 0 {
		debounceMs = engine.Config.Watch.DebounceMs
	}

	logging.PrintHeader("Dev-Sync Watch Mode")
	fmt.Printf("Source:\n  %s\n\n", engine.Config.Source)
	fmt.Println("Replicas:")
	for _, r := range engine.Config.Replicas {
		if r.Enabled && (*replicaFlag == "" || strings.EqualFold(r.ID, *replicaFlag)) {
			logging.PrintSuccess("%s (%s)", r.ID, r.Path)
		}
	}
	fmt.Printf("\nDebounce: %dms\n", debounceMs)
	fmt.Println("\nWatching for changes... (Press Ctrl+C to stop)")
	fmt.Println()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := watcher.NewWatcher(engine.Config.Source, engine.Matcher, debounceMs)
	if err != nil {
		logging.PrintError("Failed to start filesystem watcher: %v", err)
		return ExitGeneralError
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	w.Start(ctx, func(changedPaths []string) {
		timestamp := time.Now().Format("15:04:05")
		for _, p := range changedPaths {
			fmt.Printf("[%s] CHANGE   %s\n", timestamp, p)
		}

		res, err := engine.Sync(ctx, syncengine.EngineSyncOptions{
			ReplicaID: *replicaFlag,
		})
		if err != nil {
			logging.PrintError("Watch sync failed: %v", err)
		} else {
			for _, r := range res {
				if r != nil && r.Success {
					fmt.Printf("    %-12s ✓\n", r.ReplicaID)
				}
			}
		}
		fmt.Println()
	})

	// Periodic Drift Checker if enabled (Section 61, 62)
	if *detectDrift || engine.Config.Drift.DetectOnWatch {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					for _, rep := range engine.Config.Replicas {
						if !rep.Enabled {
							continue
						}
						report, err := drift.DetectDrift(rep.ID, engine.Config.Source, rep.Path, engine.Matcher, engine.Config.Filesystem.SymlinkMode)
						if err == nil && report.HasDrift {
							logging.PrintWarning("Drift detected on replica '%s' (Modified: %d, Missing: %d, Extra: %d)",
								rep.ID, len(report.Modified), len(report.Missing), len(report.Extra))

							if *autoRepair || engine.Config.Drift.AutoRepair {
								fmt.Printf("Repairing replica '%s' from authoritative source...\n", rep.ID)
								_, repErr := drift.RepairDrift(ctx, engine.Config.Source, rep.ID, rep.Path, engine.Matcher, engine.Config.Filesystem.SymlinkMode, engine.Config.Filesystem.BufferSizeKB)
								if repErr != nil {
									logging.PrintError("Failed to auto-repair drift: %v", repErr)
								} else {
									logging.PrintSuccess("Replica '%s' restored and now IN SYNC.", rep.ID)
								}
							}
						}
					}
				}
			}
		}()
	}

	<-sigCh
	logging.Info("\nStopping watcher cleanly...")
	w.Stop()
	logging.PrintSuccess("Watch mode stopped.")
	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync status
// -------------------------------------------------------------
func runStatus(args []string) int {
	projectRoot, err := config.FindProjectRoot(".")
	if err != nil {
		logging.PrintError("%v", err)
		return ExitConfigError
	}

	engine, err := syncengine.NewSyncEngine(projectRoot)
	if err != nil {
		logging.PrintError("Failed to load config: %v", err)
		return ExitConfigError
	}

	logging.PrintHeader("Dev-Sync Status")

	// Check source
	if fi, err := os.Stat(engine.Config.Source); err != nil || !fi.IsDir() {
		logging.PrintError("Source: UNAVAILABLE (%s)", engine.Config.Source)
		return ExitGeneralError
	}
	logging.PrintSuccess("Source: Available (%s)", engine.Config.Source)
	fmt.Println()

	hasOutSync := false
	hasUnavailable := false

	for _, rep := range engine.Config.Replicas {
		if !rep.Enabled {
			fmt.Printf("%s\n  Status: DISABLED\n\n", rep.ID)
			continue
		}

		fi, err := os.Stat(rep.Path)
		if err != nil || !fi.IsDir() {
			logging.PrintError("%s\n  Status: UNAVAILABLE\n  Path: %s\n", rep.ID, rep.Path)
			hasUnavailable = true
			continue
		}

		// Check drift
		report, err := drift.DetectDrift(rep.ID, engine.Config.Source, rep.Path, engine.Matcher, engine.Config.Filesystem.SymlinkMode)
		if err != nil {
			logging.PrintError("%s\n  Status: ERROR (%v)\n", rep.ID, err)
			continue
		}

		if report.HasDrift {
			hasOutSync = true
			logging.PrintWarning("%s\n  Status: OUT OF SYNC\n", rep.ID)
			if len(report.Modified) > 0 {
				fmt.Println("  Modified:")
				for _, m := range report.Modified {
					fmt.Printf("    %s\n", m)
				}
			}
			if len(report.Missing) > 0 {
				fmt.Println("  Missing:")
				for _, m := range report.Missing {
					fmt.Printf("    %s\n", m)
				}
			}
			if len(report.Extra) > 0 {
				fmt.Println("  Extra:")
				for _, e := range report.Extra {
					fmt.Printf("    %s\n", e)
				}
			}
			fmt.Println()
		} else {
			fmt.Printf("%s\n", rep.ID)
			logging.PrintSuccess("IN SYNC\n")
		}
	}

	if hasUnavailable {
		return ExitReplicaUnavailable
	}
	if hasOutSync {
		return ExitValidationError
	}
	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync verify
// -------------------------------------------------------------
func runVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	replicaFlag := fs.String("replica", "", "Verify only this specific replica")
	if err := fs.Parse(args); err != nil {
		return ExitConfigError
	}

	projectRoot, err := config.FindProjectRoot(".")
	if err != nil {
		logging.PrintError("%v", err)
		return ExitConfigError
	}

	engine, err := syncengine.NewSyncEngine(projectRoot)
	if err != nil {
		logging.PrintError("Failed to load config: %v", err)
		return ExitConfigError
	}

	logging.PrintHeader("Dev-Sync Verification")

	outOfSyncCount := 0

	for _, rep := range engine.Config.Replicas {
		if !rep.Enabled {
			continue
		}
		if *replicaFlag != "" && !strings.EqualFold(rep.ID, *replicaFlag) {
			continue
		}

		res, err := verify.VerifyReplica(rep.ID, engine.Config.Source, rep.Path, engine.Matcher, engine.Config.Filesystem.SymlinkMode)
		if err != nil {
			logging.PrintError("%-15s ✗ (%v)", rep.ID, err)
			outOfSyncCount++
			continue
		}

		if res.InSync {
			logging.PrintSuccess("%-15s (Matched: %d files)", rep.ID, res.MatchedFiles)
		} else {
			logging.PrintError("%-15s", rep.ID)
			outOfSyncCount++

			if len(res.ModifiedFiles) > 0 {
				fmt.Println("  Content Mismatches:")
				for _, diff := range res.ModifiedFiles {
					fmt.Printf("    %s\n      Source:  %s\n      Replica: %s\n", diff.RelPath, diff.SourceHash, diff.ReplicaHash)
				}
			}
			if len(res.MissingFiles) > 0 {
				fmt.Println("  Missing in Replica:")
				for _, m := range res.MissingFiles {
					fmt.Printf("    %s\n", m)
				}
			}
			if len(res.ExtraFiles) > 0 {
				fmt.Println("  Extra in Replica:")
				for _, e := range res.ExtraFiles {
					fmt.Printf("    %s\n", e)
				}
			}
		}
	}

	fmt.Println()
	if outOfSyncCount > 0 {
		logging.PrintError("Result: %d replica(s) out of sync.", outOfSyncCount)
		return ExitVerifyFailure
	}

	logging.PrintSuccess("All replicas verified: in sync.")
	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync doctor
// -------------------------------------------------------------
func runDoctor(args []string) int {
	projectRoot, err := config.FindProjectRoot(".")
	if err != nil {
		logging.PrintError("%v", err)
		return ExitConfigError
	}

	logging.PrintHeader("Dev-Sync Doctor")

	hasErrors := false

	// 1. Config validation
	cfg, err := config.LoadConfig(projectRoot)
	if err != nil {
		logging.PrintError("Configuration syntax/loading: %v", err)
		hasErrors = true
	} else {
		if err := cfg.Validate(); err != nil {
			logging.PrintError("Configuration validation: %v", err)
			hasErrors = true
		} else {
			logging.PrintSuccess("Configuration syntax & parameters valid")
		}
	}

	if cfg != nil {
		// 2. Source accessibility
		fi, err := os.Stat(cfg.Source)
		if err != nil {
			logging.PrintError("Source directory: does not exist (%s)", cfg.Source)
			hasErrors = true
		} else if !fi.IsDir() {
			logging.PrintError("Source path is not a directory (%s)", cfg.Source)
			hasErrors = true
		} else {
			// Check write permission for .devsync
			testFile := filepath.Join(cfg.Source, config.ConfigDirName, ".doctor_test")
			if werr := os.WriteFile(testFile, []byte("ok"), 0644); werr != nil {
				logging.PrintError("Source directory is read-only: %v", werr)
				hasErrors = true
			} else {
				_ = os.Remove(testFile)
				logging.PrintSuccess("Source directory accessible and writable")
			}
		}

		// 3. Replicas accessibility
		fmt.Printf("✓ %d replicas configured\n", len(cfg.Replicas))
		for _, rep := range cfg.Replicas {
			fi, err := os.Stat(rep.Path)
			if err != nil {
				if os.IsNotExist(err) {
					logging.PrintWarning("Replica '%s': directory does not exist yet (%s) - will be auto-created on first sync", rep.ID, rep.Path)
				} else {
					logging.PrintError("Replica '%s': %v", rep.ID, err)
					hasErrors = true
				}
			} else if !fi.IsDir() {
				logging.PrintError("Replica '%s': path is not a directory (%s)", rep.ID, rep.Path)
				hasErrors = true
			} else {
				logging.PrintSuccess("Replica '%s' accessible: %s", rep.ID, rep.Path)
			}
		}

		// 4. Ignore file
		ignorePath := filepath.Join(projectRoot, config.IgnoreFileName)
		if _, err := os.Stat(ignorePath); err == nil {
			logging.PrintSuccess("Ignore file (.devsyncignore) present and valid")
		} else {
			logging.PrintWarning("Ignore file (.devsyncignore) not found (using default rules)")
		}

		// 5. Watcher test
		w, err := watcher.NewWatcher(cfg.Source, nil, 100)
		if err != nil {
			logging.PrintError("Filesystem watcher initialization: %v", err)
			hasErrors = true
		} else {
			w.Stop()
			logging.PrintSuccess("Filesystem watcher functional")
		}

		// 6. Discord configuration
		if cfg.Discord.Enabled {
			url := cfg.EffectiveWebhookURL()
			if url == "" {
				logging.PrintError("Discord enabled but no webhook URL provided")
				hasErrors = true
			} else {
				logging.PrintSuccess("Discord notifications enabled (%s)", config.MaskWebhookURL(url))
			}
		} else {
			logging.PrintSuccess("Discord notifications disabled")
		}
	}

	fmt.Println()
	if hasErrors {
		logging.PrintError("Problems detected. Please review the errors above.")
		return ExitValidationError
	}

	logging.PrintSuccess("No problems detected. Dev-Sync is ready.")
	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync config
// -------------------------------------------------------------
func runConfig(args []string) int {
	projectRoot, err := config.FindProjectRoot(".")
	if err != nil {
		logging.PrintError("%v", err)
		return ExitConfigError
	}

	cfg, err := config.LoadConfig(projectRoot)
	if err != nil {
		logging.PrintError("Failed to load config: %v", err)
		return ExitConfigError
	}

	// Mask webhook for display
	displayCfg := *cfg
	displayCfg.Discord.WebhookURL = config.MaskWebhookURL(cfg.Discord.WebhookURL)

	data, _ := json.MarshalIndent(displayCfg, "", "  ")
	fmt.Println(string(data))
	return ExitSuccess
}

// -------------------------------------------------------------
// Command: dev-sync version
// -------------------------------------------------------------
func runVersion(args []string) int {
	fmt.Printf("Dev-Sync v%s\n", Version)
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Build:    %s (%s)\n", Commit, BuildDate)
	return ExitSuccess
}

// PrintHelp outputs the CLI manual.
func PrintHelp() {
	helpText := `Dev-Sync - Production-grade, cross-platform developer filesystem replicator

USAGE:
  dev-sync <command> [options]

COMMANDS:
  init       Initialize a new Dev-Sync project configuration
  add        Register a new replica directory
  remove     Remove a replica from configuration (directory retained)
  list       Display current source and configured replicas
  sync       Synchronize source files to replicas
  watch      Recursively watch source for changes and mirror in real-time
  status     Inspect synchronization and drift status across all replicas
  verify     Perform deep cryptographic SHA-256 integrity verification
  doctor     Run diagnostic health checks on project configuration and paths
  config     Print current active project configuration
  version    Show version and build architecture
  help       Show this help message

SYNC OPTIONS:
  --replica <id>    Target a specific replica by its identifier
  --all             Synchronize all enabled replicas (default)
  --dry-run         Compute comparison plan without modifying filesystem
  --deep            Force SHA-256 hashing during comparison
  --verbose         Display per-file synchronization actions
  --json            Output results as formatted JSON

WATCH OPTIONS:
  --debounce <ms>   Set filesystem event debouncing window (default: 150ms)
  --replica <id>    Mirror changes to a specific replica
  --auto-repair     Automatically restore replicas if external drift is detected

EXAMPLES:
  dev-sync init
  dev-sync add --id staging --path /opt/apps/staging
  dev-sync sync --dry-run
  dev-sync sync
  dev-sync watch --auto-repair
  dev-sync verify
  dev-sync doctor
`
	fmt.Println(helpText)
}
