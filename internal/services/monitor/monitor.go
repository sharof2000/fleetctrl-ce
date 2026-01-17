package monitor

import (
	"strings"
	"sync"
	"time"

	"fleetctrl/internal/models"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// Service handles system monitoring
type Service struct {
	stats    *models.SystemStats
	mu       sync.RWMutex
	stopChan chan struct{}
	interval time.Duration
}

// NewService creates a new monitor service
func NewService() *Service {
	return &Service{
		stats:    &models.SystemStats{},
		stopChan: make(chan struct{}),
		interval: 5 * time.Second,
	}
}

// Start begins the background stats collection
func (s *Service) Start() {
	go s.collectLoop()
}

// Stop stops the background stats collection
func (s *Service) Stop() {
	close(s.stopChan)
}

// GetStats returns the cached system stats
func (s *Service) GetStats() (*models.SystemStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	stats := *s.stats
	return &stats, nil
}

func (s *Service) collectLoop() {
	// Collect immediately on start
	s.collect()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.collect()
		case <-s.stopChan:
			return
		}
	}
}

func (s *Service) collect() {
	stats := &models.SystemStats{}

	// Host info
	if hostInfo, err := host.Info(); err == nil {
		stats.Hostname = hostInfo.Hostname
		stats.OS = hostInfo.OS
		stats.Arch = hostInfo.KernelArch
		stats.UptimeSeconds = hostInfo.Uptime
		stats.BootTime = time.Unix(int64(hostInfo.BootTime), 0).UTC().Format(time.RFC3339)
	}

	// CPU - detailed stats
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		stats.CPU.Percent = cpuPercent[0]
	}
	if cpuInfo, err := cpu.Info(); err == nil && len(cpuInfo) > 0 {
		stats.CPU.Model = cpuInfo[0].ModelName
		stats.CPU.Cores = int(cpuInfo[0].Cores)
		stats.CPU.SpeedMHz = cpuInfo[0].Mhz
		stats.CPU.SpeedGHz = cpuInfo[0].Mhz / 1000.0
	}
	// Physical cores
	if physicalCores, err := cpu.Counts(false); err == nil {
		stats.CPU.Cores = physicalCores
	}
	// Logical cores (with hyperthreading)
	if logicalCores, err := cpu.Counts(true); err == nil {
		stats.CPU.LogicalCores = logicalCores
	}

	// Memory - detailed stats
	if memInfo, err := mem.VirtualMemory(); err == nil {
		usedBytes := memInfo.Total - memInfo.Available
		stats.Memory.TotalBytes = memInfo.Total
		stats.Memory.UsedBytes = usedBytes
		stats.Memory.FreeBytes = memInfo.Available
		stats.Memory.Percent = float64(usedBytes) / float64(memInfo.Total) * 100
		stats.Memory.TotalGB = float64(memInfo.Total) / (1024 * 1024 * 1024)
		stats.Memory.UsedGB = float64(usedBytes) / (1024 * 1024 * 1024)
		stats.Memory.FreeGB = float64(memInfo.Available) / (1024 * 1024 * 1024)
	}

	// Disks - all partitions
	stats.Disks = s.collectDisks()

	// Network - all interfaces
	stats.Network = s.collectNetwork()

	s.mu.Lock()
	s.stats = stats
	s.mu.Unlock()
}

// collectDisks collects stats for all disk partitions
func (s *Service) collectDisks() []models.DiskStats {
	var disks []models.DiskStats

	partitions, err := disk.Partitions(false)
	if err != nil {
		return disks
	}

	seen := make(map[string]bool)

	for _, p := range partitions {
		if s.shouldSkipPartition(p) {
			continue
		}

		if seen[p.Mountpoint] {
			continue
		}
		seen[p.Mountpoint] = true

		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		if usage.Total == 0 {
			continue
		}

		disks = append(disks, models.DiskStats{
			Path:       p.Mountpoint,
			Device:     p.Device,
			Fstype:     p.Fstype,
			TotalBytes: usage.Total,
			UsedBytes:  usage.Used,
			FreeBytes:  usage.Free,
			Percent:    usage.UsedPercent,
			TotalGB:    float64(usage.Total) / (1024 * 1024 * 1024),
			UsedGB:     float64(usage.Used) / (1024 * 1024 * 1024),
			FreeGB:     float64(usage.Free) / (1024 * 1024 * 1024),
		})
	}

	return disks
}

// shouldSkipPartition returns true if the partition should be skipped
func (s *Service) shouldSkipPartition(p disk.PartitionStat) bool {
	skipFsTypes := []string{"squashfs", "tmpfs", "devtmpfs", "overlay", "aufs", "proc", "sysfs", "cgroup", "cgroup2", "securityfs", "pstore", "debugfs", "configfs", "fusectl", "mqueue", "hugetlbfs", "binfmt_misc", "autofs", "efivarfs", "bpf", "tracefs"}
	for _, skip := range skipFsTypes {
		if p.Fstype == skip {
			return true
		}
	}
	return false
}

// collectNetwork collects network I/O statistics
func (s *Service) collectNetwork() models.NetworkStats {
	netStats := models.NetworkStats{
		Interfaces: []models.NetInterfaceStats{},
	}

	counters, err := net.IOCounters(true)
	if err != nil {
		return netStats
	}

	var totalSent, totalRecv uint64

	for _, c := range counters {
		if s.shouldSkipInterface(c.Name) {
			continue
		}

		if c.BytesSent == 0 && c.BytesRecv == 0 {
			continue
		}

		iface := models.NetInterfaceStats{
			Name:      c.Name,
			BytesSent: c.BytesSent,
			BytesRecv: c.BytesRecv,
			SentGB:    float64(c.BytesSent) / (1024 * 1024 * 1024),
			RecvGB:    float64(c.BytesRecv) / (1024 * 1024 * 1024),
		}

		netStats.Interfaces = append(netStats.Interfaces, iface)
		totalSent += c.BytesSent
		totalRecv += c.BytesRecv
	}

	netStats.TotalBytesSent = totalSent
	netStats.TotalBytesRecv = totalRecv
	netStats.TotalSentGB = float64(totalSent) / (1024 * 1024 * 1024)
	netStats.TotalRecvGB = float64(totalRecv) / (1024 * 1024 * 1024)

	return netStats
}

// shouldSkipInterface returns true if the interface should be skipped
func (s *Service) shouldSkipInterface(name string) bool {
	if name == "lo" || strings.HasPrefix(name, "Loopback") {
		return true
	}

	skipPrefixes := []string{"docker", "br-", "veth", "virbr", "vnet"}
	nameLower := strings.ToLower(name)
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(nameLower, prefix) {
			return true
		}
	}

	return false
}
