package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"fleetctrl/internal/api"
	"fleetctrl/internal/config"
	"fleetctrl/internal/services/applications"
	"fleetctrl/internal/services/auth"
	"fleetctrl/internal/services/docker"
	"fleetctrl/internal/services/hosts"
	"fleetctrl/internal/services/monitor"
	"fleetctrl/internal/services/timeseries"
	"fleetctrl/internal/utils"
	"fleetctrl/internal/version"
	"fleetctrl/web"
)

func main() {
	// Parse command line flags
	showVersion := flag.Bool("version", false, "Show version")
	showVersionShort := flag.Bool("v", false, "Show version (short)")
	showHelp := flag.Bool("help", false, "Show help")
	showHelpShort := flag.Bool("h", false, "Show help (short)")

	flag.Parse()

	// Handle version flag
	if *showVersion || *showVersionShort {
		fmt.Printf("FleetCtrl %s\n", version.String())
		info := version.Get()
		fmt.Printf("  Build Time: %s\n", info.BuildTime)
		fmt.Printf("  Go Version: %s\n", info.GoVersion)
		fmt.Printf("  OS/Arch:    %s/%s\n", info.OS, info.Arch)
		os.Exit(0)
	}

	// Handle help flag
	if *showHelp || *showHelpShort {
		printHelp()
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[main] Failed to load config: %v", err)
	}

	log.Printf("[main] FleetCtrl %s starting...", version.String())

	// Initialize services
	monitorService := monitor.NewService()
	monitorService.Start()
	defer monitorService.Stop()

	authService := auth.NewService(cfg)

	// Initialize timeseries service
	var timeseriesService *timeseries.Service
	if cfg.Database.Enabled {
		tsConfig := timeseries.Config{
			Enabled: true,
			Path:    cfg.Database.Path,
			Retention: timeseries.RetentionConfig{
				RawSeconds:       cfg.Database.Retention.RawSeconds,
				MinuteAggSeconds: cfg.Database.Retention.MinuteAggSeconds,
				HourAggSeconds:   cfg.Database.Retention.HourAggSeconds,
				DayAggSeconds:    cfg.Database.Retention.DayAggSeconds,
			},
		}
		timeseriesService, err = timeseries.NewService(tsConfig)
		if err != nil {
			log.Printf("[main] Warning: Failed to initialize timeseries service: %v", err)
		} else {
			defer timeseriesService.Close()
		}
	}

	// Initialize hosts service
	hostsService := hosts.NewService(cfg, timeseriesService)
	hostsService.Start()
	defer hostsService.Stop()

	// Initialize docker service
	dockerService, err := docker.NewService()
	if err != nil {
		log.Printf("[main] Docker service: not available (%v)", err)
	} else if dockerService.IsAvailable() {
		log.Println("[main] Docker service: connected")
	} else {
		log.Println("[main] Docker service: not available (will retry automatically)")
	}

	// Initialize applications service
	var appsService *applications.Manager
	if cfg.Applications.Enabled {
		appsService = applications.NewManager(cfg, dockerService)
		log.Printf("[main] Applications service: enabled (path: %s)", cfg.Applications.Path)
	} else {
		log.Println("[main] Applications service: disabled")
	}

	// Detect local host and set it
	localIP := utils.GetLocalIP()
	localAddress := fmt.Sprintf("%s:%s", localIP, cfg.App.Port)
	log.Printf("[main] Local address: %s", localAddress)

	// Create router
	router := api.NewRouter(
		cfg,
		authService,
		monitorService,
		hostsService,
		timeseriesService,
		dockerService,
		appsService,
		web.TemplateFiles,
		web.StaticFiles,
	)

	// Start server
	addr := ":" + cfg.App.Port
	log.Printf("[main] Starting HTTP server on %s", addr)

	// Handle shutdown gracefully
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("[main] Shutting down...")
		os.Exit(0)
	}()

	if err := router.Run(addr); err != nil {
		log.Fatalf("[main] Failed to start server: %v", err)
	}
}

func printHelp() {
	fmt.Printf(`FleetCtrl Community Edition %s

A lightweight, cross-platform host monitoring application.

Usage:
  fleetctrl [flags]

Flags:
  -h, --help       Show this help message
  -v, --version    Show version information

Configuration:
  Config file is automatically loaded from:
    Linux:   ./config.yaml, ~/.config/fleetctrl/config.yaml, /etc/fleetctrl/config.yaml
    Windows: .\config.yaml, %%APPDATA%%\fleetctrl\config.yaml, C:\ProgramData\fleetctrl\config.yaml

  On first run, a default config will be created with:
    - Default port: 4060
    - Default username: admin
    - Password set on first login

For more information, visit: https://github.com/yourusername/fleetctrl

`, version.FullVersion())
}
