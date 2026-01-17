package models

// SystemStats represents system statistics
type SystemStats struct {
	Hostname      string       `json:"hostname"`
	OS            string       `json:"os"`
	Arch          string       `json:"arch"`
	CPU           CPUStats     `json:"cpu"`
	Memory        MemStats     `json:"memory"`
	Disks         []DiskStats  `json:"disks"`
	Network       NetworkStats `json:"network"`
	UptimeSeconds uint64       `json:"uptime_seconds"`
	BootTime      string       `json:"boot_time"`
}

// CPUStats represents CPU statistics
type CPUStats struct {
	Cores        int     `json:"cores"`
	LogicalCores int     `json:"logical_cores"`
	Percent      float64 `json:"percent"`
	Model        string  `json:"model"`
	SpeedMHz     float64 `json:"speed_mhz"`
	SpeedGHz     float64 `json:"speed_ghz"`
}

// MemStats represents memory statistics
type MemStats struct {
	TotalBytes uint64  `json:"total_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	FreeBytes  uint64  `json:"free_bytes"`
	Percent    float64 `json:"percent"`
	TotalGB    float64 `json:"total_gb"`
	UsedGB     float64 `json:"used_gb"`
	FreeGB     float64 `json:"free_gb"`
}

// DiskStats represents disk/partition statistics
type DiskStats struct {
	Path       string  `json:"path"`
	Device     string  `json:"device,omitempty"`
	Fstype     string  `json:"fstype,omitempty"`
	TotalBytes uint64  `json:"total_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	FreeBytes  uint64  `json:"free_bytes"`
	Percent    float64 `json:"percent"`
	TotalGB    float64 `json:"total_gb"`
	UsedGB     float64 `json:"used_gb"`
	FreeGB     float64 `json:"free_gb"`
}

// NetworkStats represents network I/O statistics
type NetworkStats struct {
	Interfaces     []NetInterfaceStats `json:"interfaces"`
	TotalBytesSent uint64              `json:"total_bytes_sent"`
	TotalBytesRecv uint64              `json:"total_bytes_recv"`
	TotalSentGB    float64             `json:"total_sent_gb"`
	TotalRecvGB    float64             `json:"total_recv_gb"`
}

// NetInterfaceStats represents per-interface network stats
type NetInterfaceStats struct {
	Name      string  `json:"name"`
	BytesSent uint64  `json:"bytes_sent"`
	BytesRecv uint64  `json:"bytes_recv"`
	SentGB    float64 `json:"sent_gb"`
	RecvGB    float64 `json:"recv_gb"`
}

// HostStats represents simplified stats for overview display
type HostStats struct {
	CPUPercent    float64     `json:"cpu_percent"`
	CPUCores      int         `json:"cpu_cores"`
	CPUSpeedGHz   float64     `json:"cpu_speed_ghz"`
	MemoryPercent float64     `json:"memory_percent"`
	MemoryTotalGB float64     `json:"memory_total_gb"`
	MemoryUsedGB  float64     `json:"memory_used_gb"`
	DiskPercent   float64     `json:"disk_percent"`
	DiskTotalGB   float64     `json:"disk_total_gb"`
	DiskUsedGB    float64     `json:"disk_used_gb"`
	DiskCount     int         `json:"disk_count"`
	Disks         []DiskStats `json:"disks,omitempty"`
	NetSentGB     float64     `json:"net_sent_gb"`
	NetRecvGB     float64     `json:"net_recv_gb"`
	Hostname      string      `json:"hostname"`
	OS            string      `json:"os"`
	UptimeSeconds uint64      `json:"uptime_seconds"`
}
