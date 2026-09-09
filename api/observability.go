package api

import (
	"net/http"

	"nofx/metrics"
	"nofx/notification"
	"nofx/store"

	"github.com/gin-gonic/gin"
)

// audit records an audit event asynchronously so it never blocks the request.
// Failures are logged and counted, never propagated to the caller's response.
func (s *Server) audit(c *gin.Context, action, resourceType, resourceID, status, detail string) {
	s.auditRaw(c, action, resourceType, resourceID, status, detail)
}

// auditRaw is the audit recorder without user-context helpers.
func (s *Server) auditRaw(c *gin.Context, action, resourceType, resourceID, status, detail string) {
	if s.store == nil {
		return
	}
	userID := ""
	email := ""
	if v, ok := c.Get("user_id"); ok {
		userID, _ = v.(string)
	}
	if v, ok := c.Get("email"); ok {
		email, _ = v.(string)
	}
	ua := ""
	if c.Request != nil && c.Request.UserAgent() != "" {
		ua = c.Request.UserAgent()
		if len(ua) > 250 {
			ua = ua[:250]
		}
	}
	if len(detail) > 500 {
		detail = detail[:500]
	}

	e := &store.AuditEvent{
		UserID:       userID,
		Email:        email,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Detail:       detail,
		Status:       status,
		IP:           clientIP(c),
		UserAgent:    ua,
	}
	go func() {
		if err := s.store.Audit().Record(e); err != nil {
			metrics.AuditEventsTotal.Inc() // count attempted writes; Record failed
		} else {
			metrics.AuditEventsTotal.Inc()
		}
	}()
}

// handleGetAuditLogs returns audit events for the current user.
// GET /api/audit-logs?limit=&offset=&action=
func (s *Server) handleGetAuditLogs(c *gin.Context) {
	userID := c.GetString("user_id")
	limit := intQueryDefault(c, "limit", 100)
	offset := intQueryDefault(c, "offset", 0)
	action := c.Query("action")

	events, err := s.store.Audit().List(store.AuditQuery{
		UserID: userID,
		Action: action,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}

// handleGetNotifications returns the user's in-app notification history.
// GET /api/notifications?limit=&offset=
func (s *Server) handleGetNotifications(c *gin.Context) {
	userID := c.GetString("user_id")
	limit := intQueryDefault(c, "limit", 50)
	offset := intQueryDefault(c, "offset", 0)

	items, err := s.store.Notification().List(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load notifications"})
		return
	}
	unread, _ := s.store.Notification().CountUnread(userID)
	c.JSON(http.StatusOK, gin.H{"notifications": items, "unread": unread})
}

// handleMarkNotificationRead marks one notification as read.
// POST /api/notifications/:id/read
func (s *Server) handleMarkNotificationRead(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := s.store.Notification().MarkRead(userID, c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// handleMarkAllNotificationsRead marks every notification as read.
// POST /api/notifications/read-all
func (s *Server) handleMarkAllNotificationsRead(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := s.store.Notification().MarkAllRead(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notifications"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// handleGetNotificationSettings returns the user's channel configuration.
// Credentials are masked so secrets never round-trip to the browser.
// GET /api/notification-settings
func (s *Server) handleGetNotificationSettings(c *gin.Context) {
	userID := c.GetString("user_id")
	st, err := s.store.Notification().GetSettings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": maskSettings(st)})
}

// handleUpdateNotificationSettings saves channel configuration.
// PUT /api/notification-settings
func (s *Server) handleUpdateNotificationSettings(c *gin.Context) {
	userID := c.GetString("user_id")

	var req store.NotificationSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = userID

	// Load existing settings to preserve credentials the client masked.
	existing, err := s.store.Notification().GetSettings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}
	if isMasked(req.TelegramBotToken) {
		req.TelegramBotToken = existing.TelegramBotToken
	}
	if isMasked(req.SMTPPassword) {
		req.SMTPPassword = existing.SMTPPassword
	}
	if req.TelegramBotToken == "" && req.TelegramEnabled {
		req.TelegramBotToken = existing.TelegramBotToken
	}
	if req.SMTPPassword == "" && req.EmailEnabled {
		req.SMTPPassword = existing.SMTPPassword
	}
	if req.WebhookSecret == "" && req.WebhookEnabled {
		req.WebhookSecret = existing.WebhookSecret
	}

	if err := s.store.Notification().SaveSettings(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
		return
	}
	s.audit(c, store.AuditActionNotificationUpdate, "notification_settings", userID, "success", "notification settings updated")
	c.JSON(http.StatusOK, gin.H{"settings": maskSettings(&req)})
}

// handleTestNotification sends a test notification through enabled channels.
// POST /api/notification-settings/test
func (s *Server) handleTestNotification(c *gin.Context) {
	userID := c.GetString("user_id")
	svc := notification.Default()
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Notification service unavailable"})
		return
	}
	st, err := s.store.Notification().GetSettings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}
	if err := svc.SendTest(userID, st); err != nil {
		s.audit(c, store.AuditActionNotificationTest, "notification", userID, "failure", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Test delivery failed: " + err.Error()})
		return
	}
	s.audit(c, store.AuditActionNotificationTest, "notification", userID, "success", "test notification sent")
	c.JSON(http.StatusOK, gin.H{"message": "Test notification dispatched to enabled channels"})
}

// handleMetrics exposes Prometheus metrics. Auth model: token via env, else
// any authenticated user.
// GET /api/metrics
func (s *Server) handleMetrics(c *gin.Context) {
	cfgToken := s.metricsToken
	if cfgToken != "" {
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				token = authHeader[7:]
			}
		}
		if token != cfgToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid metrics token"})
			return
		}
		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(metrics.Default().Gather()))
		return
	}
	// No token configured: fall through to the protected group's auth.
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(metrics.Default().Gather()))
}

