package timeseries

import (
	"fmt"
	"log"
	"sync"
)

// Service provides time-series data storage and retrieval
type Service struct {
	store      *Store
	aggregator *Aggregator
	cleaner    *Cleaner
	enabled    bool
	mu         sync.RWMutex
}

// Config holds configuration for the timeseries service
type Config struct {
	Enabled   bool
	Path      string
	Retention RetentionConfig
}

// DefaultConfig returns default timeseries configuration
func DefaultConfig() Config {
	return Config{
		Enabled:   true,
		Path:      "./fleetctrl.db",
		Retention: DefaultRetentionConfig(),
	}
}

// NewService creates a new timeseries service
func NewService(cfg Config) (*Service, error) {
	if !cfg.Enabled {
		log.Println("[timeseries] Service disabled")
		return &Service{enabled: false}, nil
	}

	store, err := NewStore(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	aggregator := NewAggregator(store)
	cleaner := NewCleaner(store, cfg.Retention)

	svc := &Service{
		store:      store,
		aggregator: aggregator,
		cleaner:    cleaner,
		enabled:    true,
	}

	cleaner.Start()

	log.Printf("[timeseries] Service initialized with database at %s", cfg.Path)
	return svc, nil
}

// Close shuts down the service
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return nil
	}

	if s.cleaner != nil {
		s.cleaner.Stop()
	}

	if s.store != nil {
		return s.store.Close()
	}

	return nil
}

// IsEnabled returns whether the service is enabled
func (s *Service) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// WriteHostStats records host statistics
func (s *Service) WriteHostStats(hostID string, stats *HostStatsPoint) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		return nil
	}

	return s.store.WriteHostStats(hostID, stats)
}

// WriteHostOnlineStatus records host online/offline status
func (s *Service) WriteHostOnlineStatus(hostID string, online bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		return nil
	}

	point := &HostStatusPoint{
		Timestamp: NowUnix(),
		Online:    online,
	}

	return s.store.WriteHostStatus(hostID, point)
}

// GetHostStatusTimeline retrieves host status timeline for visualization
func (s *Service) GetHostStatusTimeline(hostID string, points int, intervalSeconds int64) ([]HostStatusHistoryPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		timeline := make([]HostStatusHistoryPoint, points)
		now := NowUnix()
		for i := 0; i < points; i++ {
			timeline[i] = HostStatusHistoryPoint{
				Timestamp: now - int64(points-1-i)*intervalSeconds,
				Online:    false,
			}
		}
		return timeline, nil
	}

	return s.aggregator.BuildTimelineFromStatusHistory(hostID, points, intervalSeconds)
}

// GetHostUptime calculates uptime percentage for a host
func (s *Service) GetHostUptime(hostID string, points int, intervalSeconds int64) (float64, error) {
	timeline, err := s.GetHostStatusTimeline(hostID, points, intervalSeconds)
	if err != nil {
		return 0, err
	}
	return CalculateUptime(timeline), nil
}

// GetStorageStats returns storage statistics
func (s *Service) GetStorageStats() (*StorageStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		return &StorageStats{
			DatabasePath:  "",
			DatabaseSize:  0,
			EstimatedSize: "Disabled",
		}, nil
	}

	dbSize, err := s.store.GetDatabaseSize()
	if err != nil {
		return nil, err
	}

	bucketStats, err := s.store.GetBucketStats()
	if err != nil {
		return nil, err
	}

	stats := &StorageStats{
		DatabasePath:  s.store.path,
		DatabaseSize:  dbSize,
		HostStats:     bucketStats["host_stats_raw"],
		StatusHistory: bucketStats["host_status"],
		EstimatedSize: FormatStorageSize(dbSize),
	}

	return stats, nil
}

// IsHostOnline returns the current online status for a host
func (s *Service) IsHostOnline(hostID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		return false, nil
	}

	point, err := s.store.GetLatestHostStatus(hostID)
	if err != nil {
		return false, err
	}

	if point == nil {
		return false, nil
	}

	return point.Online, nil
}

// BackfillHostOfflineStatus writes offline status entries for a time range
func (s *Service) BackfillHostOfflineStatus(hostID string, fromTS, toTS, intervalSeconds int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return 0, nil
	}

	count := 0
	for ts := fromTS; ts < toTS; ts += intervalSeconds {
		point := &HostStatusPoint{
			Timestamp: ts,
			Online:    false,
		}
		if err := s.store.WriteHostStatus(hostID, point); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
