package server

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/bocha"
	"github.com/liu_y/oneAgent/backend/internal/handler"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/web"
)

func NewRouter(rt *runtime.Runtime) (*gin.Engine, error) {
	if rt == nil || rt.Config == nil {
		return nil, fmt.Errorf("runtime is required")
	}

	router := gin.Default()
	router.Use(middleware.InjectRuntime(rt))

	// Configure CORS.
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowCredentials = true
	router.Use(cors.New(corsConfig))

	// Auth for all /api routes by default (explicit allowlist for public endpoints).
	router.Use(middleware.APIAuth(rt,
		"/api/auth/config",
		"/api/auth/callback",
	))

	// Health check endpoint (no auth required).
	router.GET("/health", func(c *gin.Context) {
		health, err := rt.Health(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, health)
	})

	// API routes.
	api := router.Group("/api")
	{
		chatHandler := handler.NewChatHandler(rt)
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

		// Tool metadata.
		api.GET("/tools", handler.ListTools)

		// Workspace helpers (local-tool mode).
		api.POST("/workspace/choose", handler.ChooseWorkspace)

		// Bocha search services.
		api.GET("/bocha/settings", bocha.GetSettingsHandler)
		api.PUT("/bocha/settings", bocha.UpdateSettingsHandler)
		api.POST("/bocha/search", bocha.SearchHandler)

		// User info endpoint.
		api.GET("/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"user_id":  middleware.GetUserID(c),
				"username": middleware.GetUsername(c),
			})
		})
	}

	// Public Auth Endpoints (No Auth Required).
	router.GET("/api/auth/config", handler.GetAuthConfig)
	router.POST("/api/auth/callback", handler.OAuthCallback)

	// Serve static files (Vue frontend).
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		log.Printf("Warning: Failed to load static files: %v", err)
		return router, nil
	}

	fileServer := http.FileServer(http.FS(staticFS))
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API requests should return 404 JSON.
		if len(path) >= 4 && path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		// Check if file exists in staticFS.
		filePath := path
		if len(filePath) > 0 && filePath[0] == '/' {
			filePath = filePath[1:]
		}

		f, err := staticFS.Open(filePath)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// Fallback to index.html (SPA).
		indexFile, err := web.Static.ReadFile("static/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load page")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexFile)
	})

	return router, nil
}

func Serve(rt *runtime.Runtime) error {
	router, err := NewRouter(rt)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%s", rt.Config.Bind, rt.Config.Port)
	log.Printf("Server starting on %s", addr)
	return router.Run(addr)
}