// handleTriggerBackup runs a backup immediately.
// POST /api/backups
func (s *Server) handleTriggerBackup(c *gin.Context) {
	if s.backupSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Backup service not configured"})
		return
	}
	path, err := s.backupSvc.RunNow("manual")
	status := "success"
	detail := "manual backup: " + path
	if err != nil {
		status = "failure"
		detail = "manual backup failed: " + err.Error()
	}
	s.audit(c, store.AuditActionBackupRun, "backup", path, status, detail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Backup created", "path": path})
}

// handleListBackups returns recent backup history.
// GET /api/backups
func (s *Server) handleListBackups(c *gin.Context) {
	if s.backupSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Backup service not configured"})
		return
	}
	backups, err := s.backupSvc.ListBackups(20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list backups"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"backups": backups})
}

// auditIdentified records an audit event for unauthenticated endpoints where
// the user identity comes from the request payload (login attempts etc.).
func (s *Server) auditIdentified(c *gin.Context, email, action, resourceType, resourceID, status, detail string) {
	if s.store == nil {
		return
	}
	ua := ""
	if c.Request != nil {
		ua = c.Request.UserAgent()
		if len(ua) > 250 {
			ua = ua[:250]
		}
	}
	if len(detail) > 500 {
		detail = detail[:500]
	}
	e := &store.AuditEvent{
		Email:        email,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Detail:       detail,
		Status:       status,
		IP:           clientIP(c),
		UserAgent:    ua,
	}
	go func() {
		if err := s.store.Audit().Record(e); err == nil {
			metrics.AuditEventsTotal.Inc()
		}
	}()
}

// handleCryptoDecryptAudited wraps the crypto decrypt endpoint with an
// audit entry: credential decryption is the most security-sensitive public
// endpoint and must always be traceable.
func (s *Server) handleCryptoDecryptAudited(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		// Audit without a known user: use client IP only.
		email = "unknown"
	}
	defer func() {
		status := "success"
		if c.Writer.Status() >= 400 {
			status = "failure"
		}
		s.auditRaw(c, store.AuditActionCryptoDecrypt, "crypto", "sensitive-data", status, "decrypt requested for "+email)
	}()
	s.cryptoHandler.HandleDecryptSensitiveData(c)
}

// maskSecret replaces a secret with a fixed-length mask marker.
func maskSettings(st *store.NotificationSettings) *store.NotificationSettings {
	out := *st
	if out.TelegramBotToken != "" {
		out.TelegramBotToken = masked
	}
	if out.SMTPPassword != "" {
		out.SMTPPassword = masked
	}
	return &out
}

const masked = "••••••••"

func isMasked(v string) bool { return v == masked }

func intQueryDefault(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		n := 0
		for _, r := range v {
			if r < '0' || r > '9' {
				return def
			}
			n = n*10 + int(r-'0')
		}
		return n
	}
	return def
}
