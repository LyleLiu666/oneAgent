package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

var supportedProviderTypes = map[string]bool{
	llm.ProviderTypeOpenAI:         true,
	llm.ProviderTypeOpenAIResponse: true,
	llm.ProviderTypeClaude:         true,
	llm.ProviderTypeOpenRouter:     true,
	llm.ProviderTypeBedrock:        true,
	llm.ProviderTypeDeepSeek:       true,
	llm.ProviderTypeZhipuAI:        true,
	llm.ProviderTypeMiniMax:        true,
	llm.ProviderTypeAntigravity:    true,
	llm.ProviderTypeCodex:          true,
}

type providerResponse struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	ProviderType string          `json:"provider_type"`
	BaseURL      string          `json:"base_url"`
	HasAPIKey    bool            `json:"has_api_key"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Models       []modelResponse `json:"models,omitempty"`
}

type modelResponse struct {
	ID            string        `json:"id"`
	ProviderID    string        `json:"provider_id"`
	Name          string        `json:"name"`
	Model         string        `json:"model"`
	IsDefault     bool          `json:"is_default"`
	EnableKVCache bool          `json:"enable_kv_cache"`
	SafetyTier    string        `json:"safety_tier,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	Provider      *providerSlim `json:"provider,omitempty"`
}

type providerSlim struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProviderType string `json:"provider_type"`
	BaseURL      string `json:"base_url"`
}

type createProviderRequest struct {
	Name         string `json:"name" binding:"required"`
	ProviderType string `json:"provider_type" binding:"required"`
	BaseURL      string `json:"base_url" binding:"required"`
	APIKey       string `json:"api_key" binding:"required"`
}

type updateProviderRequest struct {
	Name         *string `json:"name"`
	ProviderType *string `json:"provider_type"`
	BaseURL      *string `json:"base_url"`
	APIKey       *string `json:"api_key"`
}

type createModelRequest struct {
	ProviderID    string  `json:"provider_id" binding:"required"`
	Name          string  `json:"name" binding:"required"`
	Model         string  `json:"model" binding:"required"`
	IsDefault     bool    `json:"is_default"`
	EnableKVCache *bool   `json:"enable_kv_cache"`
	SafetyTier    *string `json:"safety_tier"`
}

type updateModelRequest struct {
	Name          *string `json:"name"`
	Model         *string `json:"model"`
	IsDefault     *bool   `json:"is_default"`
	EnableKVCache *bool   `json:"enable_kv_cache"`
	SafetyTier    *string `json:"safety_tier"`
}

const (
	safetyTierHigh     = "high"
	safetyTierStandard = "standard"
)

func normalizeSafetyTier(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return safetyTierHigh, nil
	}
	if s != safetyTierHigh && s != safetyTierStandard {
		return "", fmt.Errorf("invalid safety_tier: %s", s)
	}
	return s, nil
}

func safetyTierFromOptions(options map[string]any) string {
	if options == nil {
		return safetyTierHigh
	}
	if v, ok := options["safety_tier"].(string); ok {
		if norm, err := normalizeSafetyTier(v); err == nil {
			return norm
		}
	}
	return safetyTierHigh
}

func ListProviders(c *gin.Context) {
	userID := middleware.GetUserID(c)
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusOK, []providerResponse{})
		return
	}

	ctx := c.Request.Context()
	providers, err := rt.Settings.ListProviders(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load providers"})
		return
	}

	models, err := rt.Settings.ListModels(ctx, userID, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load models"})
		return
	}
	modelsByProvider := make(map[string][]settingsdb.Model)
	for _, m := range models {
		modelsByProvider[m.ProviderID] = append(modelsByProvider[m.ProviderID], m)
	}

	resp := make([]providerResponse, 0, len(providers))
	for _, provider := range providers {
		resp = append(resp, toProviderResponse(provider, modelsByProvider[provider.ID]))
	}
	c.JSON(http.StatusOK, resp)
}

func CreateProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Settings not available"})
		return
	}

	var req createProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	providerType := strings.ToLower(strings.TrimSpace(req.ProviderType))
	if !supportedProviderTypes[providerType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider type"})
		return
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	baseURL = strings.TrimSuffix(baseURL, "/")

	provider, err := rt.Settings.CreateProvider(c.Request.Context(), settingsdb.Provider{
		ID:           uuid.New().String(),
		UserID:       userID,
		Name:         strings.TrimSpace(req.Name),
		ProviderType: providerType,
		BaseURL:      baseURL,
		APIKey:       strings.TrimSpace(req.APIKey),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create provider"})
		return
	}

	c.JSON(http.StatusCreated, toProviderResponse(provider, nil))
}

func UpdateProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	providerID := c.Param("id")
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Settings not available"})
		return
	}

	var req updateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.ProviderType != nil {
		providerType := strings.ToLower(strings.TrimSpace(*req.ProviderType))
		if !supportedProviderTypes[providerType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider type"})
			return
		}
		updates["provider_type"] = providerType
	}
	if req.BaseURL != nil {
		baseURL := strings.TrimSpace(*req.BaseURL)
		baseURL = strings.TrimSuffix(baseURL, "/")
		updates["base_url"] = baseURL
	}
	if req.APIKey != nil {
		updates["api_key"] = strings.TrimSpace(*req.APIKey)
	}

	provider, err := rt.Settings.UpdateProvider(c.Request.Context(), userID, providerID, updates)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update provider"})
		return
	}

	models, _ := rt.Settings.ListModels(c.Request.Context(), userID, provider.ID)
	triggerPendingSecretaryTriageRetry(rt, userID)
	c.JSON(http.StatusOK, toProviderResponse(provider, models))
}

func DeleteProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	providerID := c.Param("id")
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.Status(http.StatusNoContent)
		return
	}

	deleted, err := rt.Settings.DeleteProvider(c.Request.Context(), userID, providerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete provider"})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func ListModels(c *gin.Context) {
	userID := middleware.GetUserID(c)
	providerID := strings.TrimSpace(c.Query("provider_id"))
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusOK, []modelResponse{})
		return
	}

	models, err := rt.Settings.ListModels(c.Request.Context(), userID, providerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load models"})
		return
	}

	providers, _ := rt.Settings.ListProviders(c.Request.Context(), userID)
	providerByID := make(map[string]settingsdb.Provider, len(providers))
	for _, p := range providers {
		providerByID[p.ID] = p
	}

	resp := make([]modelResponse, 0, len(models))
	for _, m := range models {
		var provider *providerSlim
		if p, ok := providerByID[m.ProviderID]; ok {
			provider = &providerSlim{
				ID:           p.ID,
				Name:         p.Name,
				ProviderType: p.ProviderType,
				BaseURL:      p.BaseURL,
			}
		}
		resp = append(resp, toModelResponse(m, provider))
	}

	c.JSON(http.StatusOK, resp)
}

func CreateModel(c *gin.Context) {
	userID := middleware.GetUserID(c)
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Settings not available"})
		return
	}

	var req createModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	if _, err := rt.Settings.GetProvider(c.Request.Context(), userID, req.ProviderID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}

	enableKV := true
	if req.EnableKVCache != nil {
		enableKV = *req.EnableKVCache
	}

	tier, err := normalizeSafetyTier("")
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	if req.SafetyTier != nil {
		gotTier, err := normalizeSafetyTier(*req.SafetyTier)
		if err != nil {
			RespondError(c, http.StatusBadRequest, err)
			return
		}
		tier = gotTier
	}

	llmModel, err := rt.Settings.CreateModel(c.Request.Context(), settingsdb.Model{
		ID:            uuid.New().String(),
		ProviderID:    req.ProviderID,
		UserID:        userID,
		Name:          strings.TrimSpace(req.Name),
		Model:         strings.TrimSpace(req.Model),
		IsDefault:     req.IsDefault,
		EnableKVCache: enableKV,
		Options:       map[string]any{"safety_tier": tier},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create model"})
		return
	}

	triggerPendingSecretaryTriageRetry(rt, userID)
	c.JSON(http.StatusCreated, toModelResponse(llmModel, nil))
}

func UpdateModel(c *gin.Context) {
	userID := middleware.GetUserID(c)
	modelID := c.Param("id")
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Settings not available"})
		return
	}

	var req updateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Model != nil {
		updates["model"] = strings.TrimSpace(*req.Model)
	}
	if req.IsDefault != nil {
		updates["is_default"] = *req.IsDefault
	}
	if req.EnableKVCache != nil {
		updates["enable_kv_cache"] = *req.EnableKVCache
	}
	if req.SafetyTier != nil {
		tier, err := normalizeSafetyTier(*req.SafetyTier)
		if err != nil {
			RespondError(c, http.StatusBadRequest, err)
			return
		}
		existing, err := rt.Settings.GetModel(c.Request.Context(), userID, modelID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load model"})
			return
		}
		opts := existing.Options
		if opts == nil {
			opts = map[string]any{}
		}
		opts["safety_tier"] = tier
		updates["options"] = opts
	}

	updated, err := rt.Settings.UpdateModel(c.Request.Context(), userID, modelID, updates)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update model"})
		return
	}

	triggerPendingSecretaryTriageRetry(rt, userID)
	c.JSON(http.StatusOK, toModelResponse(updated, nil))
}

func DeleteModel(c *gin.Context) {
	userID := middleware.GetUserID(c)
	modelID := c.Param("id")
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.Status(http.StatusNoContent)
		return
	}

	deleted, err := rt.Settings.DeleteModel(c.Request.Context(), userID, modelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete model"})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

func toProviderResponse(provider settingsdb.Provider, models []settingsdb.Model) providerResponse {
	resp := providerResponse{
		ID:           provider.ID,
		Name:         provider.Name,
		ProviderType: provider.ProviderType,
		BaseURL:      provider.BaseURL,
		HasAPIKey:    strings.TrimSpace(provider.APIKey) != "",
		CreatedAt:    provider.CreatedAt,
		UpdatedAt:    provider.UpdatedAt,
	}

	if len(models) > 0 {
		resp.Models = make([]modelResponse, 0, len(models))
		for _, m := range models {
			resp.Models = append(resp.Models, toModelResponse(m, nil))
		}
	}

	return resp
}

func triggerPendingSecretaryTriageRetry(rt *runtime.Runtime, userID string) {
	if rt == nil {
		return
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}

	rt.Go(func(ctx context.Context) {
		rt.RetryPendingSecretaryAutoTriage(ctx, userID)
	})
}

func toModelResponse(m settingsdb.Model, provider *providerSlim) modelResponse {
	return modelResponse{
		ID:            m.ID,
		ProviderID:    m.ProviderID,
		Name:          m.Name,
		Model:         m.Model,
		IsDefault:     m.IsDefault,
		EnableKVCache: m.EnableKVCache,
		SafetyTier:    safetyTierFromOptions(m.Options),
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
		Provider:      provider,
	}
}
