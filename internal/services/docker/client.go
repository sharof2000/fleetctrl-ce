package docker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/docker/docker/client"
)

// ErrDockerNotAvailable is returned when Docker is not available
var ErrDockerNotAvailable = errors.New("Docker is not available")

// Service handles Docker operations
type Service struct {
	client    *client.Client
	ctx       context.Context
	available bool
	mu        sync.RWMutex
	stopChan  chan struct{}
}

// NewService creates a new Docker service
// Returns a service even if Docker is not available, with available=false
func NewService() (*Service, error) {
	ctx := context.Background()

	s := &Service{
		client:    nil,
		ctx:       ctx,
		available: false,
		stopChan:  make(chan struct{}),
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return s, err
	}

	// Test connection
	_, err = cli.Ping(ctx)
	if err != nil {
		return s, err
	}

	s.client = cli
	s.available = true
	return s, nil
}

// Reconnect attempts to reconnect to Docker
// Returns true if reconnection was successful
func (s *Service) Reconnect() bool {
	if s == nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Close existing client if any
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		s.available = false
		return false
	}

	// Test connection
	_, err = cli.Ping(s.ctx)
	if err != nil {
		cli.Close()
		s.available = false
		return false
	}

	s.client = cli
	s.available = true
	log.Println("[docker] Successfully reconnected to Docker")
	return true
}

// StartAutoReconnect starts a background goroutine that periodically attempts to reconnect
func (s *Service) StartAutoReconnect(interval time.Duration) {
	if s == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				s.mu.RLock()
				isAvailable := s.available
				s.mu.RUnlock()

				if !isAvailable {
					if s.Reconnect() {
						log.Println("[docker] Auto-reconnect successful")
					}
				}
			}
		}
	}()
}

// StopAutoReconnect stops the auto-reconnect goroutine
func (s *Service) StopAutoReconnect() {
	if s == nil || s.stopChan == nil {
		return
	}
	close(s.stopChan)
}

// Close closes the Docker client
func (s *Service) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// IsAvailable checks if Docker is available
func (s *Service) IsAvailable() bool {
	if s == nil {
		return false
	}

	s.mu.RLock()
	cli := s.client
	s.mu.RUnlock()

	if cli == nil {
		return false
	}

	_, err := cli.Ping(s.ctx)
	return err == nil
}

// checkAvailable returns an error if Docker is not available
func (s *Service) checkAvailable() error {
	if s == nil {
		return ErrDockerNotAvailable
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.available || s.client == nil {
		return ErrDockerNotAvailable
	}
	return nil
}

// getClient safely returns the Docker client
func (s *Service) getClient() *client.Client {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

// GetClient returns the Docker client for external use
func (s *Service) GetClient() *client.Client {
	return s.getClient()
}
