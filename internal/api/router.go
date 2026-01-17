package api

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"fleetctrl/internal/config"
	"fleetctrl/internal/services/auth"
	"fleetctrl/internal/services/hosts"
	"fleetctrl/internal/services/monitor"
	"fleetctrl/internal/services/timeseries"
	"fleetctrl/internal/version"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Host-Token, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

// setupSSEHeaders sets the required headers for Server-Sent Events streaming
func setupSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
}

// Router holds all dependencies for HTTP handlers
type Router struct {
	config            *config.Config
	authService       *auth.Service
	monitorService    *monitor.Service
	hostsService      *hosts.Service
	timeseriesService *timeseries.Service
	httpClient        *http.Client
}

// NewRouter creates a new router with all routes configured
func NewRouter(
	cfg *config.Config,
	authService *auth.Service,
	monitorService *monitor.Service,
	hostsService *hosts.Service,
	timeseriesService *timeseries.Service,
	templateFiles embed.FS,
	staticFiles embed.FS,
) *gin.Engine {
	r := &Router{
		config:            cfg,
		authService:       authService,
		monitorService:    monitorService,
		hostsService:      hostsService,
		timeseriesService: timeseriesService,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), CORSMiddleware(), SecurityHeadersMiddleware())

	// Load templates
	tmpl := template.Must(template.ParseFS(templateFiles, "templates/*.html", "templates/partials/*.html"))
	engine.SetHTMLTemplate(tmpl)

	// Serve static files
	staticFS, _ := fs.Sub(staticFiles, "static")
	engine.StaticFS("/static", http.FS(staticFS))

	// Public routes
	engine.GET("/", r.handleIndex)
	engine.GET("/login", r.handleLoginPage)
	engine.POST("/api/login", r.handleLogin)
	engine.POST("/api/refresh", r.handleRefreshToken)
	engine.GET("/health", r.handleHealth)
	engine.GET("/api/version", handleVersion)

	// Protected page routes
	engine.GET("/dashboard", r.handleDashboardPage)
	engine.GET("/applications", r.handleApplicationsPage)  // Placeholder
	engine.GET("/files", r.handleFilesPage)                // Placeholder
	engine.GET("/services", r.handleServicesPage)          // Placeholder (BE-only)

	// Protected API routes
	api := engine.Group("/api")
	api.Use(r.AuthMiddleware())
	{
		// Dashboard / Hosts
		api.GET("/hosts", r.handleListHosts)
		api.POST("/hosts", r.handleAddHost)
		api.POST("/hosts/test", r.handleTestHostConnection)
		api.GET("/hosts/:id", r.handleGetHost)
		api.DELETE("/hosts/:id", r.handleDeleteHost)
		api.GET("/hosts/:id/timeline", r.handleHostTimeline)

		// Dashboard streaming
		api.GET("/dashboard/stream", r.handleDashboardStream)
		api.GET("/dashboard/storage", r.handleDashboardStorage)
		api.GET("/dashboard/config", r.handleDashboardConfig)

		// Local stats
		api.GET("/local/stats", r.handleLocalStats)

		// Settings
		api.PUT("/settings/password", r.handleChangePassword)
	}

	// Peer routes (for cross-host queries)
	peerAPI := engine.Group("/api/peer")
	peerAPI.Use(r.HostAuthMiddleware())
	{
		peerAPI.GET("/stats", r.handlePeerStats)
	}

	return engine
}

func (r *Router) handleIndex(c *gin.Context) {
	c.Redirect(http.StatusFound, "/dashboard")
}

func (r *Router) handleHealth(c *gin.Context) {
	response := gin.H{"status": "ok"}

	if r.monitorService != nil {
		if stats, err := r.monitorService.GetStats(); err == nil {
			response["hostname"] = stats.Hostname
		}
	}

	c.JSON(http.StatusOK, response)
}

func handleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, version.Get())
}

// Page handlers
func (r *Router) handleDashboardPage(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"Title":      "Dashboard",
		"ActivePage": "dashboard",
		"Edition":    "Community",
	})
}

func (r *Router) handleApplicationsPage(c *gin.Context) {
	// Coming Soon placeholder
	c.HTML(http.StatusOK, "placeholder.html", gin.H{
		"Title":       "Applications",
		"ActivePage":  "applications",
		"Edition":     "Community",
		"FeatureName": "Applications",
		"Description": "Manage Docker Compose applications with an intuitive interface.",
		"ComingSoon":  true,
		"Features": []string{
			"Deploy and manage Docker Compose applications",
			"Edit compose files and environment variables",
			"View container logs and status",
			"Start, stop, and restart services",
		},
	})
}

func (r *Router) handleFilesPage(c *gin.Context) {
	// Coming Soon placeholder
	c.HTML(http.StatusOK, "placeholder.html", gin.H{
		"Title":       "Files",
		"ActivePage":  "files",
		"Edition":     "Community",
		"FeatureName": "File Manager",
		"Description": "Browse and edit configuration files for your applications.",
		"ComingSoon":  true,
		"Features": []string{
			"Browse application directories",
			"Edit configuration files with syntax highlighting",
			"View log files with search and auto-refresh",
			"Download files and folders as ZIP",
		},
	})
}

func (r *Router) handleServicesPage(c *gin.Context) {
	// BE-only placeholder
	c.HTML(http.StatusOK, "placeholder.html", gin.H{
		"Title":       "Services",
		"ActivePage":  "services",
		"Edition":     "Community",
		"FeatureName": "System Services",
		"Description": "Manage system services with a secure whitelist-based approach.",
		"ComingSoon":  false,
		"BusinessOnly": true,
		"Features": []string{
			"Control system services (start, stop, restart)",
			"View service status and logs",
			"Whitelist-based security model",
			"Cross-platform support (Windows Services, systemd)",
		},
	})
}

// handleRefreshToken handles token refresh requests
func (r *Router) handleRefreshToken(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token required"})
		return
	}

	newToken, expiresAt, err := r.authService.RefreshToken(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      newToken,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}
