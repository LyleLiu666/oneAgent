// Package model contains database models for user settings.
package model

import (
	"time"

	"gorm.io/gorm"
)

// UserSettings stores user-specific configuration values.
// Uses a key-value pattern to support extensible settings.
type UserSettings struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    string         `gorm:"type:varchar(255);uniqueIndex:idx_user_key,priority:1" json:"user_id"`
	Key       string         `gorm:"type:varchar(100);uniqueIndex:idx_user_key,priority:2" json:"key"`
	Value     string         `gorm:"type:text" json:"-"` // Sensitive data not returned in JSON
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for UserSettings.
func (UserSettings) TableName() string {
	return "user_settings"
}

// Known setting keys
const (
	SettingKeyBochaAPIKey = "bocha_api_key"
)

// GetUserSetting retrieves a single setting value for a user.
func GetUserSetting(db *gorm.DB, userID, key string) (string, error) {
	var setting UserSettings
	err := db.Where("user_id = ? AND key = ?", userID, key).First(&setting).Error
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// SetUserSetting creates or updates a user setting.
func SetUserSetting(db *gorm.DB, userID, key, value string) error {
	var setting UserSettings
	err := db.Where("user_id = ? AND key = ?", userID, key).First(&setting).Error
	if err == gorm.ErrRecordNotFound {
		// Create new setting
		setting = UserSettings{
			UserID: userID,
			Key:    key,
			Value:  value,
		}
		return db.Create(&setting).Error
	} else if err != nil {
		return err
	}
	// Update existing setting
	setting.Value = value
	return db.Save(&setting).Error
}

// GetUserSettingsMap retrieves all settings for a user as a map.
// Returns a map with "has_<key>" booleans for sensitive values.
func GetUserSettingsMap(db *gorm.DB, userID string) (map[string]interface{}, error) {
	var settings []UserSettings
	err := db.Where("user_id = ?", userID).Find(&settings).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for _, s := range settings {
		// For API keys, return whether they are set, not the actual value
		if s.Key == SettingKeyBochaAPIKey {
			result["has_bocha_api_key"] = s.Value != ""
		} else {
			result[s.Key] = s.Value
		}
	}
	return result, nil
}
