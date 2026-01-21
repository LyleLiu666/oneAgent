package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/bocha"
	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/database"
	"github.com/liu_y/oneAgent/backend/internal/handler"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	if cfg.DatabaseURL != "" {
		_, err = database.Connect(cfg)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v", err)
		} else {
			if err := database.AutoMigrate(); err != nil {
				log.Printf("Warning: Failed to run migrations: %v", err)
			}
		}
	} else {
		log.Println("Warning: DATABASE_URL not set, running without database")
	}

	// Initialize Gin router
	router := gin.Default()

	// Configure CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowCredentials = true
	router.Use(cors.New(corsConfig))

	// JWT auth for all /api routes by default (explicit allowlist for public endpoints).
	router.Use(middleware.APIJWTAuth(
		"/api/auth/config",
		"/api/auth/callback",
	))

	// Health check endpoint (no auth required)
	router.GET("/health", func(c *gin.Context) {
		status := "healthy"
		if err := database.HealthCheck(); err != nil {
			status = "degraded"
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   status,
			"database": err == nil,
		})
	})

	// API routes
	api := router.Group("/api")
	{
		chatHandler := handler.NewChatHandler(cfg)
		api.POST("/chat", chatHandler.StreamChat)
		api.GET("/sessions", chatHandler.GetSessions)
		api.GET("/sessions/:id", chatHandler.GetSession)
		api.DELETE("/sessions/:id", chatHandler.DeleteSession)
		api.POST("/sessions/:id/truncate", chatHandler.TruncateSession)

		api.GET("/llm/providers", handler.ListProviders)
		api.POST("/llm/providers", handler.CreateProvider)
		api.PUT("/llm/providers/:id", handler.UpdateProvider)
		api.DELETE("/llm/providers/:id", handler.DeleteProvider)

		api.GET("/llm/models", handler.ListModels)
		api.POST("/llm/models", handler.CreateModel)
		api.PUT("/llm/models/:id", handler.UpdateModel)
		api.DELETE("/llm/models/:id", handler.DeleteModel)

		// Bocha search services
		api.GET("/bocha/settings", bocha.GetSettingsHandler)
		api.PUT("/bocha/settings", bocha.UpdateSettingsHandler)
		api.POST("/bocha/search", bocha.SearchHandler)

		// User info endpoint
		api.GET("/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"user_id":  middleware.GetUserID(c),
				"username": middleware.GetUsername(c),
			})
		})
	}

	// Public Auth Endpoints (No Auth Required)
	router.GET("/api/auth/config", handler.GetAuthConfig)
	router.POST("/api/auth/callback", handler.OAuthCallback)

	// Serve static files (Vue frontend)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Printf("Warning: Failed to load static files: %v", err)
	} else {
		// Create file server for static files
		fileServer := http.FileServer(http.FS(staticFS))

		router.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path

			// API requests should return 404 JSON
			if len(path) >= 4 && path[:4] == "/api" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
				return
			}

			// Check if file exists in staticFS
			filePath := path
			if len(filePath) > 0 && filePath[0] == '/' {
				filePath = filePath[1:]
			}

			f, err := staticFS.Open(filePath)
			if err == nil {
				// File exists
				f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}

			// Fallback to index.html (SPA)
			indexFile, err := staticFiles.ReadFile("static/index.html")
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to load page")
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexFile)
		})
	}

	// Start server
	log.Printf("Server starting on port %s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
