package hosts

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"fleetctrl/internal/config"
	"fleetctrl/internal/models"
	"fleetctrl/internal/services/timeseries"
)

// HostState represents the current state of a host
type HostState struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Address        string            `json:"address"`
	Status         string            `json:"status"` // online, offline, unknown
	LastSeen       time.Time         `json:"last_seen"`
	LastError      string            `json:"last_error,omitempty"`
	Stats          *models.HostStats `json:"stats,omitempty"`
	ActualHostname string            `json:"actual_hostname,omitempty"`
}

// Service manages host tracking and health checks
type Service struct {
	config            *config.Config
	timeseriesService *timeseries.Service
	httpClient        *http.Client
	hosts             map[string]*HostState
	localHostID       string
	mu                sync.RWMutex
	stopChan          chan struct{}
	healthInterval    time.Duration
}

// NewService creates a new hosts service
func NewService(cfg *config.Config, ts *timeseries.Service) *Service {
	return &Service{
		config:            cfg,
		timeseriesService: ts,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		hosts:          make(map[string]*HostState),
		stopChan:       make(chan struct{}),
		healthInterval: 10 * time.Second,
	}
}

// Start initializes hosts and starts health check loop
func (s *Service) Start() {
	s.mu.Lock()
	// Initialize hosts from config
	for _, h := range s.config.Hosts {
		s.hosts[h.ID] = &HostState{
			ID:      h.ID,
			Name:    h.Name,
			Address: h.Address,
			Status:  "unknown",
		}
	}
	s.mu.Unlock()

	go s.healthCheckLoop()
	log.Printf("[hosts] Service started with %d hosts", len(s.config.Hosts))
}

// Stop stops the health check loop
func (s *Service) Stop() {
	close(s.stopChan)
}

// SetLocalHostID sets the local host identifier
func (s *Service) SetLocalHostID(id string) {
	s.mu.Lock()
	s.localHostID = id
	s.mu.Unlock()
}

// GetHosts returns all hosts with their current status
func (s *Service) GetHosts() []models.HostWithStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []models.HostWithStatus

	for _, h := range s.config.Hosts {
		state, exists := s.hosts[h.ID]
		if !exists {
			state = &HostState{
				ID:      h.ID,
				Name:    h.Name,
				Address: h.Address,
				Status:  "unknown",
			}
		}

		host := models.HostWithStatus{
			ID:             h.ID,
			Name:           h.Name,
			Address:        h.Address,
			Status:         state.Status,
			ActualHostname: state.ActualHostname,
			IsSelf:         h.ID == s.localHostID,
			Stats:          state.Stats,
		}

		if !state.LastSeen.IsZero() {
			host.LastSeen = state.LastSeen.Format(time.RFC3339)
		}

		result = append(result, host)
	}

	return result
}

// GetHost returns a single host with status
func (s *Service) GetHost(id string) (*models.HostWithStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hostConfig := s.config.GetHostByID(id)
	if hostConfig == nil {
		return nil, fmt.Errorf("host not found: %s", id)
	}

	state, exists := s.hosts[id]
	if !exists {
		state = &HostState{
			ID:      hostConfig.ID,
			Name:    hostConfig.Name,
			Address: hostConfig.Address,
			Status:  "unknown",
		}
	}

	host := &models.HostWithStatus{
		ID:             hostConfig.ID,
		Name:           hostConfig.Name,
		Address:        hostConfig.Address,
		Status:         state.Status,
		ActualHostname: state.ActualHostname,
		IsSelf:         hostConfig.ID == s.localHostID,
		Stats:          state.Stats,
	}

	if !state.LastSeen.IsZero() {
		host.LastSeen = state.LastSeen.Format(time.RFC3339)
	}

	return host, nil
}

