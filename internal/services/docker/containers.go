package docker

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"fleetctrl/internal/models"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// Docker Compose label constants
const (
	LabelComposeProject     = "com.docker.compose.project"
	LabelComposeConfigFiles = "com.docker.compose.project.config_files"
	LabelComposeService     = "com.docker.compose.service"
)

// ListContainers returns all containers
func (s *Service) ListContainers() ([]models.Container, error) {
	if err := s.checkAvailable(); err != nil {
		return nil, err
	}

	containers, err := s.client.ContainerList(s.ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]models.Container, len(containers))
	for i, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		result[i] = models.Container{
			ID:          c.ID[:12],
			Name:        name,
			Image:       c.Image,
			Status:      c.Status,
			State:       c.State,
			Created:     time.Unix(c.Created, 0).UTC().Format(time.RFC3339),
			Ports:       formatPorts(c.Ports),
			Project:     c.Labels[LabelComposeProject],
			ComposeFile: c.Labels[LabelComposeConfigFiles],
			ServiceName: c.Labels[LabelComposeService],
		}
	}

	return result, nil
}

// ListContainersWithLabels returns all containers with project labels (same as ListContainers, kept for clarity)
func (s *Service) ListContainersWithLabels() ([]models.Container, error) {
	return s.ListContainers()
}

// ListContainersByProjects returns containers filtered by project names
func (s *Service) ListContainersByProjects(projectNames []string) ([]models.Container, error) {
	allContainers, err := s.ListContainers()
	if err != nil {
		return nil, err
	}

	// Build lookup map
	projectSet := make(map[string]bool)
	for _, name := range projectNames {
		projectSet[name] = true
	}

	var filtered []models.Container
	for _, c := range allContainers {
		if projectSet[c.Project] {
			filtered = append(filtered, c)
		}
	}

	return filtered, nil
}

// GetContainerStatsByProjects returns container stats grouped by project
func (s *Service) GetContainerStatsByProjects(projectNames []string) (map[string]models.ContainerStats, error) {
	containers, err := s.ListContainersByProjects(projectNames)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]models.ContainerStats)

	for _, c := range containers {
		s := stats[c.Project]
		switch c.State {
		case "running":
			s.Running++
		case "exited", "dead":
			s.Stopped++
		case "restarting":
			s.Restarting++
		}
		stats[c.Project] = s
	}

	return stats, nil
}

// GroupContainersByProject groups containers by their project label
func (s *Service) GroupContainersByProject(projectNames []string) ([]models.ContainerGroup, error) {
	containers, err := s.ListContainersByProjects(projectNames)
	if err != nil {
		return nil, err
	}

	// Group by project
	groups := make(map[string]*models.ContainerGroup)
	for _, c := range containers {
		if c.Project == "" {
			continue
		}

		group, exists := groups[c.Project]
		if !exists {
			group = &models.ContainerGroup{
				Name:       c.Project,
				Containers: []models.Container{},
			}
			groups[c.Project] = group
		}

		group.Containers = append(group.Containers, c)

		switch c.State {
		case "running":
			group.Running++
		case "exited", "dead":
			group.Stopped++
		case "restarting":
			group.Restarting++
		}
	}

	// Convert to slice
	result := make([]models.ContainerGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, *group)
	}

	return result, nil
}

// StopContainer stops a container
func (s *Service) StopContainer(id string) error {
	if err := s.checkAvailable(); err != nil {
		return err
	}
	timeout := 10
	return s.client.ContainerStop(s.ctx, id, container.StopOptions{Timeout: &timeout})
}

// StartContainer starts a container
func (s *Service) StartContainer(id string) error {
	if err := s.checkAvailable(); err != nil {
		return err
	}
	return s.client.ContainerStart(s.ctx, id, container.StartOptions{})
}

// RestartContainer restarts a container
func (s *Service) RestartContainer(id string) error {
	if err := s.checkAvailable(); err != nil {
		return err
	}
	timeout := 10
	return s.client.ContainerRestart(s.ctx, id, container.StopOptions{Timeout: &timeout})
}

