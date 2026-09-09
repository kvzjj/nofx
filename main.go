package main

import (
	"nofx/api"
	"nofx/auth"
	"nofx/backtest"
	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/manager"
	"nofx/market"
	"nofx/mcp"
	"nofx/metrics"
	"nofx/notification"
	"nofx/notify"
	"nofx/store"
	"nofx/trader"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env environment variables
	_ = godotenv.Load()

	// Initialize logger
	logger.Init(nil)

	logger.Info("╔════════════════════════════════════════════════════════════╗")
	logger.Info("║    🤖 AI Multi-Model Trading System - DeepSeek & Qwen      ║")
	logger.Info("╚════════════════════════════════════════════════════════════╝")

	// Initialize global configuration (loaded from .env)
	config.Init()
	cfg := config.Get()
	logger.Info("✅ Configuration loaded")

	// Initialize database
	// Default path is data/data.db to work with Docker volume mount (/app/data)
	dbPath := "data/data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	// Ensure data directory exists
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Errorf("Failed to create data directory: %v", err)
		}
	}

	logger.Infof("📋 Initializing database: %s", dbPath)
	st, err := store.New(dbPath)
	if err != nil {
		logger.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer st.Close()
	backtest.UseDatabase(st.DB())

	// Initialize encryption service
	logger.Info("🔐 Initializing encryption service...")
	cryptoService, err := crypto.NewCryptoService()
	if err != nil {
		logger.Fatalf("❌ Failed to initialize encryption service: %v", err)
	}
	encryptFunc := func(plaintext string) string {
		if plaintext == "" {
			return plaintext
		}
		encrypted, err := cryptoService.EncryptForStorage(plaintext)
		if err != nil {
			logger.Warnf("⚠️ Encryption failed: %v", err)
			return plaintext
		}
		return encrypted
	}
	decryptFunc := func(encrypted string) string {
		if encrypted == "" {
			return encrypted
		}
		if !cryptoService.IsEncryptedStorageValue(encrypted) {
			return encrypted
		}
		decrypted, err := cryptoService.DecryptFromStorage(encrypted)
		if err != nil {
			logger.Warnf("⚠️ Decryption failed: %v", err)
			return encrypted
		}
		return decrypted
	}
	st.SetCryptoFuncs(encryptFunc, decryptFunc)
	logger.Info("✅ Encryption service initialized successfully")

	// Initialize notification service (Telegram / Webhook / Email)
	notify.Init(st)
	logger.Info("✅ Notification service initialized")

	// Set JWT secret
	auth.SetJWTSecret(cfg.JWTSecret)
	logger.Info("🔑 JWT secret configured")

	// P1: persist the token blacklist so logouts survive restarts
	auth.SetTokenStore(st.TokenBlacklist())
	logger.Info("🔐 Persistent token blacklist enabled")

	// P0: initialize the notification service (in-app + Telegram/webhook/email)
	notification.Init(st)
	logger.Info("🔔 Notification service initialized")

	// P1: scheduled database backups (VACUUM INTO + retention pruning)
	backupSvc := store.NewBackupService(st.DB(), store.BackupConfig{
		Enabled:        cfg.BackupEnabled,
		Dir:            cfg.BackupDir,
		Interval:       time.Duration(cfg.BackupIntervalHours) * time.Hour,
		RetentionCount: cfg.BackupRetentionCount,
	})
	backupSvc.Start()
	defer backupSvc.Stop()
	if cfg.BackupEnabled {
		logger.Infof("🗄️  Scheduled backups enabled: every %dh to %s (keep %d)",
			cfg.BackupIntervalHours, cfg.BackupDir, cfg.BackupRetentionCount)
		// Take one backup at boot so a fresh deployment always has a baseline.
		go func() {
			if _, err := backupSvc.RunNow("startup"); err != nil {
				logger.Errorf("🗄️  Startup backup failed: %v", err)
			}
		}()
	}

	// P1: runtime metrics (goroutines / memory / GC) for /api/metrics
	metricsStop := make(chan struct{})
	go metrics.StartRuntimeCollector(metricsStop, 15*time.Second)

	// P0: daily P&L digest scheduler
	notification.StartDailySummary(st, metricsStop)

	// P1: retention pruning for audit logs and notification history
	go startRetentionPruner(st, cfg)

	// Start WebSocket market monitor FIRST (before loading traders that may need market data)
	// This ensures WSMonitorCli is initialized before any trader tries to access it
	go market.NewWSMonitor(150).Start(nil)
	logger.Info("📊 WebSocket market monitor started")
	// Give WebSocket monitor time to initialize
	time.Sleep(500 * time.Millisecond)

	// Create TraderManager and BacktestManager
	traderManager := manager.NewTraderManager()
	mcpClient := newSharedMCPClient()
	backtestManager := backtest.NewManager(mcpClient)
	if err := backtestManager.RestoreRuns(); err != nil {
		logger.Warnf("⚠️ Failed to restore backtest history: %v", err)
	}

	// Start position sync manager (detects manual closures, TP/SL triggers)
	positionSyncManager := trader.NewPositionSyncManager(st, 0) // 0 = use default 10s interval
	positionSyncManager.Start()
	defer positionSyncManager.Stop()

	// Load all traders from database to memory (may auto-start traders with IsRunning=true)
	if err := traderManager.LoadTradersFromStore(st); err != nil {
		logger.Fatalf("❌ Failed to load traders: %v", err)
	}

	// Display loaded trader information
	traders, err := st.Trader().List("default")
	if err != nil {
		logger.Fatalf("❌ Failed to get trader list: %v", err)
	}

	logger.Info("🤖 AI Trader Configurations in Database:")
	if len(traders) == 0 {
		logger.Info("  (No trader configurations, please create via Web interface)")
	} else {
		for _, t := range traders {
			status := "❌ Stopped"
			if t.IsRunning {
				status = "✅ Running"
			}
			logger.Infof("  • %s [%s] %s - AI Model: %s, Exchange: %s",
				t.Name, t.ID[:8], status, t.AIModelID, t.ExchangeID)
		}
	}

	// Start API server
	server := api.NewServer(traderManager, st, cryptoService, backtestManager, cfg.APIServerPort)
	server.SetBackupService(backupSvc)
	go func() {
		if err := server.Start(); err != nil {
			logger.Fatalf("❌ Failed to start API server: %v", err)
		}
	}()

	// P1: keep trading gauges fresh (active traders / open positions)
	go startTradingGaugeUpdater(traderManager, st, metricsStop)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("✅ System started successfully, waiting for trading commands...")
	logger.Info("📌 Tip: Use Ctrl+C to stop the system")

	<-quit
	logger.Info("📴 Shutdown signal received, closing system...")
	close(metricsStop)

	// Stop all traders
	traderManager.StopAll()
	logger.Info("✅ System shut down safely")
}

