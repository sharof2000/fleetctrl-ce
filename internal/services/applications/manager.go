package applications

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"fleetctrl/internal/config"
	"fleetctrl/internal/models"
	"fleetctrl/internal/services/compose"
	"fleetctrl/internal/services/docker"
)

// composeNameInvalid matches characters Docker Compose strips when deriving a
// project name from a directory.
var composeNameInvalid = regexp.MustCompile(`[^a-z0-9_-]`)

// normalizeProjectName mirrors how Docker Compose derives a project name from a
// directory: lowercase, drop characters outside [a-z0-9_-], trim leading separators.
// Folder "MyApp" -> "myapp", matching the com.docker.compose.project label.
func normalizeProjectName(s string) string {
	s = strings.ToLower(s)
	s = composeNameInvalid.ReplaceAllString(s, "")
	return strings.TrimLeft(s, "_-")
}

// wrapWriteErr returns a more actionable error when a write fails because the
// path is on a read-only mount as seen by this process. EROFS is distinct from
// EACCES: it indicates the mount itself (or the systemd/container namespace
// view of it) is read-only, even if the on-disk perms would allow the write.
func wrapWriteErr(err error, path string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EROFS) {
		return fmt.Errorf("cannot write %s: path is on a read-only mount as seen by fleetctrl "+
			"(check systemd ProtectHome/ReadOnlyPaths on the unit, or a :ro container volume): %w", path, err)
	}
	return err
}

// Manager handles application deployment and lifecycle
type Manager struct {
	config        *config.Config
	dockerService *docker.Service
}

// NewManager creates a new application manager
func NewManager(cfg *config.Config, dockerService *docker.Service) *Manager {
	return &Manager{
		config:        cfg,
		dockerService: dockerService,
	}
}

// List returns all applications found in the applications directory
func (m *Manager) List() ([]models.Application, error) {
	appsPath := m.config.Applications.Path

	// Ensure directory exists
	if _, err := os.Stat(appsPath); os.IsNotExist(err) {
		return []models.Application{}, nil
	}

	entries, err := os.ReadDir(appsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read applications directory: %w", err)
	}

	var apps []models.Application

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appPath := filepath.Join(appsPath, entry.Name())
		app, err := m.scanApplication(entry.Name(), appPath)
		if err != nil {
			continue // Skip apps with errors
		}

		if len(app.ComposeFiles) > 0 {
			apps = append(apps, *app)
		}
	}

	return apps, nil
}

// Get returns a single application with full details
func (m *Manager) Get(name string) (*models.Application, error) {
	appPath := filepath.Join(m.config.Applications.Path, name)

	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("application %s not found", name)
	}

	app, err := m.scanApplication(name, appPath)
	if err != nil {
		return nil, err
	}

	// Load env file content for editing
	envPath := filepath.Join(appPath, ".env")
	if content, err := os.ReadFile(envPath); err == nil {
		app.EnvContent = string(content)
	}

	return app, nil
}

