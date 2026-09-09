package store

import (
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("open memory store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestAuditRecordAndList(t *testing.T) {
	st := newTestStore(t)

	err := st.Audit().Record(&AuditEvent{
		UserID: "u1", Email: "a@b.c", Action: AuditActionLogin,
		ResourceType: "user", ResourceID: "u1", Status: "success",
		IP: "1.2.3.4", Detail: "ok",
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	events, err := st.Audit().List(AuditQuery{UserID: "u1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len = %d, want 1", len(events))
	}
	e := events[0]
	if e.Action != AuditActionLogin || e.IP != "1.2.3.4" || e.Status != "success" {
		t.Fatalf("unexpected event: %+v", e)
	}
	if e.ID == "" || e.CreatedAt.IsZero() {
		t.Fatal("id/created_at should be auto-filled")
	}
}

func TestAuditListFiltersByAction(t *testing.T) {
	st := newTestStore(t)
	for _, action := range []string{AuditActionLogin, AuditActionLoginFailed, AuditActionLogin} {
		if err := st.Audit().Record(&AuditEvent{UserID: "u", Action: action}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	events, err := st.Audit().List(AuditQuery{UserID: "u", Action: AuditActionLoginFailed})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len = %d, want 1", len(events))
	}
}

func TestAuditPruneBefore(t *testing.T) {
	st := newTestStore(t)
	if err := st.Audit().Record(&AuditEvent{UserID: "u", Action: AuditActionLogin}); err != nil {
		t.Fatalf("record: %v", err)
	}
	n, err := st.Audit().PruneBefore(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Fatalf("pruned = %d, want 1", n)
	}
	events, _ := st.Audit().List(AuditQuery{UserID: "u"})
	if len(events) != 0 {
		t.Fatalf("events after prune = %d, want 0", len(events))
	}
}

func TestNotificationSettingsRoundTrip(t *testing.T) {
	st := newTestStore(t)

	// Default creation on first read.
	def, err := st.Notification().GetSettings("user-1")
	if err != nil {
		t.Fatalf("get default: %v", err)
	}
	if !def.Enabled || def.TelegramEnabled {
		t.Fatalf("unexpected defaults: %+v", def)
	}

	def.TelegramEnabled = true
	def.TelegramBotToken = "123:ABC"
	def.TelegramChatID = "-100200"
	def.EventSubscriptions = map[string]bool{"position.opened": false}
	def.QuietHoursStart = 22
	def.QuietHoursEnd = 6
	if err := st.Notification().SaveSettings(def); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.Notification().GetSettings("user-1")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !got.TelegramEnabled || got.TelegramBotToken != "123:ABC" || got.TelegramChatID != "-100200" {
		t.Fatalf("telegram settings lost: %+v", got)
	}
	if got.QuietHoursStart != 22 || got.QuietHoursEnd != 6 {
		t.Fatalf("quiet hours lost: %+v", got)
	}
	if v, ok := got.EventSubscriptions["position.opened"]; !ok || v {
		t.Fatalf("event subscriptions lost: %+v", got.EventSubscriptions)
	}
}

func TestNotificationRecordsAndUnread(t *testing.T) {
	st := newTestStore(t)
	for i := 0; i < 3; i++ {
		if err := st.Notification().Insert(&NotificationRecord{
			UserID: "u", EventType: "position.opened", Severity: "info",
			Title: "t", Body: "b",
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	unread, err := st.Notification().CountUnread("u")
	if err != nil || unread != 3 {
		t.Fatalf("unread = %d err=%v, want 3", unread, err)
	}
	list, err := st.Notification().List("u", 10, 0)
	if err != nil || len(list) != 3 {
		t.Fatalf("list = %d err=%v", len(list), err)
	}
	if err := st.Notification().MarkRead("u", list[0].ID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	unread, _ = st.Notification().CountUnread("u")
	if unread != 2 {
		t.Fatalf("unread after mark = %d, want 2", unread)
	}
	if err := st.Notification().MarkAllRead("u"); err != nil {
		t.Fatalf("mark all: %v", err)
	}
	unread, _ = st.Notification().CountUnread("u")
	if unread != 0 {
		t.Fatalf("unread after mark-all = %d, want 0", unread)
	}
}

func TestTokenBlacklistPersistence(t *testing.T) {
	st := newTestStore(t)
	tb := st.TokenBlacklist()

	hash := "abc123"
	if revoked, err := tb.IsTokenBlacklisted(hash); err != nil || revoked {
		t.Fatalf("empty blacklist flagged revoked=%v err=%v", revoked, err)
	}

	if err := tb.BlacklistToken(hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("blacklist: %v", err)
	}
	if revoked, err := tb.IsTokenBlacklisted(hash); err != nil || !revoked {
		t.Fatalf("revoked=%v err=%v, want true", revoked, err)
	}

	// Expired entries are lazily dropped.
	if err := tb.BlacklistToken("expired", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("blacklist expired: %v", err)
	}
	if revoked, _ := tb.IsTokenBlacklisted("expired"); revoked {
		t.Fatal("expired entry must not report revoked")
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	st := newTestStore(t)
	// Running migrations twice (New already ran them once) must not fail.
	if err := RunMigrations(st.DB()); err != nil {
		t.Fatalf("second run: %v", err)
	}
	vs := MigrationVersions(st.DB())
	if len(vs) == 0 {
		t.Fatal("no migration versions recorded")
	}
	for i, v := range vs {
		if v != i+1 {
			t.Fatalf("versions not contiguous: %v", vs)
		}
	}
}
