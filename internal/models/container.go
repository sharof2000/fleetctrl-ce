package models

// Container represents a Docker container
type Container struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Image       string   `json:"image"`
	Status      string   `json:"status"`
	State       string   `json:"state"`
	Created     string   `json:"created"`
	Ports       []string `json:"ports,omitempty"`
	Project     string   `json:"project,omitempty"`      // Docker Compose project name (from label)
	ComposeFile string   `json:"compose_file,omitempty"` // Docker Compose file used
	ServiceName string   `json:"service_name,omitempty"` // Docker Compose service name (from label)
}

// ContainerGroup groups containers by project
type ContainerGroup struct {
	Name       string      `json:"name"`
	Containers []Container `json:"containers"`
	Running    int         `json:"running"`
	Stopped    int         `json:"stopped"`
	Restarting int         `json:"restarting"`
}

// ContainerStats holds container statistics for an application
type ContainerStats struct {
	Running    int `json:"running"`
	Stopped    int `json:"stopped"`
	Restarting int `json:"restarting"`
}

// ContainerResourceStats holds detailed resource usage for a container
type ContainerResourceStats struct {
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`

	// CPU metrics
	CPUPercent float64 `json:"cpu_percent"`
	CPUCores   int     `json:"cpu_cores"`

	// Memory metrics
	MemoryUsage   uint64  `json:"memory_usage"` // bytes
	MemoryLimit   uint64  `json:"memory_limit"` // bytes
	MemoryPercent float64 `json:"memory_percent"`

	// Network I/O (aggregate across all interfaces)
	NetRxBytes uint64 `json:"net_rx_bytes"`
	NetTxBytes uint64 `json:"net_tx_bytes"`

	// Block I/O
	BlockRead  uint64 `json:"block_read"`  // bytes
	BlockWrite uint64 `json:"block_write"` // bytes

	Timestamp int64 `json:"timestamp"`
}

// ContainerStatsSummary holds a brief summary of container stats
type ContainerStatsSummary struct {
	ContainerID   string  `json:"container_id"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
}

// BatchContainerStats holds stats for multiple containers (for batch API response)
type BatchContainerStats struct {
	Stats     map[string]ContainerResourceStats `json:"stats"`     // keyed by container ID
	Summaries map[string]ContainerStatsSummary  `json:"summaries"` // keyed by container ID
	Timestamp int64                             `json:"timestamp"`
}
