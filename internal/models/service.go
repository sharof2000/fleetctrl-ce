package models

// Service represents a service defined in a docker-compose file
// It unifies compose definitions with running container status
type Service struct {
	Name          string   `json:"name"`           // Service name from compose (e.g., "web")
	Image         string   `json:"image"`          // Image name (e.g., "nginx:latest")
	ContainerName string   `json:"container_name"` // Optional explicit container name
	Ports         []string `json:"ports"`          // Port mappings from compose
	Status        string   `json:"status"`         // running, stopped, exited, not_started
	State         string   `json:"state"`          // Docker state if running
	ContainerID   string   `json:"container_id"`   // Docker container ID if exists
	ActualName    string   `json:"actual_name"`    // Actual container name when running
	Source        string   `json:"source"`         // "compose" (defined) or "orphan" (running but not in compose)
}

// ServiceStatus constants
const (
	ServiceStatusRunning    = "running"
	ServiceStatusStopped    = "stopped"
	ServiceStatusExited     = "exited"
	ServiceStatusNotStarted = "not_started"
)
