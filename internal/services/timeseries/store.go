package timeseries

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// Bucket names
var (
	bucketHostStatsRaw    = []byte("host_stats_raw")
	bucketHostStatsMinute = []byte("host_stats_minute")
	bucketHostStatsHour   = []byte("host_stats_hour")
	bucketHostStatsDay    = []byte("host_stats_day")
	bucketHostStatus      = []byte("host_status")
)

// Store provides bbolt-based time-series storage
type Store struct {
	db   *bolt.DB
	path string
	mu   sync.RWMutex
}

// NewStore creates a new time-series store
func NewStore(path string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database with default options
	db, err := bolt.Open(path, 0600, &bolt.Options{
		Timeout: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{
		db:   db,
		path: path,
	}

	// Initialize buckets
	if err := store.initBuckets(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initBuckets creates all required buckets
func (s *Store) initBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketHostStatsRaw,
			bucketHostStatsMinute,
			bucketHostStatsHour,
			bucketHostStatsDay,
			bucketHostStatus,
		}

		for _, name := range buckets {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", name, err)
			}
		}
		return nil
	})
}

// Close closes the database
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// makeKey creates a composite key from host and timestamp
func makeKey(hostID string, ts int64) []byte {
	prefix := fmt.Sprintf("%s:", hostID)
	key := append([]byte(prefix), TimestampKey(ts)...)
	return key
}

// WriteHostStats writes a host stats point to the raw bucket
func (s *Store) WriteHostStats(hostID string, point *HostStatsPoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := point.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode host stats: %w", err)
	}

	key := makeKey(hostID, point.Timestamp)

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHostStatsRaw)
		return b.Put(key, data)
	})
}

// WriteHostStatsAgg writes aggregated host stats to the appropriate bucket
func (s *Store) WriteHostStatsAgg(hostID string, agg *HostStatsAgg, level string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var bucketName []byte
	switch level {
	case "minute":
		bucketName = bucketHostStatsMinute
	case "hour":
		bucketName = bucketHostStatsHour
	case "day":
		bucketName = bucketHostStatsDay
	default:
		return fmt.Errorf("invalid aggregation level: %s", level)
	}

	data, err := agg.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode host stats agg: %w", err)
	}

	key := makeKey(hostID, agg.Timestamp)

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Put(key, data)
	})
}

// WriteHostStatus writes a host online/offline status point
func (s *Store) WriteHostStatus(hostID string, point *HostStatusPoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := point.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode host status: %w", err)
	}

	key := makeKey(hostID, point.Timestamp)

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHostStatus)
		return b.Put(key, data)
	})
}

// GetHostStatsRaw retrieves raw host stats within a time range
func (s *Store) GetHostStatsRaw(hostID string, startTS, endTS int64) ([]*HostStatsPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var points []*HostStatsPoint
	prefix := []byte(fmt.Sprintf("%s:", hostID))
	startKey := makeKey(hostID, startTS)
	endKey := makeKey(hostID, endTS)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHostStatsRaw)
		c := b.Cursor()

		for k, v := c.Seek(startKey); k != nil; k, v = c.Next() {
			if len(k) < len(prefix) {
				continue
			}
			if string(k[:len(prefix)]) != string(prefix) {
				break
			}
			if compareKeys(k, endKey) > 0 {
				break
			}

			point, err := DecodeHostStatsPoint(v)
			if err != nil {
				continue
			}
			points = append(points, point)
		}
		return nil
	})

	return points, err
}

// GetHostStatusHistory retrieves host status history for timeline
func (s *Store) GetHostStatusHistory(hostID string, startTS, endTS int64) ([]*HostStatusPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var points []*HostStatusPoint
	prefix := []byte(fmt.Sprintf("%s:", hostID))
	startKey := makeKey(hostID, startTS)
	endKey := makeKey(hostID, endTS)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHostStatus)
		c := b.Cursor()

		for k, v := c.Seek(startKey); k != nil; k, v = c.Next() {
			if len(k) < len(prefix) {
				continue
			}
			if string(k[:len(prefix)]) != string(prefix) {
				break
			}
			if compareKeys(k, endKey) > 0 {
				break
			}

			point, err := DecodeHostStatusPoint(v)
			if err != nil {
				continue
			}
			points = append(points, point)
		}
		return nil
	})

	return points, err
}

// GetLatestHostStatus gets the most recent status for a host
func (s *Store) GetLatestHostStatus(hostID string) (*HostStatusPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latestPoint *HostStatusPoint
	prefix := []byte(fmt.Sprintf("%s:", hostID))

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHostStatus)
		c := b.Cursor()

		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			if len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix) {
				point, err := DecodeHostStatusPoint(v)
				if err != nil {
					continue
				}
				latestPoint = point
				return nil
			}
		}
		return nil
	})

	return latestPoint, err
}

// DeleteOlderThan deletes entries older than the given timestamp from a bucket
func (s *Store) DeleteOlderThan(bucketName []byte, olderThan int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil
		}

		c := b.Cursor()
		var toDelete [][]byte

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if len(k) < 8 {
				continue
			}
			ts := ParseTimestampKey(k[len(k)-8:])
			if ts < olderThan {
				toDelete = append(toDelete, append([]byte(nil), k...))
			}
		}

		for _, key := range toDelete {
			if err := b.Delete(key); err != nil {
				return err
			}
			deleted++
		}

		return nil
	})

	return deleted, err
}

// GetBucketStats returns statistics for each bucket
func (s *Store) GetBucketStats() (map[string]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]int64)

	err := s.db.View(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketHostStatsRaw,
			bucketHostStatsMinute,
			bucketHostStatsHour,
			bucketHostStatsDay,
			bucketHostStatus,
		}

		for _, name := range buckets {
			b := tx.Bucket(name)
			if b != nil {
				bStats := b.Stats()
				stats[string(name)] = int64(bStats.KeyN)
			}
		}
		return nil
	})

	return stats, err
}

// GetDatabaseSize returns the current database file size in bytes
func (s *Store) GetDatabaseSize() (int64, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// compareKeys compares two byte slices lexicographically
func compareKeys(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}
