package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/liu_y/oneAgent/backend/internal/database"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/model"
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
	ProviderID    string `json:"provider_id" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Model         string `json:"model" binding:"required"`
	IsDefault     bool   `json:"is_default"`
	EnableKVCache *bool  `json:"enable_kv_cache"`
}

type updateModelRequest struct {
	Name          *string `json:"name"`
	Model         *string `json:"model"`
	IsDefault     *bool   `json:"is_default"`
	EnableKVCache *bool   `json:"enable_kv_cache"`
}

// ListProviders returns providers (optionally with models) for the current user.
func ListProviders(c *gin.Context) {
	userID := middleware.GetUserID(c)
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusOK, []providerResponse{})
		return
	}

	var providers []model.LLMProvider
	if err := db.Where("user_id = ?", userID).
		Preload("Models").
		Order("updated_at DESC").
		Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load providers"})
		return
	}

	resp := make([]providerResponse, 0, len(providers))
	for _, provider := range providers {
		resp = append(resp, toProviderResponse(provider))
	}

	c.JSON(http.StatusOK, resp)
}

// CreateProvider stores a new provider configuration.
func CreateProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	var req createProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	providerType := strings.ToLower(strings.TrimSpace(req.ProviderType))
	if !supportedProviderTypes[providerType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider type"})
		return
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	baseURL = strings.TrimSuffix(baseURL, "/")

	provider := model.LLMProvider{
		ID:           uuid.New().String(),
		UserID:       userID,
		Name:         strings.TrimSpace(req.Name),
		ProviderType: providerType,
		BaseURL:      baseURL,
		APIKey:       strings.TrimSpace(req.APIKey),
	}

	if err := db.Create(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create provider"})
		return
	}

	c.JSON(http.StatusCreated, toProviderResponse(provider))
}

// UpdateProvider updates an existing provider configuration.
func UpdateProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	providerID := c.Param("id")
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	var provider model.LLMProvider
	if err := db.Where("id = ? AND user_id = ?", providerID, userID).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}

	var req updateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	if len(updates) == 0 {
		c.JSON(http.StatusOK, toProviderResponse(provider))
		return
	}

	if err := db.Model(&provider).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update provider"})
		return
	}

	db.Preload("Models").First(&provider, "id = ?", providerID)
	c.JSON(http.StatusOK, toProviderResponse(provider))
}

// DeleteProvider removes a provider and its models.
func DeleteProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	providerID := c.Param("id")
	db := database.GetDB()
	if db == nil {
		c.Status(http.StatusNoContent)
		return
	}

	result := db.Where("id = ? AND user_id = ?", providerID, userID).Delete(&model.LLMProvider{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// ListModels returns models for the current user.
func ListModels(c *gin.Context) {
	userID := middleware.GetUserID(c)
	providerID := strings.TrimSpace(c.Query("provider_id"))
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusOK, []modelResponse{})
		return
	}

	query := db.Where("user_id = ?", userID)
	if providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}

	var models []model.LLMModel
	if err := query.Preload("Provider").Order("updated_at DESC").Find(&models).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load models"})
		return
	}

	resp := make([]modelResponse, 0, len(models))
	for _, m := range models {
		resp = append(resp, toModelResponse(m))
	}

	c.JSON(http.StatusOK, resp)
}

// CreateModel adds a model under a provider.
func CreateModel(c *gin.Context) {
	userID := middleware.GetUserID(c)
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	var req createModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var provider model.LLMProvider
	if err := db.Where("id = ? AND user_id = ?", req.ProviderID, userID).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}

	enableKV := true
	if req.EnableKVCache != nil {
		enableKV = *req.EnableKVCache
	}

	llmModel := model.LLMModel{
		ID:            uuid.New().String(),
		ProviderID:    req.ProviderID,
		UserID:        userID,
		Name:          strings.TrimSpace(req.Name),
		Model:         strings.TrimSpace(req.Model),
		IsDefault:     req.IsDefault,
		EnableKVCache: enableKV,
	}

	if err := db.Create(&llmModel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create model"})
		return
	}

	if llmModel.IsDefault {
		clearOtherDefaults(db, userID, llmModel.ProviderID, llmModel.ID)
	}

	db.Preload("Provider").First(&llmModel, "id = ?", llmModel.ID)
	c.JSON(http.StatusCreated, toModelResponse(llmModel))
}

// UpdateModel updates an existing model.
func UpdateModel(c *gin.Context) {
	userID := middleware.GetUserID(c)
	modelID := c.Param("id")
	db := database.GetDB()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	var llmModel model.LLMModel
	if err := db.Where("id = ? AND user_id = ?", modelID, userID).First(&llmModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	var req updateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	if len(updates) == 0 {
		db.Preload("Provider").First(&llmModel, "id = ?", llmModel.ID)
		c.JSON(http.StatusOK, toModelResponse(llmModel))
		return
	}

	if err := db.Model(&llmModel).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update model"})
		return
	}

	if req.IsDefault != nil && *req.IsDefault {
		clearOtherDefaults(db, userID, llmModel.ProviderID, llmModel.ID)
	}

	db.Preload("Provider").First(&llmModel, "id = ?", llmModel.ID)
	c.JSON(http.StatusOK, toModelResponse(llmModel))
}

// DeleteModel removes a model.
func DeleteModel(c *gin.Context) {
	userID := middleware.GetUserID(c)
	modelID := c.Param("id")
	db := database.GetDB()
	if db == nil {
		c.Status(http.StatusNoContent)
		return
	}

	result := db.Where("id = ? AND user_id = ?", modelID, userID).Delete(&model.LLMModel{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func clearOtherDefaults(db *gorm.DB, userID, providerID, modelID string) {
	db.Model(&model.LLMModel{}).
		Where("user_id = ? AND provider_id = ? AND id <> ?", userID, providerID, modelID).
		Update("is_default", false)
}

func toProviderResponse(provider model.LLMProvider) providerResponse {
	resp := providerResponse{
		ID:           provider.ID,
		Name:         provider.Name,
		ProviderType: provider.ProviderType,
		BaseURL:      provider.BaseURL,
		HasAPIKey:    provider.APIKey != "",
		CreatedAt:    provider.CreatedAt,
		UpdatedAt:    provider.UpdatedAt,
	}

	if len(provider.Models) > 0 {
		resp.Models = make([]modelResponse, 0, len(provider.Models))
		for _, m := range provider.Models {
			resp.Models = append(resp.Models, toModelResponse(m))
		}
	}

	return resp
}

func toModelResponse(m model.LLMModel) modelResponse {
	resp := modelResponse{
		ID:            m.ID,
		ProviderID:    m.ProviderID,
		Name:          m.Name,
		Model:         m.Model,
		IsDefault:     m.IsDefault,
		EnableKVCache: m.EnableKVCache,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
	if m.Provider.ID != "" {
		resp.Provider = &providerSlim{
			ID:           m.Provider.ID,
			Name:         m.Provider.Name,
			ProviderType: m.Provider.ProviderType,
			BaseURL:      m.Provider.BaseURL,
		}
	}
	return resp
}
