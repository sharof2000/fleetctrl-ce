package models

// Application represents a deployed application with one or more Docker Compose files
type Application struct {
	Name         string            `json:"name"`
	Path         string            `json:"path"`
	ComposeFiles []ComposeFile     `json:"compose_files"`
	Status       string            `json:"status"` // running, stopped, partial, unknown
	Env          map[string]string `json:"env,omitempty"`
	EnvContent   string            `json:"env_content,omitempty"` // Raw .env file content for editing
	HasEnvFile   bool              `json:"has_env_file"`          // Whether .env file exists
	GitURL       string            `json:"git_url,omitempty"`
	GitBranch    string            `json:"git_branch,omitempty"`
}

// ComposeFile represents a single docker-compose file within an application
type ComposeFile struct {
	Name          string      `json:"name"`           // e.g., "docker-compose.yml" or "docker-compose.jobs.yml"
	Path          string      `json:"path"`           // Full path to the file
	Status        string      `json:"status"`         // running, stopped, partial, unknown
	Containers    int         `json:"containers"`     // Number of containers from this compose file
	Running       int         `json:"running"`        // Number of running containers
	Stopped       int         `json:"stopped"`        // Number of stopped containers
	ContainerList []Container `json:"container_list"` // List of containers for this compose file
	ServiceList   []Service   `json:"service_list"`   // Parsed services with status (from compose file)
	TotalServices int         `json:"total_services"` // Total number of defined services
}
