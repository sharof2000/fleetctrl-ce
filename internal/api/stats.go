package api

import (
	"net/http"

	"fleetctrl/internal/models"
	"github.com/gin-gonic/gin"
)

// handleLocalStats returns local system statistics
func (r *Router) handleLocalStats(c *gin.Context) {
	stats, err := r.monitorService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handlePeerStats returns stats for peer queries (used by other hosts)
func (r *Router) handlePeerStats(c *gin.Context) {
	stats, err := r.monitorService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to simplified HostStats
	hostStats := convertToHostStats(stats)
	c.JSON(http.StatusOK, hostStats)
}

// convertToHostStats converts SystemStats to simplified HostStats
func convertToHostStats(stats *models.SystemStats) *models.HostStats {
	result := &models.HostStats{
		CPUPercent:    stats.CPU.Percent,
		CPUCores:      stats.CPU.LogicalCores,
		CPUSpeedGHz:   stats.CPU.SpeedGHz,
		MemoryPercent: stats.Memory.Percent,
		MemoryTotalGB: stats.Memory.TotalGB,
		MemoryUsedGB:  stats.Memory.UsedGB,
		NetSentGB:     stats.Network.TotalSentGB,
		NetRecvGB:     stats.Network.TotalRecvGB,
		Hostname:      stats.Hostname,
		OS:            stats.OS,
		UptimeSeconds: stats.UptimeSeconds,
		Disks:         stats.Disks,
		DiskCount:     len(stats.Disks),
	}

	// Calculate total disk usage
	var totalDisk, usedDisk uint64
	for _, d := range stats.Disks {
		totalDisk += d.TotalBytes
		usedDisk += d.UsedBytes
	}

	if totalDisk > 0 {
		result.DiskPercent = float64(usedDisk) / float64(totalDisk) * 100
		result.DiskTotalGB = float64(totalDisk) / (1024 * 1024 * 1024)
		result.DiskUsedGB = float64(usedDisk) / (1024 * 1024 * 1024)
	}

	return result
}
