package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"silobang/internal/audit"
	"silobang/internal/auth"
	"silobang/internal/config"
	"silobang/internal/constants"
	"silobang/internal/database"
	"silobang/internal/logger"
	"silobang/internal/prompts"
	"silobang/internal/queries"
	"silobang/internal/server"
	"silobang/internal/version"
	"silobang/web"
)

func main() {
	// 0. Version flag
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("%s %s\n", constants.AppDisplayName, version.Version)
		os.Exit(0)
	}

	// 1. Initialize debug logger
	log := logger.NewLogger(constants.DefaultLogLevel)
	log.Info("%s version %s starting", constants.AppDisplayName, version.Version)

	// 2. Load or create config
	log.Info("Loading configuration...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Error("Failed to load config: %v", err)
		os.Exit(1)
	}
	log.Debug("Config directory: %s", config.GetConfigDir())

	// 3. Create application instance
	app := server.NewApp(cfg, log)

	// 4. If working_directory is set and valid, initialize it
	if cfg.WorkingDirectory != "" {
		log.Info("Initializing working directory: %s", cfg.WorkingDirectory)
		if err := config.InitializeWorkingDirectory(cfg.WorkingDirectory); err != nil {
			log.Error("Failed to initialize working directory: %v", err)
			cfg.WorkingDirectory = "" // Clear invalid path
		} else {
			// Open orchestrator DB
			orchPath := filepath.Join(cfg.WorkingDirectory, constants.InternalDir, constants.OrchestratorDB)
			orchDB, err := database.InitOrchestratorDB(orchPath)
			if err != nil {
				log.Error("Failed to open orchestrator database: %v", err)
				os.Exit(1)
			}
			app.OrchestratorDB = orchDB

			// Initialize audit logger
			app.AuditLogger = audit.NewLogger(orchDB, cfg.Audit.MaxLogSizeBytes, cfg.Audit.PurgePercentage)
			log.Debug("Audit logger initialized")

			// Re-initialize services now that orchestrator DB is available
			// (AuthService requires the DB and returns nil without it)
			app.SetOrchestratorDB(orchDB)
			app.ReinitServices()

			// Bootstrap auth: create admin user if no users exist
			authStore := auth.NewStore(orchDB, cfg.Auth.MaxLoginAttempts, cfg.Auth.LockoutDurationMins, cfg.Auth.SessionDuration())
			bootstrapResult, err := auth.Bootstrap(authStore, log)
			if err != nil {
				log.Error("Auth bootstrap failed: %v", err)
				os.Exit(1)
			}
			if bootstrapResult != nil {
				fmt.Println("╔══════════════════════════════════════════════════════════════╗")
				fmt.Println("║              INITIAL ADMIN CREDENTIALS                      ║")
				fmt.Println("║  Save these now — they will NOT be shown again.             ║")
				fmt.Println("╠══════════════════════════════════════════════════════════════╣")
				fmt.Printf("║  Username : %-48s║\n", bootstrapResult.Username)
				fmt.Printf("║  Password : %-48s║\n", bootstrapResult.Password)
				fmt.Printf("║  API Key  : %-48s║\n", bootstrapResult.APIKey)
				fmt.Println("╚══════════════════════════════════════════════════════════════╝")
				log.Info("Auth: bootstrap complete — admin account created")
			}

			// Enable file logging now that workdir is available
			if err := log.SetWorkDir(cfg.WorkingDirectory); err != nil {
				log.Warn("Failed to enable file logging: %v", err)
			} else {
				log.Info("File logging enabled in %s", cfg.WorkingDirectory)
			}

			// Discover existing topics
			topics, err := config.DiscoverTopics(cfg.WorkingDirectory)
			if err != nil {
				log.Warn("Topic discovery failed: %v", err)
			} else {
				log.Info("Discovered %d topic(s)", len(topics))
				for _, t := range topics {
					app.RegisterTopic(t.Name, t.Healthy, t.Error)
					if t.Healthy {
						log.Debug("  - %s (healthy)", t.Name)
						// Index to orchestrator
						if err := config.IndexTopicToOrchestrator(t.Path, t.Name, app.OrchestratorDB); err != nil {
							log.Warn("Failed to index topic %s: %v", t.Name, err)
						}
					} else {
						log.Warn("  - %s (unhealthy: %s)", t.Name, t.Error)
					}
				}
			}

			// Reconcile: purge orphaned asset_index entries for topics no longer on disk
		reconcileResult, reconcileErr := app.Services.Reconcile.Reconcile()
		if reconcileErr != nil {
			log.Warn("Reconciliation failed: %v", reconcileErr)
		} else if reconcileResult.TopicsRemoved > 0 {
			log.Info("Reconciliation: removed %d orphaned topic(s), purged %d index entries",
				reconcileResult.TopicsRemoved, reconcileResult.EntriesPurged)
		}

		// Load queries from .internal/queries/ directory
			queriesConfig, err := queries.LoadQueries(cfg.WorkingDirectory, log)
			if err != nil {
				log.Warn("Failed to load queries: %v, using defaults", err)
				queriesConfig = queries.GetDefaultConfig()
			}
			app.QueriesConfig = queriesConfig

			// Initialize prompts manager with base URL
			port := cfg.Port
			if port == 0 {
				port = constants.DefaultPort
			}
			baseURL := fmt.Sprintf("http://localhost:%d", port)
			promptsManager := prompts.NewManager(cfg.WorkingDirectory, baseURL)
			if err := promptsManager.EnsurePromptsDir(cfg.WorkingDirectory, log); err != nil {
				log.Warn("Failed to initialize prompts directory: %v", err)
			}
			if err := promptsManager.LoadPrompts(log); err != nil {
				log.Warn("Failed to load prompts: %v", err)
			}
			app.PromptsManager = promptsManager
		}
	} else {
		log.Warn("Working directory not set - configure via dashboard")
		// Use embedded defaults when no working directory
		app.QueriesConfig = queries.GetDefaultConfig()
		log.Debug("Using embedded query defaults (no working directory)")
	}

	// 5. Load embedded web frontend
	webFS, err := web.GetDistFS()
	if err != nil {
		log.Warn("Failed to load embedded web frontend: %v", err)
		// Continue without frontend - API still works
	}

	// 6. Start HTTP server
	port := cfg.Port
	if port == 0 {
		port = constants.DefaultPort
	}

	addr := fmt.Sprintf(":%d", port)
	srv := server.NewServer(app, addr, webFS)

	log.Info("Starting SiloBang server on port %d", port)
	if err := srv.Start(); err != nil {
		log.Error("Server error: %v", err)
		os.Exit(1)
	}
}
