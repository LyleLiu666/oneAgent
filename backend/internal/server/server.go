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
	"github.com/liu_y/oneAgent/backend/internal/learning"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/netutil"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/web"
)

func NewRouter(rt *runtime.Runtime) (*gin.Engine, error) {
	if rt == nil || rt.Config == nil {
		return nil, fmt.Errorf("runtime is required")
	}

	if err := ensureTaskQueue(rt); err != nil {
		return nil, err
	}
	learning.StartDailyScheduler(rt)

	router := gin.Default()
	router.Use(middleware.InjectRuntime(rt))
	router.Use(middleware.RequestID())

	// Configure CORS.
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowCredentials = true
	router.Use(cors.New(corsConfig))

	// Auth for all /api routes by default (explicit allowlist for public endpoints).
	router.Use(middleware.APIAuth(rt,
		"/api/auth/config",
		"/api/auth/callback",
		"/api/auth/pair/exchange",
	))

	// Health check endpoint (no auth required).
	router.GET("/health", func(c *gin.Context) {
		health, err := rt.Health(c.Request.Context())
		if err != nil {
			handler.RespondError(c, http.StatusInternalServerError, err)
			return
		}
		c.JSON(http.StatusOK, health)
	})

	// API routes.
	api := router.Group("/api")
	{
		api.GET("/config", handler.GetRuntimeConfig)

		chatHandler := handler.NewChatHandler(rt)
		api.POST("/chat", chatHandler.StreamChat)
		api.GET("/sessions", chatHandler.GetSessions)
		api.GET("/sessions/:id", chatHandler.GetSession)
		api.GET("/sessions/:id/stream", chatHandler.AttachSessionStream)
		api.DELETE("/sessions/:id", chatHandler.DeleteSession)
		api.POST("/sessions/:id/stop", chatHandler.StopSessionStream)
		api.POST("/sessions/:id/truncate", chatHandler.TruncateSession)

		// Task queue.
		api.POST("/tasks", handler.CreateTask)
		api.GET("/tasks", handler.ListTasks)
		api.GET("/tasks/:id", handler.GetTask)
		api.POST("/tasks/:id/cancel", handler.CancelTask)
		api.POST("/tasks/:id/resume", handler.ResumeTask)
		api.GET("/tasks/:id/events", handler.GetTaskEvents)
		api.GET("/tasks/governance", handler.GetTaskQueueGovernance)
		api.POST("/tasks/governance/global", handler.UpdateTaskQueueGlobalPolicy)
		api.POST("/tasks/governance/workspace", handler.UpdateTaskQueueWorkspacePolicy)
		api.POST("/tasks/governance/schedules", handler.CreateTaskQueueSchedule)
		api.GET("/tasks/:id/attempts/:attempt_id/artifacts/:kind", handler.GetTaskAttemptArtifact)
		api.GET("/tasks/:id/attempts/:attempt_id/diff_patch", handler.GetTaskAttemptDiffPatch)
		api.GET("/tasks/:id/attempts/:attempt_id/changed_files", handler.GetTaskAttemptChangedFiles)
		api.GET("/tasks/:id/attempts/:attempt_id/review_comments", handler.ListTaskAttemptReviewComments)
		api.POST("/tasks/:id/attempts/:attempt_id/review_comments", handler.PostTaskAttemptReviewComment)
		api.POST("/tasks/:id/attempts/:attempt_id/rollback", handler.RollbackTaskAttempt)

		// Work ledger.
		api.GET("/ledger/receipts", handler.ListReceipts)
		api.GET("/ledger/receipts/:id", handler.GetReceipt)
		api.GET("/ledger/digests/today", handler.GetDigestToday)
		api.GET("/ledger/digests/:day", handler.GetDigest)
		api.GET("/ledger/digests/today/structured", handler.GetStructuredDigestToday)
		api.GET("/ledger/digests/:day/structured", handler.GetStructuredDigest)
		api.POST("/ledger/followups", handler.CreateLedgerFollowUpTask)
		api.GET("/ledger/status/today", handler.GetLedgerStatusToday)
		api.GET("/ledger/learning/jobs/today", handler.GetLearningJobToday)
		api.POST("/ledger/learning/jobs/run_today", handler.RunLearningJobToday)

		// SOP suggestions.
		api.POST("/ledger/sop_suggestions", handler.CreateSuggestion)
		api.POST("/ledger/sop_suggestions/generate", handler.GenerateSuggestions)
		api.GET("/ledger/sop_suggestions", handler.ListSuggestions)
		api.GET("/ledger/sop_suggestions/:id", handler.GetSuggestion)
		api.GET("/ledger/sop_suggestions/:id/similar", handler.GetSimilarSuggestions)
		api.PUT("/ledger/sop_suggestions/:id", handler.UpdateSuggestion)
		api.POST("/ledger/sop_suggestions/:id/status", handler.UpdateSuggestionStatus)
		api.POST("/ledger/sop_suggestions/load_more", handler.LoadMoreSuggestions)

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
		api.POST("/approvals/:id/approve", handler.ApproveToolApproval)
		api.POST("/approvals/:id/deny", handler.DenyToolApproval)
		api.GET("/command_approvals/settings", handler.GetCommandApprovalSettings)
		api.PUT("/command_approvals/settings", handler.UpdateCommandApprovalSettings)

		// MCP server (local-only by default).
		mcp := api.Group("/mcp")
		mcp.Use(middleware.RequireLoopback(rt.Config.MCPAllowRemote, "MCP server", "MCP_ALLOW_REMOTE"))
		mcp.POST("", handler.HandleMCP)
		mcp.GET("/events/tasks", handler.StreamMCPTaskEvents)

		// Document export (pandoc).
		api.POST("/documents/export", handler.ExportDocument)

		// Admin (local-only).
		api.GET("/admin/tokens", handler.ListAuthTokens)
		api.POST("/admin/tokens", handler.CreateAuthToken)
		api.POST("/admin/tokens/revoke", handler.RevokeAuthToken)
		api.POST("/admin/pairing_codes", handler.CreatePairingCode)
		api.GET("/admin/tool_policies/:principal_id", handler.GetToolPolicy)
		api.PUT("/admin/tool_policies/:principal_id", handler.SetToolPolicy)

		// Skills governance (read-only list + archive personal skills).
		api.GET("/skills", handler.ListSkills)
		api.GET("/skills/duplicates", handler.ListSkillDuplicates)
		api.GET("/skills/stale", handler.ListStaleSkills)
		api.GET("/skills/:id", handler.GetSkill)
		api.GET("/skills/:id/file", handler.ReadSkillFile)
		api.PUT("/skills/:id", handler.UpdateSkill)
		api.POST("/skills/:id/archive", handler.ArchiveSkill)
		api.POST("/skills/:id/deprecate", handler.DeprecateSkill)
		api.POST("/skills/:id/pin", handler.PinSkillCandidate)
		api.POST("/skills/:id/archive_shadowed", handler.ArchiveShadowedPersonalDuplicates)

		// Workspace helpers (local-tool mode).
		api.POST("/workspace/choose", handler.ChooseWorkspace)

		// Workflow orchestration (MVP, workspace-scoped).
		api.GET("/workflows", handler.ListWorkflows)
		api.POST("/workflows", handler.CreateWorkflow)
		api.PATCH("/workflows/:id", handler.RenameWorkflow)
		api.DELETE("/workflows/:id", handler.DeleteWorkflow)
		api.POST("/workflows/:id/publish", handler.PublishWorkflowVersion)
		api.POST("/workflows/:id/runs", handler.CreateWorkflowRun)
		api.GET("/workflows/:id/runs/:run_id", handler.GetWorkflowRun)
		api.POST("/workflows/:id/runs/:run_id/execute", handler.ExecuteWorkflowRun)
		api.POST("/workflows/:id/runs/:run_id/cancel", handler.CancelWorkflowRun)

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
	router.POST("/api/auth/pair/exchange", handler.ExchangePairingCode)

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
			// If frontend assets were not bundled (e.g. backend-only CI), don't 500.
			c.String(http.StatusNotFound, "Not found")
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
	if !netutil.IsLoopbackBind(rt.Config.Bind) {
		log.Printf("WARNING: Server bind=%s is non-loopback and may expose your local agent to the network. Prefer bind=127.0.0.1.", rt.Config.Bind)
	}
	log.Printf("Server starting on %s", addr)
	return router.Run(addr)
}