// scanApplication scans an application directory for compose files
func (m *Manager) scanApplication(name, appPath string) (*models.Application, error) {
	app := &models.Application{
		Name:   name,
		Path:   appPath,
		Status: "unknown",
	}

	// Load env variables FIRST (before parsing compose files for variable substitution)
	envPath := filepath.Join(appPath, ".env")
	var envVars map[string]string
	// Check if .env file exists first
	if _, statErr := os.Stat(envPath); statErr == nil {
		app.HasEnvFile = true
		if env, err := readEnvFile(envPath); err == nil {
			app.Env = env
			envVars = env
		}
	}

	// Find all docker-compose*.yml files
	entries, err := os.ReadDir(appPath)
	if err != nil {
		return nil, err
	}

	// Get all containers for this project at once
	var allContainers []models.Container
	if m.dockerService != nil {
		allContainers = m.getProjectContainers(name)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		if strings.HasPrefix(fileName, "docker-compose") && (strings.HasSuffix(fileName, ".yml") || strings.HasSuffix(fileName, ".yaml")) {
			composeFilePath := filepath.Join(appPath, fileName)
			composeFile := models.ComposeFile{
				Name:          fileName,
				Path:          composeFilePath,
				Status:        "unknown",
				ContainerList: []models.Container{},
				ServiceList:   []models.Service{},
			}

			// Parse the compose file to get service definitions
			spec, parseErr := compose.ParseFile(composeFilePath)
			if parseErr == nil && spec != nil {
				// Pass env vars for variable substitution in image, ports, etc.
				services := spec.ExtractServices(envVars)
				composeFile.TotalServices = len(services)

				// Match running containers to services
				composeFile.ServiceList = m.mergeServicesWithContainers(services, allContainers, composeFilePath, fileName)

				// Calculate running/stopped counts from services
				for _, svc := range composeFile.ServiceList {
					if svc.Status == models.ServiceStatusRunning {
						composeFile.Running++
					} else if svc.Status != models.ServiceStatusNotStarted {
						composeFile.Stopped++
					}
				}
				composeFile.Containers = composeFile.Running + composeFile.Stopped

				// Determine status based on services
				if composeFile.TotalServices > 0 {
					if composeFile.Running == composeFile.TotalServices {
						composeFile.Status = "running"
					} else if composeFile.Running > 0 {
						composeFile.Status = "partial"
					} else {
						composeFile.Status = "stopped"
					}
				} else {
					composeFile.Status = "stopped"
				}
			} else {
				// Fallback to container-based status if parsing fails
				if m.dockerService != nil {
					for _, c := range allContainers {
						if m.containerMatchesComposeFile(c, composeFilePath, fileName) {
							composeFile.ContainerList = append(composeFile.ContainerList, c)
							switch c.State {
							case "running":
								composeFile.Running++
							case "exited", "dead":
								composeFile.Stopped++
							}
						}
					}
					composeFile.Containers = composeFile.Running + composeFile.Stopped

					if composeFile.Running > 0 && composeFile.Stopped == 0 {
						composeFile.Status = "running"
					} else if composeFile.Running == 0 && composeFile.Stopped > 0 {
						composeFile.Status = "stopped"
					} else if composeFile.Running > 0 && composeFile.Stopped > 0 {
						composeFile.Status = "partial"
					} else {
						composeFile.Status = "stopped"
					}
				}
			}

			app.ComposeFiles = append(app.ComposeFiles, composeFile)
		}
	}

	// Determine overall app status
	app.Status = m.determineAppStatus(app.ComposeFiles)

	return app, nil
}

// getProjectContainers returns all containers for a specific project
func (m *Manager) getProjectContainers(projectName string) []models.Container {
	if m.dockerService == nil {
		return nil
	}

	containers, err := m.dockerService.ListContainersWithLabels()
	if err != nil {
		return nil
	}

	target := normalizeProjectName(projectName)
	var result []models.Container
	for _, c := range containers {
		if normalizeProjectName(c.Project) == target {
			result = append(result, c)
		}
	}

	return result
}

// containerMatchesComposeFile checks if a container was started from a specific compose file
func (m *Manager) containerMatchesComposeFile(container models.Container, composeFilePath, composeFileName string) bool {
	if container.ComposeFile == "" {
		return composeFileName == "docker-compose.yml" || composeFileName == "docker-compose.yaml"
	}

	if strings.Contains(container.ComposeFile, composeFilePath) {
		return true
	}

	if strings.Contains(container.ComposeFile, composeFileName) {
		return true
	}

	return false
}

// determineAppStatus determines overall application status from compose files
func (m *Manager) determineAppStatus(composeFiles []models.ComposeFile) string {
	if len(composeFiles) == 0 {
		return "stopped"
	}

	running := 0
	stopped := 0

	for _, cf := range composeFiles {
		if cf.Running > 0 {
			running++
		} else {
			stopped++
		}
	}

	if running > 0 && stopped == 0 {
		return "running"
	} else if running == 0 && stopped > 0 {
		return "stopped"
	} else if running > 0 {
		return "partial"
	}

	return "stopped"
}

