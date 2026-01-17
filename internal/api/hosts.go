package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleListHosts returns all configured hosts with their status
func (r *Router) handleListHosts(c *gin.Context) {
	hosts := r.hostsService.GetHosts()
	c.JSON(http.StatusOK, gin.H{"hosts": hosts})
}

// handleAddHost adds a new host
func (r *Router) handleAddHost(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Address string `json:"address" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name and address required"})
		return
	}

	host, err := r.hostsService.AddHost(req.Name, req.Address)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, host)
}

// handleGetHost returns a single host
func (r *Router) handleGetHost(c *gin.Context) {
	id := c.Param("id")

	host, err := r.hostsService.GetHost(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, host)
}

// handleDeleteHost removes a host
func (r *Router) handleDeleteHost(c *gin.Context) {
	id := c.Param("id")

	if err := r.hostsService.RemoveHost(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Host removed"})
}

// handleTestHostConnection tests connection to a host address
func (r *Router) handleTestHostConnection(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Address required"})
		return
	}

	hostname, err := r.hostsService.TestConnection(req.Address)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"hostname": hostname,
	})
}

// handleHostTimeline returns the status timeline for a host
func (r *Router) handleHostTimeline(c *gin.Context) {
	id := c.Param("id")

	timeline, err := r.hostsService.GetHostTimeline(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"timeline": timeline})
}
