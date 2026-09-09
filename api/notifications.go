package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"nofx/logger"
	"nofx/notify"
	"nofx/store"

	"github.com/gin-gonic/gin"
)

// notificationConfigFields describes per-type channel config keys.
// Secrets are masked when returning channels to the frontend.
var notificationConfigFields = map[string][]string{
	"telegram": {"bot_token", "chat_id"},
	"webhook":  {"url", "secret"},
	"email":    {"smtp_host", "smtp_port", "smtp_user", "smtp_pass", "from", "to"},
}

var notificationSecretFields = map[string]bool{
	"bot_token": true,
	"secret":    true,
	"smtp_pass": true,
	"smtp_user": true,
}

type notificationChannelRequest struct {
	Name    string          `json:"name" binding:"required"`
	Type    string          `json:"type" binding:"required"`
	Config  json.RawMessage `json:"config" binding:"required"`
	Enabled *bool           `json:"enabled"`
}

// validateChannelConfig ensures the raw config JSON contains exactly the
// required fields for the channel type and returns the canonical string.
func validateChannelConfig(channelType string, raw json.RawMessage) (string, error) {
	fields, ok := notificationConfigFields[channelType]
	if !ok {
		return "", errInvalidChannelType
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", errInvalidConfigJSON
	}
	clean := make(map[string]interface{}, len(fields))
	for _, field := range fields {
		v, exists := cfg[field]
		if !exists {
			continue
		}
		if str, ok := v.(string); ok {
			clean[field] = str
		}
	}
	// Required fields per type
	required := map[string][]string{
		"telegram": {"bot_token", "chat_id"},
		"webhook":  {"url"},
		"email":    {"smtp_host", "smtp_port", "from", "to"},
	}
	for _, req := range required[channelType] {
		if s, _ := clean[req].(string); strings.TrimSpace(s) == "" {
			return "", &missingConfigFieldError{field: req}
		}
	}
	out, err := json.Marshal(clean)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

var (
	errInvalidChannelType = &validationError{msg: "invalid channel type (telegram/webhook/email)"}
	errInvalidConfigJSON  = &validationError{msg: "invalid config JSON"}
	errChannelNotFound    = &validationError{msg: "channel not found"}
)

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

type missingConfigFieldError struct{ field string }

func (e *missingConfigFieldError) Error() string {
	return "missing required config field: " + e.field
}

// maskChannelConfig parses stored config JSON and masks secret fields.
func maskChannelConfig(config string) map[string]interface{} {
	result := map[string]interface{}{}
	if strings.TrimSpace(config) == "" {
		return result
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return result
	}
	for k, v := range cfg {
		if s, ok := v.(string); ok && notificationSecretFields[k] && s != "" {
			result[k] = MaskSensitiveString(s)
		} else {
			result[k] = v
		}
	}
	return result
}

func channelToJSON(ch *store.NotificationChannel) gin.H {
	return gin.H{
		"id":         ch.ID,
		"name":       ch.Name,
		"type":       ch.Type,
		"enabled":    ch.Enabled,
		"config":     maskChannelConfig(ch.Config),
		"created_at": ch.CreatedAt,
		"updated_at": ch.UpdatedAt,
	}
}

// handleListNotificationChannels returns the user's channels (secrets masked).
func (s *Server) handleListNotificationChannels(c *gin.Context) {
	userID := c.GetString("user_id")
	channels, err := s.store.Notification().ListChannels(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]gin.H, 0, len(channels))
	for _, ch := range channels {
		result = append(result, channelToJSON(ch))
	}
	c.JSON(http.StatusOK, result)
}

// handleCreateNotificationChannel creates a channel.
func (s *Server) handleCreateNotificationChannel(c *gin.Context) {
	userID := c.GetString("user_id")
	var req notificationChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, type and config are required"})
		return
	}
	configStr, err := validateChannelConfig(req.Type, req.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	ch, err := s.store.Notification().CreateChannel(userID, req.Name, req.Type, configStr, enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	logger.Infof("🔔 User %s created notification channel %s (%s)", userID, req.Name, req.Type)
	c.JSON(http.StatusOK, channelToJSON(ch))
}

// handleUpdateNotificationChannel updates a channel. Masked secret values
// ("abcd****wxyz") are ignored so the stored secret is preserved.
func (s *Server) handleUpdateNotificationChannel(c *gin.Context) {
	userID := c.GetString("user_id")
	channelID := c.Param("id")

	existing, err := s.store.Notification().GetChannel(userID, channelID)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errChannelNotFound.Error()})
		return
	}

	var req notificationChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, type and config are required"})
		return
	}

	// Merge: keep stored secrets when the request masks them.
	var incoming map[string]interface{}
	if err := json.Unmarshal(req.Config, &incoming); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidConfigJSON.Error()})
		return
	}
	var stored map[string]interface{}
	_ = json.Unmarshal([]byte(existing.Config), &stored)
	for _, field := range notificationConfigFields[req.Type] {
		if v, ok := incoming[field].(string); ok && strings.Contains(v, "****") {
			if storedVal, ok := stored[field].(string); ok {
				incoming[field] = storedVal
			}
		}
	}
	merged, _ := json.Marshal(incoming)

	configStr, err := validateChannelConfig(req.Type, merged)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	ch, err := s.store.Notification().UpdateChannel(userID, channelID, req.Name, req.Type, configStr, enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, channelToJSON(ch))
}

// handleDeleteNotificationChannel deletes a channel.
func (s *Server) handleDeleteNotificationChannel(c *gin.Context) {
	userID := c.GetString("user_id")
	channelID := c.Param("id")
	if err := s.store.Notification().DeleteChannel(userID, channelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// handleTestNotificationChannel sends a test message through the channel.
func (s *Server) handleTestNotificationChannel(c *gin.Context) {
	userID := c.GetString("user_id")
	channelID := c.Param("id")
	service := notify.Default()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "notification service unavailable"})
		return
	}
	if err := service.SendTest(userID, channelID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "test notification sent"})
}

// handleNotificationLogs returns recent delivery logs.
func (s *Server) handleNotificationLogs(c *gin.Context) {
	userID := c.GetString("user_id")
	limit := 50
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 200 {
		limit = v
	}
	logs, err := s.store.Notification().ListLogs(userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if logs == nil {
		logs = []*store.NotificationLog{}
	}
	c.JSON(http.StatusOK, logs)
}
