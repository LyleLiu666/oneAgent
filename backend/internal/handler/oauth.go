package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

// OAuthCallbackRequest represents the request body for the callback endpoint
type OAuthCallbackRequest struct {
	Code        string `json:"code" binding:"required"`
	RedirectURI string `json:"redirect_uri" binding:"required"`
}

// OAuthCallbackResponse represents the response from the callback endpoint
type OAuthCallbackResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	User        struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Name     string `json:"name"`
	} `json:"user"`
}

// KeycloakTokenResponse represents the response from Keycloak token endpoint
type KeycloakTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	IDToken          string `json:"id_token"`
}

// KeycloakTokenClaims represents claims from Keycloak access token
type KeycloakTokenClaims struct {
	jwt.RegisteredClaims
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
}

// LocalTokenClaims represents claims for backend-issued JWT
type LocalTokenClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

// OAuthCallback handles the OAuth authorization code exchange
func OAuthCallback(c *gin.Context) {
	cfg := config.GetConfig()

	var req OAuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Exchange code for token with Keycloak
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		cfg.KeycloakURL, cfg.KeycloakRealm)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", cfg.KeycloakClientID)
	data.Set("client_secret", cfg.KeycloakClientSecret)
	data.Set("code", req.Code)
	data.Set("redirect_uri", req.RedirectURI)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Token exchange failed",
			"details": errResp,
		})
		return
	}

	var tokenResp KeycloakTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse token response"})
		return
	}

	// Parse Keycloak access token to get user info (without validation since we just got it)
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, _, err := parser.ParseUnverified(tokenResp.AccessToken, &KeycloakTokenClaims{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Keycloak token"})
		return
	}

	keycloakClaims, ok := token.Claims.(*KeycloakTokenClaims)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract Keycloak claims"})
		return
	}

	// Generate our own JWT
	expireDays := cfg.JWTExpireDays
	if expireDays <= 0 {
		expireDays = 7
	}
	expiresAt := time.Now().Add(time.Duration(expireDays) * 24 * time.Hour)

	localClaims := LocalTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   keycloakClaims.Subject,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "oneAgent-backend",
		},
		Username: keycloakClaims.PreferredUsername,
		Email:    keycloakClaims.Email,
		Name:     keycloakClaims.Name,
	}

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		jwtSecret = "default-secret-change-me" // Fallback for dev
	}

	localToken := jwt.NewWithClaims(jwt.SigningMethodHS256, localClaims)
	signedToken, err := localToken.SignedString([]byte(jwtSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign token"})
		return
	}

	// Build response
	response := OAuthCallbackResponse{
		AccessToken: signedToken,
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
	}
	response.User.ID = keycloakClaims.Subject
	response.User.Username = keycloakClaims.PreferredUsername
	response.User.Email = keycloakClaims.Email
	response.User.Name = keycloakClaims.Name

	c.JSON(http.StatusOK, response)
}

