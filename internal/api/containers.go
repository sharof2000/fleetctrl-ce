package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// requireDocker checks if Docker is available and returns an error response if not.
// Returns true if Docker is available, false otherwise.
func (r *Router) requireDocker(c *gin.Context) bool {
	if r.dockerService == nil || !r.dockerService.IsAvailable() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Docker is not available", "docker_available": false})
		return false
	}
	return true
}

func (r *Router) handleListContainers(c *gin.Context) {
	if !r.requireDocker(c) {
		return
	}

	containers, err := r.dockerService.ListContainers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"containers": containers})
}

func (r *Router) handleStopContainer(c *gin.Context) {
	if !r.requireDocker(c) {
		return
	}

	id := c.Param("id")
	if err := r.dockerService.StopContainer(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

func (r *Router) handleStartContainer(c *gin.Context) {
	if !r.requireDocker(c) {
		return
	}

	id := c.Param("id")
	if err := r.dockerService.StartContainer(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

func (r *Router) handleRestartContainer(c *gin.Context) {
	if !r.requireDocker(c) {
		return
	}

	id := c.Param("id")
	if err := r.dockerService.RestartContainer(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

func (r *Router) handleContainerLogs(c *gin.Context) {
	if !r.requireDocker(c) {
		return
	}

	id := c.Param("id")
	tail := c.DefaultQuery("tail", "100")

	logs, err := r.dockerService.GetContainerLogs(id, tail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func (r *Router) handleDockerStatus(c *gin.Context) {
	available := r.dockerService != nil && r.dockerService.IsAvailable()
	c.JSON(http.StatusOK, gin.H{"docker_available": available})
}

func (r *Router) handleDockerReconnect(c *gin.Context) {
	if r.dockerService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Docker service not initialized"})
		return
	}

	success := r.dockerService.Reconnect()
	if success {
		c.JSON(http.StatusOK, gin.H{"status": "reconnected", "docker_available": true})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "failed", "docker_available": false})
	}
}

func (r *Router) handleContainerStats(c *gin.Context) {
	if !r.requireDocker(c) {
		return
	}

	id := c.Param("id")
	stats, err := r.dockerService.GetContainerResourceStats(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (r *Router) handleBatchContainerStats(c *gin.Context) {
	if !r.requireDocker(c) {
		return
	}

	var req struct {
		ContainerIDs []string `json:"container_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	stats, err := r.dockerService.GetBatchContainerStats(req.ContainerIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
