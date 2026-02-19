package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	App          AppConfig          `yaml:"app"`
	Applications ApplicationsConfig `yaml:"applications"`
	Database     DatabaseConfig     `yaml:"database"`
	Dashboard    DashboardConfig    `yaml:"dashboard"`
	Auth         AuthConfig         `yaml:"auth"`
	Hosts        []HostConfig       `yaml:"hosts"`
}

// ApplicationsConfig holds applications feature settings
type ApplicationsConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Path             string `yaml:"path"`
	GitDeployEnabled bool   `yaml:"git_deploy_enabled"`
}

// AppConfig holds application settings
type AppConfig struct {
	Name     string `yaml:"name"`
	Port     string `yaml:"port"`
	LogLevel string `yaml:"log_level"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	Username       string `yaml:"username"`
	PasswordHash   string `yaml:"password_hash"`
	JWTSecret      string `yaml:"jwt_secret"`
	JWTExpiryHours int    `yaml:"jwt_expiry_hours"`
}

// HostConfig represents a host in the configuration
type HostConfig struct {
	ID      string `yaml:"id" json:"id"`
	Name    string `yaml:"name" json:"name"`
	Address string `yaml:"address" json:"address"`
	AddedAt string `yaml:"added_at" json:"added_at,omitempty"`
	Token   string `yaml:"token" json:"token,omitempty"` // Per-host token for peer communication
}

// DatabaseConfig holds time-series database settings
type DatabaseConfig struct {
	Enabled   bool            `yaml:"enabled"`
	Path      string          `yaml:"path"`
	Retention RetentionConfig `yaml:"retention"`
}

// RetentionConfig holds data retention settings
type RetentionConfig struct {
	RawSeconds       int64 `yaml:"raw_seconds"`
	MinuteAggSeconds int64 `yaml:"minute_agg_seconds"`
	HourAggSeconds   int64 `yaml:"hour_agg_seconds"`
	DayAggSeconds    int64 `yaml:"day_agg_seconds"`
}

// DashboardConfig holds dashboard settings
type DashboardConfig struct {
	TimelinePoints   int `yaml:"timeline_points"`
	TimelineInterval int `yaml:"timeline_interval"`
	RefreshInterval  int `yaml:"refresh_interval"`
}

// Load loads the configuration from file
func Load() (*Config, error) {
	configPath := findConfigPath()

	if configPath == "" {
		// No config file found, create default
		log.Println("[config] No config file found, creating default")
		cfg := DefaultConfig()
		if err := cfg.Save(); err != nil {
			return nil, err
		}
		log.Printf("[config] Created default config at: %s", getDefaultConfigPath())
		return cfg, nil
	}

	log.Printf("[config] Loading config from: %s", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Validate and apply defaults for missing values
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	log.Printf("[config] Loaded %d hosts", len(cfg.Hosts))

	return &cfg, nil
}

// DefaultConfig returns a new configuration with default values
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Port:     "4060",
			LogLevel: "info",
		},
		Applications: ApplicationsConfig{
			Enabled:          true,
			Path:             defaultApplicationsPath(),
			GitDeployEnabled: false,
		},
		Database: DatabaseConfig{
			Enabled: true,
			Path:    defaultDatabasePath(),
			Retention: RetentionConfig{
				RawSeconds:       3600,    // 1 hour
				MinuteAggSeconds: 86400,   // 24 hours
				HourAggSeconds:   604800,  // 7 days
				DayAggSeconds:    2592000, // 30 days
			},
		},
		Dashboard: DashboardConfig{
			TimelinePoints:   60,
			TimelineInterval: 60, // seconds
			RefreshInterval:  5,  // seconds
		},
		Auth: AuthConfig{
			Username:       "admin",
			PasswordHash:   "", // Set on first login
			JWTSecret:      GenerateRandomToken(32),
			JWTExpiryHours: 24,
		},
		Hosts: []HostConfig{},
	}
}

// Validate validates the configuration and applies defaults for missing values
func (c *Config) Validate() error {
	// App config validation
	if c.App.Port == "" {
		c.App.Port = "4060"
	}
	if _, err := strconv.Atoi(c.App.Port); err != nil {
		return fmt.Errorf("invalid port: %s", c.App.Port)
	}

	if c.App.LogLevel == "" {
		c.App.LogLevel = "info"
	}

	// Applications config validation
	if c.Applications.Path == "" {
		c.Applications.Path = defaultApplicationsPath()
	}

	// Database config validation
	if c.Database.Path == "" {
		c.Database.Path = defaultDatabasePath()
	}
	if c.Database.Retention.RawSeconds <= 0 {
		c.Database.Retention.RawSeconds = 3600 // 1 hour
	}
	if c.Database.Retention.MinuteAggSeconds <= 0 {
		c.Database.Retention.MinuteAggSeconds = 86400 // 24 hours
	}
	if c.Database.Retention.HourAggSeconds <= 0 {
		c.Database.Retention.HourAggSeconds = 604800 // 7 days
	}
	if c.Database.Retention.DayAggSeconds <= 0 {
		c.Database.Retention.DayAggSeconds = 2592000 // 30 days
	}

	// Dashboard config validation
	if c.Dashboard.TimelinePoints <= 0 {
		c.Dashboard.TimelinePoints = 60
	}
	if c.Dashboard.TimelineInterval <= 0 {
		c.Dashboard.TimelineInterval = 60 // seconds
	}
	if c.Dashboard.RefreshInterval <= 0 {
		c.Dashboard.RefreshInterval = 5 // seconds
	}

	// Auth config validation
	if c.Auth.Username == "" {
		c.Auth.Username = "admin"
	}

	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("jwt_secret is required")
	}

	if c.Auth.JWTExpiryHours <= 0 {
		c.Auth.JWTExpiryHours = 24
	}

	// Hosts validation
	if c.Hosts == nil {
		c.Hosts = []HostConfig{}
	}

	return nil
}

// GetHostByID returns a pointer to a host by its ID
func (c *Config) GetHostByID(id string) *HostConfig {
	for i := range c.Hosts {
		if c.Hosts[i].ID == id {
			return &c.Hosts[i]
		}
	}
	return nil
}

// AddHost adds a new host to the configuration
func (c *Config) AddHost(host HostConfig) error {
	// Check if host already exists
	for _, h := range c.Hosts {
		if h.ID == host.ID {
			return fmt.Errorf("host with ID %s already exists", host.ID)
		}
		if h.Address == host.Address {
			return fmt.Errorf("host with address %s already exists", host.Address)
		}
	}

	// Generate ID if not provided
	if host.ID == "" {
		host.ID = GenerateRandomToken(8)
	}

	// Set added timestamp
	if host.AddedAt == "" {
		host.AddedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Generate token for peer communication
	if host.Token == "" {
		host.Token = GenerateRandomToken(32)
	}

	c.Hosts = append(c.Hosts, host)
	return nil
}

// RemoveHost removes a host from the configuration
func (c *Config) RemoveHost(id string) error {
	for i, h := range c.Hosts {
		if h.ID == id {
			c.Hosts = append(c.Hosts[:i], c.Hosts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("host with ID %s not found", id)
}

// Save saves the configuration to file
func (c *Config) Save() error {
	configPath := getDefaultConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

// findConfigPath searches for config file in standard locations
func findConfigPath() string {
	paths := getConfigSearchPaths()
	log.Printf("[config] Search paths: %v", paths)
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			log.Printf("[config] Found config at: %s", p)
			return p
		}
	}
	return ""
}

// getConfigSearchPaths returns platform-specific config search paths
func getConfigSearchPaths() []string {
	// First, check next to executable
	execPath, err := os.Executable()
	execDir := ""
	if err == nil {
		execDir = filepath.Dir(execPath)
	}

	if runtime.GOOS == "windows" {
		paths := []string{}
		if execDir != "" {
			paths = append(paths, filepath.Join(execDir, "config.yaml"))
		}
		paths = append(paths,
			".\\config.yaml",
			filepath.Join(os.Getenv("APPDATA"), "fleetctrl", "config.yaml"),
			"C:\\ProgramData\\fleetctrl\\config.yaml",
		)
		return paths
	}

	home, _ := os.UserHomeDir()
	paths := []string{}
	if execDir != "" {
		paths = append(paths, filepath.Join(execDir, "config.yaml"))
	}
	paths = append(paths,
		"./config.yaml",
		filepath.Join(home, ".config", "fleetctrl", "config.yaml"),
		"/etc/fleetctrl/config.yaml",
	)
	return paths
}

// getDefaultConfigPath returns the default config path for saving
func getDefaultConfigPath() string {
	// Prefer saving next to executable
	execPath, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(execPath), "config.yaml")
	}
	return "./config.yaml"
}

// defaultDatabasePath returns the database path relative to the executable
func defaultDatabasePath() string {
	execPath, err := os.Executable()
	if err != nil {
		return "./fleetctrl.db"
	}
	return filepath.Join(filepath.Dir(execPath), "fleetctrl.db")
}

// defaultApplicationsPath returns the applications path relative to the executable
func defaultApplicationsPath() string {
	execPath, err := os.Executable()
	if err != nil {
		return "./applications"
	}
	return filepath.Join(filepath.Dir(execPath), "applications")
}

// GenerateRandomToken generates a random hex token of specified byte length
func GenerateRandomToken(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based token if crypto/rand fails
		return hex.EncodeToString([]byte(time.Now().String()))[:length*2]
	}
	return hex.EncodeToString(bytes)
}
