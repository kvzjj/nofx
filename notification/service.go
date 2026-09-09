package notification

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
	"sync"
	"time"

	"nofx/logger"
	"nofx/store"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Service fans out events to user channels and records in-app history.
type Service struct {
	store *store.Store

	// cached Telegram bots keyed by token (bot API object is stateless per
	// token, reuse avoids re-auth on every event)
	tgMu   sync.Mutex
	tgBots map[string]*tgbotapi.BotAPI

	httpClient *http.Client
}

var defaultService *Service

// Init wires the global service to the store and subscribes it to the bus.
// Call once after store initialization.
func Init(st *store.Store) {
	defaultService = &Service{
		store:    st,
		tgBots:   map[string]*tgbotapi.BotAPI{},
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	Subscribe(defaultService.handleEvent)
}

// Default returns the global service (nil before Init).
func Default() *Service { return defaultService }

func timeNowUTC() time.Time { return time.Now().UTC() }

// handleEvent is the bus subscriber driving delivery.
func (s *Service) handleEvent(e *Event) {
	if s == nil || s.store == nil {
		return
	}

	// Resolve target users: explicit user, or all users for broadcasts.
	userIDs := []string{}
	if e.UserID != "" {
		userIDs = []string{e.UserID}
	} else {
		ids, err := s.store.User().GetAllIDs()
		if err != nil {
			logger.Errorf("notification: list users failed: %v", err)
			return
		}
		userIDs = ids
	}

	for _, uid := range userIDs {
		s.deliverToUser(uid, e)
	}
}

// deliverToUser applies the user's preferences and pushes to each channel.
func (s *Service) deliverToUser(userID string, e *Event) {
	settings, err := s.store.Notification().GetSettings(userID)
	if err != nil {
		logger.Errorf("notification: load settings for %s failed: %v", userID, err)
		return
	}
	if !settings.Enabled {
		return
	}
	// Per-event subscription: an explicit false disables, absent = enabled.
	if enabled, ok := settings.EventSubscriptions[e.EventType]; ok && !enabled {
		return
	}
	if !withinAllowedHours(settings, e) {
		return
	}

	// In-app history is always recorded (even if quiet hours muted pushes,
	// this branch is reached only when allowed; critical events always pass).
	if err := s.store.Notification().Insert(&store.NotificationRecord{
		UserID:     userID,
		TraderID:   e.TraderID,
		TraderName: e.TraderName,
		EventType:  e.EventType,
		Severity:   string(e.Severity),
		Title:      e.Title,
		Body:       e.Body,
		CreatedAt:  e.Time,
	}); err != nil {
		logger.Errorf("notification: insert record failed: %v", err)
	}

	if settings.TelegramEnabled {
		if err := s.sendTelegram(settings, e); err != nil {
			logger.Errorf("notification: telegram delivery failed: %v", err)
		} else {
			metricsIncSent()
		}
	}
	if settings.WebhookEnabled && settings.WebhookURL != "" {
		if err := s.sendWebhook(settings, e); err != nil {
			logger.Errorf("notification: webhook delivery failed: %v", err)
		} else {
			metricsIncSent()
		}
	}
	if settings.EmailEnabled && settings.EmailTo != "" && settings.SMTPHost != "" {
		if err := s.sendEmail(settings, e); err != nil {
			logger.Errorf("notification: email delivery failed: %v", err)
		} else {
			metricsIncSent()
		}
	}
}

// withinAllowedHours enforces quiet hours. Critical events always pass.
func withinAllowedHours(st *store.NotificationSettings, e *Event) bool {
	if e.IsCritical() || st.QuietHoursStart < 0 || st.QuietHoursEnd < 0 {
		return true
	}
	hour := time.Now().UTC().Hour()
	start, end := st.QuietHoursStart, st.QuietHoursEnd
	if start == end {
		return true
	}
	inQuiet := false
	if start < end {
		inQuiet = hour >= start && hour < end
	} else { // wraps midnight (e.g. 22 -> 6)
		inQuiet = hour >= start || hour < end
	}
	return !inQuiet
}

func (s *Service) telegramBot(token string) (*tgbotapi.BotAPI, error) {
	s.tgMu.Lock()
	defer s.tgMu.Unlock()
	if bot, ok := s.tgBots[token]; ok {
		return bot, nil
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegram bot init: %w", err)
	}
	bot.Debug = false
	if len(s.tgBots) > 32 { // simple cap to avoid unbounded cache growth
		s.tgBots = map[string]*tgbotapi.BotAPI{}
	}
	s.tgBots[token] = bot
	return bot, nil
}

func (s *Service) sendTelegram(st *store.NotificationSettings, e *Event) error {
	if st.TelegramBotToken == "" || st.TelegramChatID == "" {
		return fmt.Errorf("telegram credentials incomplete")
	}
	bot, err := s.telegramBot(st.TelegramBotToken)
	if err != nil {
		return err
	}
	var chatID int64
	if _, err := fmt.Sscanf(st.TelegramChatID, "%d", &chatID); err != nil {
		return fmt.Errorf("invalid telegram chat id %q: %w", st.TelegramChatID, err)
	}
	text := formatMessage(e, true)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	_, err = bot.Send(msg)
	return err
}

func (s *Service) sendWebhook(st *store.NotificationSettings, e *Event) error {
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, st.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-NOFX-Event", e.EventType)
	if st.WebhookSecret != "" {
		mac := hmac.New(sha256.New, []byte(st.WebhookSecret))
		mac.Write(payload)
		req.Header.Set("X-NOFX-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %s", resp.Status)
	}
	return nil
}

func (s *Service) sendEmail(st *store.NotificationSettings, e *Event) error {
	addr := fmt.Sprintf("%s:%d", st.SMTPHost, st.SMTPPort)
	auth := smtp.PlainAuth("", st.SMTPUsername, st.SMTPPassword, st.SMTPHost)

	subject := fmt.Sprintf("[NOFX][%s] %s", strings.ToUpper(string(e.Severity)), e.Title)
	body := formatMessage(e, false)
	content := "From: " + st.SMTPFrom + "\r\n" +
		"To: " + st.EmailTo + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body

	return smtp.SendMail(addr, auth, st.SMTPFrom, []string{st.EmailTo}, []byte(content))
}

// formatMessage renders a human-readable message for push channels.
func formatMessage(e *Event, html bool) string {
	emoji := map[Severity]string{
		SeverityInfo:     "ℹ️",
		SeverityWarning:  "⚠️",
		SeverityCritical: "🚨",
	}[e.Severity]

	var b strings.Builder
	if html {
		fmt.Fprintf(&b, "%s <b>%s</b>\n", emoji, escapeHTML(e.Title))
	} else {
		fmt.Fprintf(&b, "%s %s\n", emoji, e.Title)
	}
	if e.Body != "" {
		b.WriteString(e.Body)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	var meta []string
	if e.TraderName != "" {
		meta = append(meta, "Trader: "+e.TraderName)
	}
	if e.Symbol != "" {
		meta = append(meta, "Symbol: "+e.Symbol)
	}
	meta = append(meta, "Time: "+e.Time.Format("2006-01-02 15:04:05 UTC"))
	b.WriteString(strings.Join(meta, " | "))
	return b.String()
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// SendTest delivers a test notification through the user's enabled channels
// without touching the event bus.
func (s *Service) SendTest(userID string, st *store.NotificationSettings) error {
	e := &Event{
		EventType: EventSystem,
		Severity:  SeverityInfo,
		UserID:    userID,
		Title:     "NOFX 测试通知 / Test notification",
		Body:      "如果你收到这条消息，说明该渠道配置正确。/ If you received this, the channel works.",
		Time:      timeNowUTC(),
	}
	if err := s.store.Notification().Insert(&store.NotificationRecord{
		UserID: userID, EventType: e.EventType, Severity: string(e.Severity),
		Title: e.Title, Body: e.Body, CreatedAt: e.Time,
	}); err != nil {
		logger.Errorf("notification: insert test record failed: %v", err)
	}
	var errs []string
	if st.TelegramEnabled {
		if err := s.sendTelegram(st, e); err != nil {
			errs = append(errs, "telegram: "+err.Error())
		}
	}
	if st.WebhookEnabled && st.WebhookURL != "" {
		if err := s.sendWebhook(st, e); err != nil {
			errs = append(errs, "webhook: "+err.Error())
		}
	}
	if st.EmailEnabled && st.EmailTo != "" && st.SMTPHost != "" {
		if err := s.sendEmail(st, e); err != nil {
			errs = append(errs, "email: "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
