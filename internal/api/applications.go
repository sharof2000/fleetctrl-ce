package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// getFileParam extracts and normalizes the file path parameter.
// Strips leading slash from wildcard parameters.
func getFileParam(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("file"), "/")
}

// Request/Response types for applications API

type gitDeployRequest struct {
	Name   string `json:"name" binding:"required"`
	GitURL string `json:"git_url" binding:"required"`
	Branch string `json:"branch"`
}

type saveFileRequest struct {
	Content string `json:"content" binding:"required"`
}

// handleListApplications returns all applications
func (r *Router) handleListApplications(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	apps, err := r.appsService.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"applications":       apps,
		"git_deploy_enabled": r.config.Applications.GitDeployEnabled,
	})
}

// handleGetApplication returns a single application with full details
func (r *Router) handleGetApplication(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")

	app, err := r.appsService.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, app)
}

// handleDeleteApplication deletes an application
func (r *Router) handleDeleteApplication(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")

	if err := r.appsService.Delete(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// handleBackupApplication creates a ZIP backup of an application and serves it for download
func (r *Router) handleBackupApplication(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")

	// Create the backup
	backupPath, err := r.appsService.CreateBackup(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get the filename for the download
	fileName := filepath.Base(backupPath)

	// Serve the file for download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "application/zip")
	c.File(backupPath)

	// Clean up the backup file after serving (in a goroutine to not block response)
	go func() {
		if err := os.Remove(backupPath); err != nil {
			log.Printf("[applications] Failed to cleanup backup file %s: %v", backupPath, err)
		}
	}()
}

// handleGitDeploy deploys an application from a git repository
func (r *Router) handleGitDeploy(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	if !r.config.Applications.GitDeployEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Git deployment is disabled"})
		return
	}

	var req gitDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := r.appsService.GitDeploy(req.Name, req.GitURL, req.Branch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "deployed", "name": req.Name})
}

// handleGetComposeFile returns the content of a compose file
func (r *Router) handleGetComposeFile(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")
	file := getFileParam(c)

	content, err := r.appsService.GetComposeFile(name, file)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": content, "file": file})
}

// handleSaveComposeFile saves compose file content
func (r *Router) handleSaveComposeFile(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")
	file := getFileParam(c)

	var req saveFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := r.appsService.SaveComposeFile(name, file, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "saved"})
}

// handleStartCompose starts a specific compose file
func (r *Router) handleStartCompose(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")
	file := getFileParam(c)

	if err := r.appsService.StartCompose(name, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

// handleStopCompose stops a specific compose file
func (r *Router) handleStopCompose(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")
	file := getFileParam(c)

	if err := r.appsService.StopCompose(name, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// handleRestartCompose restarts a specific compose file
func (r *Router) handleRestartCompose(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")
	file := getFileParam(c)

	if err := r.appsService.RestartCompose(name, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

// handleGetEnvFile returns the .env file content
func (r *Router) handleGetEnvFile(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")

	content, err := r.appsService.GetEnvFile(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": content})
}

// handleSaveEnvFile saves the .env file content
func (r *Router) handleSaveEnvFile(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")

	var req saveFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := r.appsService.SaveEnvFile(name, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "saved"})
}

// handleStartService starts a specific service within a compose file
func (r *Router) handleStartService(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")
	file := getFileParam(c)
	service := c.Param("service")

	if err := r.appsService.StartService(name, file, service); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

// handleStopService stops a specific service within a compose file
func (r *Router) handleStopService(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")
	file := getFileParam(c)
	service := c.Param("service")

	if err := r.appsService.StopService(name, file, service); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// handleRestartService restarts a specific service within a compose file
func (r *Router) handleRestartService(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")
	file := getFileParam(c)
	service := c.Param("service")

	if err := r.appsService.RestartService(name, file, service); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

// handleComposeOutput streams docker compose output via SSE
func (r *Router) handleComposeOutput(c *gin.Context) {
	if r.appsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Applications service not available"})
		return
	}

	name := c.Param("name")
	action := c.Param("action")
	file := getFileParam(c)

	// Validate action
	validActions := map[string]bool{"up": true, "stop": true, "restart": true, "down": true, "pull": true}
	if !validActions[action] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
		return
	}

	// Set SSE headers
	setupSSEHeaders(c)

	// Get the response writer for flushing
	w := c.Writer

	// Create a channel for output lines
	outputChan := make(chan string, 100)
	doneChan := make(chan error, 1)

	// Run compose command in a goroutine
	go func() {
		err := r.appsService.RunComposeWithOutput(name, file, action, func(line string) {
			outputChan <- line
		})
		doneChan <- err
		close(outputChan)
	}()

	// Stream output as SSE events
	for {
		select {
		case line, ok := <-outputChan:
			if !ok {
				// Channel closed, wait for done
				err := <-doneChan
				if err != nil {
					c.SSEvent("error", err.Error())
				}
				c.SSEvent("done", "completed")
				w.Flush()
				return
			}
			c.SSEvent("output", line)
			w.Flush()

		case err := <-doneChan:
			// Drain remaining output
			for line := range outputChan {
				c.SSEvent("output", line)
				w.Flush()
			}
			if err != nil {
				c.SSEvent("error", err.Error())
			}
			c.SSEvent("done", "completed")
			w.Flush()
			return

		case <-c.Request.Context().Done():
			// Client disconnected
			return
		}
	}
}