// AddHost adds a new host
func (s *Service) AddHost(name, address string) (*config.HostConfig, error) {
	host := config.HostConfig{
		Name:    name,
		Address: address,
	}

	if err := s.config.AddHost(host); err != nil {
		return nil, err
	}

	if err := s.config.Save(); err != nil {
		return nil, err
	}

	// Get the added host (with generated ID)
	for _, h := range s.config.Hosts {
		if h.Address == address {
			s.mu.Lock()
			s.hosts[h.ID] = &HostState{
				ID:      h.ID,
				Name:    h.Name,
				Address: h.Address,
				Status:  "unknown",
			}
			s.mu.Unlock()
			return &h, nil
		}
	}

	return nil, fmt.Errorf("failed to find added host")
}

// RemoveHost removes a host
func (s *Service) RemoveHost(id string) error {
	if err := s.config.RemoveHost(id); err != nil {
		return err
	}

	if err := s.config.Save(); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.hosts, id)
	s.mu.Unlock()

	return nil
}

// RefreshHost manually refreshes a host's status
func (s *Service) RefreshHost(id string) error {
	hostConfig := s.config.GetHostByID(id)
	if hostConfig == nil {
		return fmt.Errorf("host not found: %s", id)
	}

	s.checkHost(hostConfig)
	return nil
}

// healthCheckLoop periodically checks all hosts
func (s *Service) healthCheckLoop() {
	// Initial check
	s.checkAllHosts()

	ticker := time.NewTicker(s.healthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAllHosts()
		case <-s.stopChan:
			return
		}
	}
}

// checkAllHosts checks all configured hosts
func (s *Service) checkAllHosts() {
	for _, h := range s.config.Hosts {
		// Skip local host
		if h.ID == s.localHostID {
			s.updateHostStatus(h.ID, "online", nil, nil, "")
			continue
		}
		s.checkHost(&h)
	}
}

// checkHost checks a single host's health and fetches stats
func (s *Service) checkHost(h *config.HostConfig) {
	url := fmt.Sprintf("http://%s/api/peer/stats", h.Address)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		s.updateHostStatus(h.ID, "offline", nil, err, "")
		return
	}

	// Use host's token for authentication
	if h.Token != "" {
		req.Header.Set("X-Host-Token", h.Token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.updateHostStatus(h.ID, "offline", nil, err, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.updateHostStatus(h.ID, "offline", nil, fmt.Errorf("status %d", resp.StatusCode), "")
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.updateHostStatus(h.ID, "offline", nil, err, "")
		return
	}

	var stats models.HostStats
	if err := json.Unmarshal(body, &stats); err != nil {
		s.updateHostStatus(h.ID, "offline", nil, err, "")
		return
	}

	s.updateHostStatus(h.ID, "online", &stats, nil, stats.Hostname)
}

// updateHostStatus updates a host's status
func (s *Service) updateHostStatus(id, status string, stats *models.HostStats, err error, actualHostname string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.hosts[id]
	if !exists {
		state = &HostState{ID: id}
		s.hosts[id] = state
	}

	previousStatus := state.Status
	state.Status = status
	state.Stats = stats
	state.ActualHostname = actualHostname

	if status == "online" {
		state.LastSeen = time.Now()
		state.LastError = ""
	} else if err != nil {
		state.LastError = err.Error()
	}

	// Record status change in timeseries
	if s.timeseriesService != nil && previousStatus != status {
		online := status == "online"
		if tsErr := s.timeseriesService.WriteHostOnlineStatus(id, online); tsErr != nil {
			log.Printf("[hosts] Failed to write status to timeseries: %v", tsErr)
		}
	}
}

// TestConnection tests connection to a host address
func (s *Service) TestConnection(address string) (string, error) {
	url := fmt.Sprintf("http://%s/health", address)

	resp, err := s.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("connection failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var result struct {
		Status   string `json:"status"`
		Hostname string `json:"hostname"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("invalid response: %v", err)
	}

	return result.Hostname, nil
}

// GetHostTimeline returns the status timeline for a host
func (s *Service) GetHostTimeline(id string) ([]timeseries.HostStatusHistoryPoint, error) {
	if s.timeseriesService == nil {
		return nil, fmt.Errorf("timeseries service not available")
	}

	points := s.config.Dashboard.TimelinePoints
	interval := int64(s.config.Dashboard.TimelineInterval)

	return s.timeseriesService.GetHostStatusTimeline(id, points, interval)
}
