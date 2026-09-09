// Package notify delivers trading event notifications to user-configured
// channels (Telegram bot, generic Webhook, SMTP email) and persists delivery
// logs. Sending is asynchronous: callers never block the trading loop.
package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"nofx/logger"
	"nofx/store"
)

// Service fans out notifications to all enabled channels of a user.
type Service struct {
	store  *store.Store
	client *http.Client
}

var defaultService *Service

// Init wires the global service (called once at bootstrap).
func Init(st *store.Store) {
	defaultService = &Service{
		store:  st,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Default returns the global service (nil-safe: returns nil before Init).
func Default() *Service {
	return defaultService
}

// Event type constants used by trading hooks.
const (
	EventTradeOpened   = "trade_opened"
	EventTradeClosed   = "trade_closed"
	EventTradeFailed   = "trade_failed"
	EventTraderStopped = "trader_stopped"
	EventTest          = "test"
)

// Notify asynchronously delivers an event to all enabled channels of the user.
func (s *Service) Notify(userID, event, title, message string) {
	if s == nil || userID == "" {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Infof("⚠️ notification panic recovered: %v", r)
			}
		}()
		channels, err := s.store.Notification().ListChannels(userID)
		if err != nil {
			logger.Infof("⚠️ failed to load notification channels: %v", err)
			return
		}
		for _, ch := range channels {
			if !ch.Enabled {
				continue
			}
			var sendErr error
			switch ch.Type {
			case "telegram":
				sendErr = s.sendTelegram(ch, title, message)
			case "webhook":
				sendErr = s.sendWebhook(ch, event, title, message)
			case "email":
				sendErr = s.sendEmail(ch, title, message)
			default:
				sendErr = fmt.Errorf("unknown channel type: %s", ch.Type)
			}
			logEntry := &store.NotificationLog{
				UserID:      userID,
				ChannelID:   ch.ID,
				ChannelName: ch.Name,
				ChannelType: ch.Type,
				Event:       event,
				Title:       title,
				Message:     message,
				Success:     sendErr == nil,
			}
			if sendErr != nil {
				logEntry.Error = sendErr.Error()
			}
			_ = s.store.Notification().InsertLog(logEntry)
		}
	}()
}

// NotifyGlobal is a nil-safe helper for packages holding no service reference.
func NotifyGlobal(userID, event, title, message string) {
	if defaultService != nil {
		defaultService.Notify(userID, event, title, message)
	}
}

func channelConfig(ch *store.NotificationChannel) map[string]string {
	raw := ch.Config
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(parsed))
	for k, v := range parsed {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}

// sendTelegram posts a message via the Bot API.
func (s *Service) sendTelegram(ch *store.NotificationChannel, title, message string) error {
	cfg := channelConfig(ch)
	botToken := cfg["bot_token"]
	chatID := cfg["chat_id"]
	if botToken == "" || chatID == "" {
		return fmt.Errorf("telegram channel requires bot_token and chat_id")
	}
	payload := map[string]string{
		"chat_id": chatID,
		"text":    fmt.Sprintf("*%s*\n%s", title, message),
		// Plain text with markdown bold title; avoid full parse_mode to keep
		// arbitrary messages (symbols, underscores) delivery-safe.
	}
	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(
		"https://api.telegram.org/bot"+botToken+"/sendMessage",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram api status %d", resp.StatusCode)
	}
	return nil
}

// sendWebhook POSTs a JSON payload; if a secret is configured the body is
// signed with HMAC-SHA256 in the X-Nofx-Signature header.
func (s *Service) sendWebhook(ch *store.NotificationChannel, event, title, message string) error {
	cfg := channelConfig(ch)
	url := cfg["url"]
	if url == "" {
		return fmt.Errorf("webhook channel requires url")
	}
	payload := map[string]interface{}{
		"event":     event,
		"title":     title,
		"message":   message,
		"channel":   ch.Name,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret := cfg["secret"]; secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		req.Header.Set("X-Nofx-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}

// sendEmail delivers via user-provided SMTP credentials.
func (s *Service) sendEmail(ch *store.NotificationChannel, title, message string) error {
	cfg := channelConfig(ch)
	host := cfg["smtp_host"]
	port := cfg["smtp_port"]
	user := cfg["smtp_user"]
	pass := cfg["smtp_pass"]
	from := cfg["from"]
	to := cfg["to"]
	if host == "" || port == "" || from == "" || to == "" {
		return fmt.Errorf("email channel requires smtp_host, smtp_port, from, to")
	}
	addr := host + ":" + port

	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: [NOFX] " + title,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		message,
	}, "\r\n")

	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

// SendTest delivers a test message through one channel synchronously and
// returns the error (used by the API test button).
func (s *Service) SendTest(userID, channelID string) error {
	ch, err := s.store.Notification().GetChannel(userID, channelID)
	if err != nil || ch == nil {
		return fmt.Errorf("channel not found")
	}
	var sendErr error
	switch ch.Type {
	case "telegram":
		sendErr = s.sendTelegram(ch, "NOFX Test", "✅ This is a test notification from NOFX.")
	case "webhook":
		sendErr = s.sendWebhook(ch, EventTest, "NOFX Test", "✅ This is a test notification from NOFX.")
	case "email":
		sendErr = s.sendEmail(ch, "NOFX Test", "✅ This is a test notification from NOFX.")
	default:
		sendErr = fmt.Errorf("unknown channel type: %s", ch.Type)
	}
	logEntry := &store.NotificationLog{
		UserID:      userID,
		ChannelID:   ch.ID,
		ChannelName: ch.Name,
		ChannelType: ch.Type,
		Event:       EventTest,
		Title:       "NOFX Test",
		Message:     "Test notification",
		Success:     sendErr == nil,
	}
	if sendErr != nil {
		logEntry.Error = sendErr.Error()
	}
	_ = s.store.Notification().InsertLog(logEntry)
	return sendErr
}
