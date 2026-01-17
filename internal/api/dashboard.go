package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleDashboardStream provides SSE streaming for dashboard updates
func (r *Router) handleDashboardStream(c *gin.Context) {
	setupSSEHeaders(c)

	// Get refresh interval from config (default 5 seconds)
	interval := time.Duration(r.config.Dashboard.RefreshInterval) * time.Second
	if interval < time.Second {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send initial data
	r.sendDashboardData(c)

	for {
		select {
		case <-ticker.C:
			r.sendDashboardData(c)
		case <-c.Request.Context().Done():
			return
		}
	}
}

// sendDashboardData sends current dashboard data as SSE event
func (r *Router) sendDashboardData(c *gin.Context) {
	hosts := r.hostsService.GetHosts()

	data := gin.H{
		"hosts":     hosts,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	c.SSEvent("message", string(jsonData))
	c.Writer.Flush()
}

// handleDashboardStorage returns storage statistics
func (r *Router) handleDashboardStorage(c *gin.Context) {
	if r.timeseriesService == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
		})
		return
	}

	stats, err := r.timeseriesService.GetStorageStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"stats":   stats,
	})
}

// handleDashboardConfig returns dashboard configuration
func (r *Router) handleDashboardConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"timeline_points":   r.config.Dashboard.TimelinePoints,
		"timeline_interval": r.config.Dashboard.TimelineInterval,
		"refresh_interval":  r.config.Dashboard.RefreshInterval,
	})
}