// GetContainerLogs returns container logs
func (s *Service) GetContainerLogs(id string, tail string) (string, error) {
	if err := s.checkAvailable(); err != nil {
		return "", err
	}

	// Inspect the container to determine if its stdio is multiplexed (non-TTY)
	// or a raw stream (TTY=true). The Docker logs endpoint returns 8-byte
	// stdcopy frame headers for non-TTY containers, which must be stripped
	// before the caller sees the payload.
	info, err := s.client.ContainerInspect(s.ctx, id)
	if err != nil {
		return "", err
	}
	multiplexed := info.Config == nil || !info.Config.Tty

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Timestamps: true,
	}

	reader, err := s.client.ContainerLogs(s.ctx, id, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	if !multiplexed {
		logs, err := io.ReadAll(reader)
		if err != nil {
			return "", err
		}
		return string(logs), nil
	}

	// Demultiplex Docker stdcopy frames: 8-byte header (stream type + length)
	// followed by payload. Merge stdout and stderr in receive order.
	var buf bytes.Buffer
	header := make([]byte, 8)
	for {
		_, err := io.ReadFull(reader, header)
		if err != nil {
			break
		}
		frameSize := binary.BigEndian.Uint32(header[4:8])
		if frameSize == 0 {
			continue
		}
		payload := make([]byte, frameSize)
		if _, err := io.ReadFull(reader, payload); err != nil {
			break
		}
		buf.Write(payload)
	}

	return buf.String(), nil
}

func formatPorts(ports []types.Port) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(ports))
	for _, p := range ports {
		if p.PublicPort > 0 {
			portStr := fmt.Sprintf("%d:%d", p.PublicPort, p.PrivatePort)
			if !seen[portStr] {
				seen[portStr] = true
				result = append(result, portStr)
			}
		}
	}
	return result
}

// GetContainerResourceStats fetches real-time stats for a single container
func (s *Service) GetContainerResourceStats(containerID string) (*models.ContainerResourceStats, error) {
	if err := s.checkAvailable(); err != nil {
		return nil, err
	}

	// Get stats with stream=false for a single snapshot
	statsResponse, err := s.client.ContainerStats(s.ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}
	defer statsResponse.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(statsResponse.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	return s.parseContainerStats(containerID, &stats), nil
}

// GetBatchContainerStats fetches stats for multiple containers concurrently
func (s *Service) GetBatchContainerStats(containerIDs []string) (*models.BatchContainerStats, error) {
	if err := s.checkAvailable(); err != nil {
		return nil, err
	}

	result := &models.BatchContainerStats{
		Stats:     make(map[string]models.ContainerResourceStats),
		Summaries: make(map[string]models.ContainerStatsSummary),
		Timestamp: time.Now().Unix(),
	}

	if len(containerIDs) == 0 {
		return result, nil
	}

	// Use a mutex to protect concurrent map writes
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to avoid overwhelming Docker API
	semaphore := make(chan struct{}, 5)

	for _, id := range containerIDs {
		wg.Add(1)
		go func(containerID string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			stats, err := s.GetContainerResourceStats(containerID)
			if err != nil {
				// Skip containers that fail (might be stopped)
				return
			}

			mu.Lock()
			result.Stats[containerID] = *stats
			result.Summaries[containerID] = models.ContainerStatsSummary{
				ContainerID:   containerID,
				CPUPercent:    stats.CPUPercent,
				MemoryPercent: stats.MemoryPercent,
			}
			mu.Unlock()
		}(id)
	}

	wg.Wait()
	return result, nil
}

// parseContainerStats converts Docker stats response to our model
func (s *Service) parseContainerStats(containerID string, stats *container.StatsResponse) *models.ContainerResourceStats {
	result := &models.ContainerResourceStats{
		ContainerID:   containerID,
		ContainerName: strings.TrimPrefix(stats.Name, "/"),
		Timestamp:     time.Now().Unix(),
	}

	// Calculate CPU percentage
	// CPU % = (container_cpu_delta / system_cpu_delta) * num_cpus * 100
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta > 0 && cpuDelta > 0 {
		numCPUs := float64(stats.CPUStats.OnlineCPUs)
		if numCPUs == 0 {
			numCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
		}
		if numCPUs == 0 {
			numCPUs = 1
		}
		result.CPUPercent = (cpuDelta / systemDelta) * numCPUs * 100.0
		result.CPUCores = int(numCPUs)
	}

	// Memory stats
	result.MemoryUsage = stats.MemoryStats.Usage
	result.MemoryLimit = stats.MemoryStats.Limit
	if result.MemoryLimit > 0 {
		result.MemoryPercent = (float64(result.MemoryUsage) / float64(result.MemoryLimit)) * 100.0
	}

	// Network I/O (aggregate all interfaces)
	for _, netStats := range stats.Networks {
		result.NetRxBytes += netStats.RxBytes
		result.NetTxBytes += netStats.TxBytes
	}

	// Block I/O
	for _, bioEntry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch bioEntry.Op {
		case "read", "Read":
			result.BlockRead += bioEntry.Value
		case "write", "Write":
			result.BlockWrite += bioEntry.Value
		}
	}

	return result
}
