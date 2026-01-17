package timeseries

import (
	"encoding/json"
	"time"
)

// HostStatsPoint represents a single point of host statistics
type HostStatsPoint struct {
	Timestamp   int64   `json:"timestamp"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryUsed  uint64  `json:"memory_used"`
	MemoryTotal uint64  `json:"memory_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskTotal   uint64  `json:"disk_total"`
	NetworkIn   uint64  `json:"network_in"`
	NetworkOut  uint64  `json:"network_out"`
}

// HostStatsAgg represents aggregated host statistics over a period
type HostStatsAgg struct {
	Timestamp       int64   `json:"timestamp"`
	CPUAvg          float64 `json:"cpu_avg"`
	CPUMax          float64 `json:"cpu_max"`
	MemoryAvg       uint64  `json:"memory_avg"`
	MemoryMax       uint64  `json:"memory_max"`
	MemoryTotal     uint64  `json:"memory_total"`
	DiskAvg         uint64  `json:"disk_avg"`
	DiskMax         uint64  `json:"disk_max"`
	DiskTotal       uint64  `json:"disk_total"`
	NetworkInTotal  uint64  `json:"network_in_total"`
	NetworkOutTotal uint64  `json:"network_out_total"`
	PointCount      int     `json:"point_count"`
}

// HostStatusPoint represents host online/offline status at a point in time
type HostStatusPoint struct {
	Timestamp int64 `json:"timestamp"`
	Online    bool  `json:"online"`
}

// HostStatusHistoryPoint represents a point in the status timeline
type HostStatusHistoryPoint struct {
	Timestamp int64 `json:"timestamp"`
	Online    bool  `json:"online"`
}

// StorageStats represents database storage statistics
type StorageStats struct {
	DatabasePath   string `json:"database_path"`
	DatabaseSize   int64  `json:"database_size"`
	HostStats      int64  `json:"host_stats_count"`
	StatusHistory  int64  `json:"status_history_count"`
	EstimatedSize  string `json:"estimated_size"`
}

// Encode serializes a point to JSON bytes
func (p *HostStatsPoint) Encode() ([]byte, error) {
	return json.Marshal(p)
}

// DecodeHostStatsPoint deserializes JSON bytes to a HostStatsPoint
func DecodeHostStatsPoint(data []byte) (*HostStatsPoint, error) {
	var p HostStatsPoint
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Encode serializes an aggregate to JSON bytes
func (a *HostStatsAgg) Encode() ([]byte, error) {
	return json.Marshal(a)
}

// DecodeHostStatsAgg deserializes JSON bytes to a HostStatsAgg
func DecodeHostStatsAgg(data []byte) (*HostStatsAgg, error) {
	var a HostStatsAgg
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Encode serializes a host status point to JSON bytes
func (p *HostStatusPoint) Encode() ([]byte, error) {
	return json.Marshal(p)
}

// DecodeHostStatusPoint deserializes JSON bytes to a HostStatusPoint
func DecodeHostStatusPoint(data []byte) (*HostStatusPoint, error) {
	var p HostStatusPoint
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// TimestampKey generates a key for time-series storage
func TimestampKey(ts int64) []byte {
	key := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		key[i] = byte(ts)
		ts >>= 8
	}
	return key
}

// ParseTimestampKey extracts timestamp from a key
func ParseTimestampKey(key []byte) int64 {
	if len(key) < 8 {
		return 0
	}
	var ts int64
	for i := 0; i < 8; i++ {
		ts = (ts << 8) | int64(key[i])
	}
	return ts
}

// NowUnix returns current Unix timestamp in seconds
func NowUnix() int64 {
	return time.Now().Unix()
}

// TruncateToMinute truncates timestamp to the start of the minute
func TruncateToMinute(ts int64) int64 {
	return ts - (ts % 60)
}

// TruncateToHour truncates timestamp to the start of the hour
func TruncateToHour(ts int64) int64 {
	return ts - (ts % 3600)
}

// TruncateToDay truncates timestamp to the start of the day (UTC)
func TruncateToDay(ts int64) int64 {
	return ts - (ts % 86400)
}