// mergeServicesWithContainers matches parsed services with running containers
func (m *Manager) mergeServicesWithContainers(services []models.Service, containers []models.Container, composeFilePath, composeFileName string) []models.Service {
	result := make([]models.Service, len(services))
	copy(result, services)

	for i := range result {
		svc := &result[i]

		// Find matching container by service name
		for _, c := range containers {
			if !m.containerMatchesComposeFile(c, composeFilePath, composeFileName) {
				continue
			}

			// Match by service name (from Docker label)
			if c.ServiceName == svc.Name {
				svc.Status = c.State
				svc.State = c.State
				svc.ContainerID = c.ID
				svc.ActualName = c.Name
				if len(c.Ports) > 0 {
					svc.Ports = c.Ports
				}
				break
			}
		}
	}

	return result
}

// GetComposeFile returns the content of a compose file
func (m *Manager) GetComposeFile(appName, fileName string) (string, error) {
	filePath := filepath.Join(m.config.Applications.Path, appName, fileName)

	if !strings.HasPrefix(fileName, "docker-compose") {
		return "", fmt.Errorf("invalid compose file name")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read compose file: %w", err)
	}

	return string(content), nil
}

// SaveComposeFile saves content to a compose file
func (m *Manager) SaveComposeFile(appName, fileName, content string) error {
	filePath := filepath.Join(m.config.Applications.Path, appName, fileName)

	if !strings.HasPrefix(fileName, "docker-compose") {
		return fmt.Errorf("invalid compose file name")
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return wrapWriteErr(err, filePath)
	}
	return nil
}

// GetEnvFile returns the content of the .env file
func (m *Manager) GetEnvFile(appName string) (string, error) {
	filePath := filepath.Join(m.config.Applications.Path, appName, ".env")

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read env file: %w", err)
	}

	return string(content), nil
}

// SaveEnvFile saves content to the .env file
func (m *Manager) SaveEnvFile(appName, content string) error {
	filePath := filepath.Join(m.config.Applications.Path, appName, ".env")

	// Use explicit open/write/sync/close to ensure data is flushed to disk
	// before docker compose reads the file.
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return wrapWriteErr(err, filePath)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to write env file: %w", err)
	}

	// Force flush to disk to ensure compose reads the new values.
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync env file: %w", err)
	}

	return nil
}

// StartCompose starts a specific compose file
func (m *Manager) StartCompose(appName, composeName string) error {
	if m.dockerService == nil {
		return fmt.Errorf("docker not available")
	}

	appPath := filepath.Join(m.config.Applications.Path, appName)
	return m.dockerService.ComposeUpFile(appPath, composeName)
}

// StopCompose stops a specific compose file
func (m *Manager) StopCompose(appName, composeName string) error {
	if m.dockerService == nil {
		return fmt.Errorf("docker not available")
	}

	appPath := filepath.Join(m.config.Applications.Path, appName)
	return m.dockerService.ComposeStopFile(appPath, composeName)
}

// RestartCompose restarts a specific compose file
func (m *Manager) RestartCompose(appName, composeName string) error {
	if m.dockerService == nil {
		return fmt.Errorf("docker not available")
	}

	appPath := filepath.Join(m.config.Applications.Path, appName)
	return m.dockerService.ComposeRestartFile(appPath, composeName)
}

// DownCompose stops and removes containers for a specific compose file
func (m *Manager) DownCompose(appName, composeName string) error {
	if m.dockerService == nil {
		return fmt.Errorf("docker not available")
	}

	appPath := filepath.Join(m.config.Applications.Path, appName)
	return m.dockerService.ComposeDownFile(appPath, composeName)
}

