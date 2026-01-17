package timeseries

// Aggregator handles aggregation of raw time-series data
type Aggregator struct {
	store *Store
}

// NewAggregator creates a new aggregator
func NewAggregator(store *Store) *Aggregator {
	return &Aggregator{store: store}
}

// AggregateHostStatsToMinute aggregates raw host stats to minute-level
func (a *Aggregator) AggregateHostStatsToMinute(hostID string, minuteTS int64) (*HostStatsAgg, error) {
	startTS := minuteTS
	endTS := minuteTS + 59

	points, err := a.store.GetHostStatsRaw(hostID, startTS, endTS)
	if err != nil {
		return nil, err
	}

	if len(points) == 0 {
		return nil, nil
	}

	return aggregateHostStatsPoints(points, minuteTS), nil
}

// AggregateHostStatsToHour aggregates minute stats to hour-level
func (a *Aggregator) AggregateHostStatsToHour(hostID string, hourTS int64) (*HostStatsAgg, error) {
	startTS := hourTS
	endTS := hourTS + 3599

	points, err := a.store.GetHostStatsRaw(hostID, startTS, endTS)
	if err != nil {
		return nil, err
	}

	if len(points) == 0 {
		return nil, nil
	}

	return aggregateHostStatsPoints(points, hourTS), nil
}

// AggregateHostStatsToDay aggregates hour stats to day-level
func (a *Aggregator) AggregateHostStatsToDay(hostID string, dayTS int64) (*HostStatsAgg, error) {
	startTS := dayTS
	endTS := dayTS + 86399

	points, err := a.store.GetHostStatsRaw(hostID, startTS, endTS)
	if err != nil {
		return nil, err
	}

	if len(points) == 0 {
		return nil, nil
	}

	return aggregateHostStatsPoints(points, dayTS), nil
}

// aggregateHostStatsPoints creates an aggregate from multiple points
func aggregateHostStatsPoints(points []*HostStatsPoint, timestamp int64) *HostStatsAgg {
	if len(points) == 0 {
		return nil
	}

	agg := &HostStatsAgg{
		Timestamp:  timestamp,
		PointCount: len(points),
	}

	var cpuSum float64
	var memSum, diskSum uint64
	var networkInSum, networkOutSum uint64

	for _, p := range points {
		cpuSum += p.CPUPercent
		memSum += p.MemoryUsed
		diskSum += p.DiskUsed
		networkInSum += p.NetworkIn
		networkOutSum += p.NetworkOut

		if p.CPUPercent > agg.CPUMax {
			agg.CPUMax = p.CPUPercent
		}
		if p.MemoryUsed > agg.MemoryMax {
			agg.MemoryMax = p.MemoryUsed
		}
		if p.DiskUsed > agg.DiskMax {
			agg.DiskMax = p.DiskUsed
		}

		agg.MemoryTotal = p.MemoryTotal
		agg.DiskTotal = p.DiskTotal
	}

	n := len(points)
	agg.CPUAvg = cpuSum / float64(n)
	agg.MemoryAvg = memSum / uint64(n)
	agg.DiskAvg = diskSum / uint64(n)
	agg.NetworkInTotal = networkInSum
	agg.NetworkOutTotal = networkOutSum

	return agg
}

// BuildTimelineFromStatusHistory builds a timeline with specified points and interval
func (a *Aggregator) BuildTimelineFromStatusHistory(hostID string, points int, intervalSeconds int64) ([]HostStatusHistoryPoint, error) {
	now := NowUnix()
	timeline := make([]HostStatusHistoryPoint, points)

	for i := 0; i < points; i++ {
		timeline[i] = HostStatusHistoryPoint{
			Timestamp: now - int64(points-1-i)*intervalSeconds,
			Online:    false,
		}
	}

	startTS := now - int64(points)*intervalSeconds
	endTS := now

	history, err := a.store.GetHostStatusHistory(hostID, startTS, endTS)
	if err != nil {
		return timeline, err
	}

	if len(history) == 0 {
		return timeline, nil
	}

	historyIdx := 0
	for i := 0; i < points; i++ {
		pointTS := timeline[i].Timestamp

		for historyIdx < len(history) && history[historyIdx].Timestamp <= pointTS {
			historyIdx++
		}

		if historyIdx > 0 {
			timeline[i].Online = history[historyIdx-1].Online
		}
	}

	return timeline, nil
}

// CalculateUptime calculates uptime percentage from timeline
func CalculateUptime(timeline []HostStatusHistoryPoint) float64 {
	if len(timeline) == 0 {
		return 0
	}

	online := 0
	for _, p := range timeline {
		if p.Online {
			online++
		}
	}

	return float64(online) / float64(len(timeline)) * 100
}
