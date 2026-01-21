package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

// UserClaims represents JWT claims from Keycloak
type UserClaims struct {
	jwt.RegisteredClaims
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

// JWKS represents the JSON Web Key Set response from Keycloak
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a single JSON Web Key
type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

var (
	jwksCache     *JWKS
	jwksCacheLock sync.RWMutex
	cacheTime     time.Time
	cacheDuration = 15 * time.Minute
)

// KeycloakAuth returns a Gin middleware that validates Keycloak JWT tokens
func KeycloakAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.GetConfig()

		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			return
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			return
		}

		tokenString := parts[1]

		// Parse and validate the token
		token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
			// Validate signing algorithm
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			// Get the key ID from token header
			kid, ok := token.Header["kid"].(string)
			if !ok {
				return nil, fmt.Errorf("kid not found in token header")
			}

			// Get public key from Keycloak
			return getPublicKey(cfg, kid)
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": fmt.Sprintf("Invalid token: %v", err),
			})
			return
		}

		// Extract claims and add to context
		claims, ok := token.Claims.(*UserClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Failed to parse token claims",
			})
			return
		}

		// Add user info to context
		c.Set("user_id", claims.Subject)
		c.Set("username", claims.PreferredUsername)
		c.Set("email", claims.Email)
		c.Set("name", claims.Name)
		c.Set("claims", claims)

		c.Next()
	}
}

// getPublicKey fetches the RSA public key from Keycloak JWKS endpoint
func getPublicKey(cfg *config.Config, kid string) (*rsa.PublicKey, error) {
	jwks, err := fetchJWKS(cfg)
	if err != nil {
		return nil, err
	}

	// Find the key with matching kid
	for _, key := range jwks.Keys {
		if key.Kid == kid {
			return parseRSAPublicKey(key)
		}
	}

	return nil, fmt.Errorf("key with kid %s not found", kid)
}

// fetchJWKS retrieves the JWKS from Keycloak with caching
func fetchJWKS(cfg *config.Config) (*JWKS, error) {
	jwksCacheLock.RLock()
	if jwksCache != nil && time.Since(cacheTime) < cacheDuration {
		defer jwksCacheLock.RUnlock()
		return jwksCache, nil
	}
	jwksCacheLock.RUnlock()

	// Fetch new JWKS
	jwksCacheLock.Lock()
	defer jwksCacheLock.Unlock()

	// Double-check after acquiring write lock
	if jwksCache != nil && time.Since(cacheTime) < cacheDuration {
		return jwksCache, nil
	}

	url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs",
		cfg.KeycloakURL, cfg.KeycloakRealm)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch JWKS: status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}

	jwksCache = &jwks
	cacheTime = time.Now()

	return &jwks, nil
}

// parseRSAPublicKey converts a JWK to an RSA public key
func parseRSAPublicKey(key JWK) (*rsa.PublicKey, error) {
	// Decode the modulus (n)
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %v", err)
	}
	n := new(big.Int).SetBytes(nBytes)

	// Decode the exponent (e)
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %v", err)
	}
	e := int(new(big.Int).SetBytes(eBytes).Int64())

	return &rsa.PublicKey{N: n, E: e}, nil
}

// GetUserID extracts user ID from Gin context
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(string)
	}
	return ""
}

// GetUsername extracts username from Gin context
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		return username.(string)
	}
	return ""
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if path == "/" {
		return "/"
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	return path
}

// LocalTokenClaims represents claims for backend-issued JWT
type LocalTokenClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

// APIJWTAuth enforces backend-issued JWT auth for all /api routes,
// except for explicitly allowed public paths.
func APIJWTAuth(publicPaths ...string) gin.HandlerFunc {
	public := make(map[string]struct{}, len(publicPaths))
	for _, p := range publicPaths {
		public[normalizePath(p)] = struct{}{}
	}

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		path := normalizePath(c.Request.URL.Path)
		if !strings.HasPrefix(path, "/api") {
			c.Next()
			return
		}

		if _, ok := public[path]; ok {
			c.Next()
			return
		}

		if !authenticateLocalJWT(c) {
			return
		}

		c.Next()
	}
}

func authenticateLocalJWT(c *gin.Context) bool {
	cfg := config.GetConfig()

	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization header required",
		})
		return false
	}

	// Parse Bearer token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid authorization header format",
		})
		return false
	}

	tokenString := parts[1]

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		jwtSecret = "default-secret-change-me"
	}

	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &LocalTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing algorithm
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": fmt.Sprintf("Invalid token: %v", err),
		})
		return false
	}

	// Extract claims and add to context
	claims, ok := token.Claims.(*LocalTokenClaims)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Failed to parse token claims",
		})
		return false
	}

	// Add user info to context
	c.Set("user_id", claims.Subject)
	c.Set("username", claims.Username)
	c.Set("email", claims.Email)
	c.Set("name", claims.Name)
	c.Set("claims", claims)

	return true
}

// LocalJWTAuth returns a Gin middleware that validates backend-issued JWT tokens
func LocalJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authenticateLocalJWT(c) {
			return
		}

		c.Next()
	}
}
