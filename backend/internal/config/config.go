package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	// Server settings
	Port string `mapstructure:"PORT"`

	// Database settings
	DatabaseURL string `mapstructure:"DATABASE_URL"`

	// Keycloak settings
	KeycloakURL          string `mapstructure:"KEYCLOAK_URL"`
	KeycloakRealm        string `mapstructure:"KEYCLOAK_REALM"`
	KeycloakClientID     string `mapstructure:"KEYCLOAK_CLIENT_ID"`
	KeycloakClientSecret string `mapstructure:"KEYCLOAK_CLIENT_SECRET"`

	// JWT settings (for backend-issued tokens)
	JWTSecret     string `mapstructure:"JWT_SECRET"`
	JWTExpireDays int    `mapstructure:"JWT_EXPIRE_DAYS"`

	// Feature flags
	EnableTrace bool `mapstructure:"ENABLE_TRACE"`

	// Bash sandbox settings
	BashRootDir string `mapstructure:"BASH_ROOT_DIR"`
}

var AppConfig *Config

// Load reads configuration from environment variables
func Load() (*Config, error) {
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("KEYCLOAK_REALM", "base-realm")
	viper.SetDefault("KEYCLOAK_CLIENT_ID", "base-app")
	viper.SetDefault("JWT_EXPIRE_DAYS", 7)
	viper.SetDefault("ENABLE_TRACE", false)
	viper.SetDefault("BASH_ROOT_DIR", "./bash-root")

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	config := &Config{}

	// Bind environment variables
	_ = viper.BindEnv("PORT")
	_ = viper.BindEnv("DATABASE_URL")
	_ = viper.BindEnv("KEYCLOAK_URL")
	_ = viper.BindEnv("KEYCLOAK_REALM")
	_ = viper.BindEnv("KEYCLOAK_CLIENT_ID")
	_ = viper.BindEnv("KEYCLOAK_CLIENT_SECRET")
	_ = viper.BindEnv("JWT_SECRET")
	_ = viper.BindEnv("JWT_EXPIRE_DAYS")
	_ = viper.BindEnv("ENABLE_TRACE")
	_ = viper.BindEnv("BASH_ROOT_DIR")

	if err := viper.Unmarshal(config); err != nil {
		return nil, err
	}

	AppConfig = config
	log.Printf("Configuration loaded: Port=%s, EnableTrace=%v", config.Port, config.EnableTrace)

	return config, nil
}

// GetConfig returns the current configuration
func GetConfig() *Config {
	if AppConfig == nil {
		cfg, err := Load()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}
		return cfg
	}
	return AppConfig
}
