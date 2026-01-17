package timeseries

import (
	"log"
	"sync"
	"time"
)

// RetentionConfig holds retention settings for different data levels
type RetentionConfig struct {
	RawSeconds       int64 // How long to keep raw data (default: 1 hour)
	MinuteAggSeconds int64 // How long to keep minute aggregates (default: 24 hours)
	HourAggSeconds   int64 // How long to keep hour aggregates (default: 7 days)
	DayAggSeconds    int64 // How long to keep day aggregates (default: 30 days)
}

// DefaultRetentionConfig returns default retention settings
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		RawSeconds:       3600,    // 1 hour
		MinuteAggSeconds: 86400,   // 24 hours
		HourAggSeconds:   604800,  // 7 days
		DayAggSeconds:    2592000, // 30 days
	}
}

// Cleaner handles cleanup of old time-series data
type Cleaner struct {
	store     *Store
	retention RetentionConfig
	interval  time.Duration
	stopCh    chan struct{}
	wg        sync.WaitGroup
	running   bool
	mu        sync.Mutex
}

// NewCleaner creates a new cleaner with the given retention config
func NewCleaner(store *Store, retention RetentionConfig) *Cleaner {
	return &Cleaner{
		store:     store,
		retention: retention,
		interval:  time.Minute,
		stopCh:    make(chan struct{}),
	}
}

// Start starts the cleanup goroutine
func (c *Cleaner) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	c.wg.Add(1)
	go c.cleanupLoop()
}

// Stop stops the cleanup goroutine
func (c *Cleaner) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	c.mu.Unlock()

	close(c.stopCh)
	c.wg.Wait()
}

// cleanupLoop runs the cleanup process periodically
func (c *Cleaner) cleanupLoop() {
	defer c.wg.Done()

	c.runCleanup()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.runCleanup()
		}
	}
}

// runCleanup performs cleanup on all buckets
func (c *Cleaner) runCleanup() {
	now := NowUnix()

	if c.retention.RawSeconds > 0 {
		cutoff := now - c.retention.RawSeconds
		c.cleanupBucket(bucketHostStatsRaw, cutoff, "host_stats_raw")
		c.cleanupBucket(bucketHostStatus, cutoff, "host_status")
	}

	if c.retention.MinuteAggSeconds > 0 {
		cutoff := now - c.retention.MinuteAggSeconds
		c.cleanupBucket(bucketHostStatsMinute, cutoff, "host_stats_minute")
	}

	if c.retention.HourAggSeconds > 0 {
		cutoff := now - c.retention.HourAggSeconds
		c.cleanupBucket(bucketHostStatsHour, cutoff, "host_stats_hour")
	}

	if c.retention.DayAggSeconds > 0 {
		cutoff := now - c.retention.DayAggSeconds
		c.cleanupBucket(bucketHostStatsDay, cutoff, "host_stats_day")
	}
}

// cleanupBucket deletes old entries from a specific bucket
func (c *Cleaner) cleanupBucket(bucketName []byte, olderThan int64, label string) {
	deleted, err := c.store.DeleteOlderThan(bucketName, olderThan)
	if err != nil {
		log.Printf("[timeseries] Error cleaning %s: %v", label, err)
		return
	}
	if deleted > 0 {
		log.Printf("[timeseries] Cleaned %d old entries from %s", deleted, label)
	}
}

// FormatStorageSize formats bytes into a human-readable string
func FormatStorageSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return formatFloat(float64(bytes)/float64(GB)) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/float64(MB)) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/float64(KB)) + " KB"
	default:
		return formatFloat(float64(bytes)) + " B"
	}
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return intToString(int64(f))
	}
	intPart := int64(f)
	decPart := int64((f - float64(intPart)) * 10)
	if decPart == 0 {
		return intToString(intPart)
	}
	return intToString(intPart) + "." + intToString(decPart)
}

func intToString(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