// StartService starts a specific service within a compose file
func (m *Manager) StartService(appName, composeName, serviceName string) error {
	if m.dockerService == nil {
		return fmt.Errorf("docker not available")
	}

	appPath := filepath.Join(m.config.Applications.Path, appName)
	return m.dockerService.ComposeUpService(appPath, composeName, serviceName)
}

// StopService stops a specific service within a compose file
func (m *Manager) StopService(appName, composeName, serviceName string) error {
	if m.dockerService == nil {
		return fmt.Errorf("docker not available")
	}

	appPath := filepath.Join(m.config.Applications.Path, appName)
	return m.dockerService.ComposeStopService(appPath, composeName, serviceName)
}

// RestartService restarts a specific service within a compose file
func (m *Manager) RestartService(appName, composeName, serviceName string) error {
	if m.dockerService == nil {
		return fmt.Errorf("docker not available")
	}

	appPath := filepath.Join(m.config.Applications.Path, appName)
	return m.dockerService.ComposeRestartService(appPath, composeName, serviceName)
}

// Delete removes an application and its containers
func (m *Manager) Delete(appName string) error {
	app, err := m.Get(appName)
	if err != nil {
		return err
	}

	// Stop all compose files
	if m.dockerService != nil {
		for _, cf := range app.ComposeFiles {
			m.dockerService.ComposeDownFile(app.Path, cf.Name)
		}
	}

	// Remove application directory
	return os.RemoveAll(app.Path)
}

// GitDeploy clones a git repository and sets up the application (if enabled)
func (m *Manager) GitDeploy(name, gitURL, branch string) error {
	if !m.config.Applications.GitDeployEnabled {
		return fmt.Errorf("git deployment is disabled")
	}

	appPath := filepath.Join(m.config.Applications.Path, name)

	// Check if app already exists
	if _, err := os.Stat(appPath); !os.IsNotExist(err) {
		return fmt.Errorf("application %s already exists", name)
	}

	// Clone repository
	if err := gitClone(gitURL, branch, appPath); err != nil {
		os.RemoveAll(appPath)
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	return nil
}

// GetApplicationNames returns a list of all application folder names
func (m *Manager) GetApplicationNames() ([]string, error) {
	appsPath := m.config.Applications.Path

	if _, err := os.Stat(appsPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(appsPath)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	return names, nil
}

// GetAppPath returns the filesystem path for an application
func (m *Manager) GetAppPath(appName string) (string, error) {
	appPath := filepath.Join(m.config.Applications.Path, appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return "", fmt.Errorf("application %s not found", appName)
	}
	return appPath, nil
}

// RunComposeWithOutput runs a compose command with streaming output
func (m *Manager) RunComposeWithOutput(appName, composeName, action string, outputFn func(line string)) error {
	if m.dockerService == nil {
		return fmt.Errorf("docker not available")
	}

	appPath := filepath.Join(m.config.Applications.Path, appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return fmt.Errorf("application %s not found", appName)
	}

	return m.dockerService.RunComposeWithOutput(appPath, composeName, action, outputFn)
}

// CreateBackup creates a ZIP archive of an application folder
// Returns the path to the created ZIP file
func (m *Manager) CreateBackup(appName string) (string, error) {
	appPath := filepath.Join(m.config.Applications.Path, appName)
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		return "", fmt.Errorf("application %s not found", appName)
	}

	// Create backup filename with date
	dateStr := time.Now().Format("20060102")
	backupName := fmt.Sprintf("%s_%s.zip", appName, dateStr)
	backupPath := filepath.Join(m.config.Applications.Path, backupName)

	// Create the ZIP file
	if err := createZipArchive(appPath, backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}

// createZipArchive creates a ZIP archive of a directory
func createZipArchive(sourceDir, destPath string) error {
	zipFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	baseName := filepath.Base(sourceDir)

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		archivePath := filepath.Join(baseName, relPath)
		archivePath = strings.ReplaceAll(archivePath, "\\", "/")

		if relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = archivePath

		if info.IsDir() {
			header.Name += "/"
			_, err = zipWriter.CreateHeader(header)
			return err
		}

		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}