// startRetentionPruner periodically deletes expired audit/notification rows.
func startRetentionPruner(st *store.Store, cfg *config.Config) {
	prune := func() {
		if cfg.AuditRetentionDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -cfg.AuditRetentionDays)
			if n, err := st.Audit().PruneBefore(cutoff); err == nil && n > 0 {
				logger.Infof("🧹 Pruned %d audit log(s) older than %dd", n, cfg.AuditRetentionDays)
			}
		}
		if cfg.NotificationRetentionDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -cfg.NotificationRetentionDays)
			if n, err := st.Notification().PruneBefore(cutoff); err == nil && n > 0 {
				logger.Infof("🧹 Pruned %d notification(s) older than %dd", n, cfg.NotificationRetentionDays)
			}
		}
	}
	prune() // once at boot
	ticker := time.NewTicker(6 * time.Hour)
	for range ticker.C {
		prune()
	}
}

// startTradingGaugeUpdater refreshes active-trader / open-position gauges.
func startTradingGaugeUpdater(tm *manager.TraderManager, st *store.Store, stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			active := 0
			for _, t := range tm.GetAllTraders() {
				if t.IsRunning() {
					active++
				}
			}
			metrics.ActiveTraders.Set(float64(active))
			if positions, err := st.Position().CountAllOpen(); err == nil {
				metrics.OpenPositions.Set(float64(positions))
			}
		}
	}
}

// newSharedMCPClient creates a shared MCP AI client (for backtesting)
func newSharedMCPClient() mcp.AIClient {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		logger.Warn("⚠️ DEEPSEEK_API_KEY not set, AI features will be unavailable")
		return nil
	}
	return mcp.NewDeepSeekClient()
}
