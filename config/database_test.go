package config

import (
	"nofx/crypto"
	"os"
	"testing"
	"time"
)

// updateExchange is a thin wrapper around UpdateExchange (binance-only).
func updateExchange(db *Database, userID, id string, enabled bool, apiKey, secretKey string, testnet bool) error {
	return db.UpdateExchange(userID, id, enabled, apiKey, secretKey, testnet)
}

// TestUpdateExchange_EmptyValuesShouldNotOverwrite 测试空值不应覆盖现有数据
// 这是 Bug 的核心：空字符串不得覆盖现有的 API/Secret 密钥
func TestUpdateExchange_EmptyValuesShouldNotOverwrite(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-001"

	// 步骤 1: 创建初始配置（包含密钥）
	initialAPIKey := "initial-api-key-12345"
	initialSecretKey := "initial-secret-key-67890"

	if err := updateExchange(db, userID, "binance", true, initialAPIKey, initialSecretKey, false); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 步骤 2: 用空值更新（模拟前端发送空值的场景）
	// 空 apiKey/secretKey 不应该覆盖现有密钥
	if err := updateExchange(db, userID, "binance", false, "", "", true); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	// 步骤 3: 验证密钥没有被空值覆盖
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("获取更新后配置失败: %v", err)
	}
	if len(exchanges) == 0 {
		t.Fatal("未找到配置")
	}

	// 🎯 关键断言：密钥应该保持不变
	if exchanges[0].APIKey != initialAPIKey {
		t.Errorf("❌ Bug 确认：APIKey 被空值覆盖了！期望 %s，实际 %s", initialAPIKey, exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != initialSecretKey {
		t.Errorf("❌ Bug 确认：SecretKey 被空值覆盖了！期望 %s，实际 %s", initialSecretKey, exchanges[0].SecretKey)
	}

	// 验证非敏感字段正常更新
	if exchanges[0].Enabled {
		t.Error("enabled 应该被更新为 false")
	}
	if !exchanges[0].Testnet {
		t.Error("testnet 应该被更新为 true")
	}
}

// TestUpdateExchange_NonEmptyValuesShouldUpdate 测试非空值应该正常更新
func TestUpdateExchange_NonEmptyValuesShouldUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-003"

	if err := updateExchange(db, userID, "binance", true, "old-api-key", "old-secret-key", false); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	newAPIKey := "new-api-key-456"
	newSecretKey := "new-secret-key-789"
	if err := updateExchange(db, userID, "binance", true, newAPIKey, newSecretKey, false); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("获取配置失败: %v", err)
	}

	if exchanges[0].APIKey != newAPIKey {
		t.Errorf("APIKey 未更新，期望 %s，实际 %s", newAPIKey, exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != newSecretKey {
		t.Errorf("SecretKey 未更新，期望 %s，实际 %s", newSecretKey, exchanges[0].SecretKey)
	}
}

// TestUpdateExchange_PartialUpdateShouldWork 测试部分字段更新
func TestUpdateExchange_PartialUpdateShouldWork(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-005"

	if err := updateExchange(db, userID, "binance", true, "api-key-123", "secret-key-456", false); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 只更新 enabled 和 testnet，密钥留空
	if err := updateExchange(db, userID, "binance", false, "", "", true); err != nil {
		t.Fatalf("部分更新失败: %v", err)
	}

	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("获取配置失败: %v", err)
	}

	// 密钥应该保持不变
	if exchanges[0].APIKey != "api-key-123" {
		t.Errorf("APIKey 不应改变，期望 api-key-123，实际 %s", exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != "secret-key-456" {
		t.Errorf("SecretKey 不应改变，期望 secret-key-456，实际 %s", exchanges[0].SecretKey)
	}

	// 其他字段应该更新
	if exchanges[0].Enabled {
		t.Error("enabled 应该更新为 false")
	}
	if !exchanges[0].Testnet {
		t.Error("testnet 应该更新为 true")
	}
}

// TestUpdateExchange_SupportedAndUnsupportedTypes 测试交易所白名单行为：
// 仅 binance/okx 可写入，已下线的交易所类型必须被拒绝
func TestUpdateExchange_SupportedAndUnsupportedTypes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-006"

	supported := []struct {
		exchangeID string
		name       string
	}{
		{"binance", "Binance Futures"},
	}
	for _, tc := range supported {
		t.Run(tc.exchangeID, func(t *testing.T) {
			if err := updateExchange(db, userID, tc.exchangeID, true,
				"api-key-"+tc.exchangeID, "secret-key-"+tc.exchangeID, false); err != nil {
				t.Fatalf("创建 %s 失败: %v", tc.exchangeID, err)
			}

			exchanges, err := db.GetExchanges(userID)
			if err != nil {
				t.Fatalf("获取配置失败: %v", err)
			}

			found := false
			for _, ex := range exchanges {
				if ex.ID == tc.exchangeID {
					found = true
					if ex.Name != tc.name {
						t.Errorf("交易所名称不正确，期望 %s，实际 %s", tc.name, ex.Name)
					}
					if ex.Type != "cex" {
						t.Errorf("交易所类型不正确，期望 cex，实际 %s", ex.Type)
					}
					if ex.APIKey != "api-key-"+tc.exchangeID {
						t.Errorf("APIKey 不正确")
					}
					break
				}
			}
			if !found {
				t.Errorf("未找到交易所 %s", tc.exchangeID)
			}
		})
	}

	// 已下线的交易所不再支持：本测试验证白名单拒绝除 binance 外的一切类型
	for _, unsupported := range []string{"okx", "hyperliquid", "aster", "bybit", "unknown-exchange"} {
		t.Run("reject_"+unsupported, func(t *testing.T) {
			if err := updateExchange(db, userID, unsupported, true, "k", "s", false); err == nil {
				t.Errorf("已下线的交易所 %s 不应写入成功", unsupported)
			}
		})
	}
}

// TestUpdateExchange_MixedSensitiveFields 测试混合更新敏感和非敏感字段
func TestUpdateExchange_MixedSensitiveFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-007"

	if err := updateExchange(db, userID, "binance", true, "old-api-key", "old-secret-key", false); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 场景1: 只更新 apiKey，secretKey 留空
	if err := updateExchange(db, userID, "binance", false, "new-api-key", "", true); err != nil {
		t.Fatalf("更新1失败: %v", err)
	}

	exchanges, _ := db.GetExchanges(userID)
	if exchanges[0].APIKey != "new-api-key" {
		t.Error("APIKey 应该更新")
	}
	if exchanges[0].SecretKey != "old-secret-key" {
		t.Error("SecretKey 应该保持不变")
	}

	// 场景2: 只更新 secretKey，apiKey 留空
	if err := updateExchange(db, userID, "binance", true, "", "new-secret-key", false); err != nil {
		t.Fatalf("更新2失败: %v", err)
	}

	exchanges, _ = db.GetExchanges(userID)
	if exchanges[0].APIKey != "new-api-key" {
		t.Error("APIKey 应该保持不变")
	}
	if exchanges[0].SecretKey != "new-secret-key" {
		t.Error("SecretKey 应该更新")
	}
	if exchanges[0].Enabled != true {
		t.Error("Enabled 应该更新为 true")
	}
}

// TestUpdateExchange_OnlyNonSensitiveFields 测试只更新非敏感字段
func TestUpdateExchange_OnlyNonSensitiveFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-008"

	if err := updateExchange(db, userID, "binance", true, "binance-api", "binance-secret", false); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 只更新非敏感字段（密钥字段留空）
	if err := updateExchange(db, userID, "binance", false, "", "", true); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	exchanges, _ := db.GetExchanges(userID)
	if exchanges[0].APIKey != "binance-api" {
		t.Errorf("APIKey 应该保持不变，实际 %s", exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != "binance-secret" {
		t.Errorf("SecretKey 应该保持不变，实际 %s", exchanges[0].SecretKey)
	}

	// 验证非敏感字段已更新
	if exchanges[0].Enabled != false {
		t.Error("Enabled 应该更新为 false")
	}
	if exchanges[0].Testnet != true {
		t.Error("Testnet 应该更新为 true")
	}
}

// TestUpdateExchange_AllSensitiveFieldsUpdate 测试同时更新所有敏感字段
func TestUpdateExchange_AllSensitiveFieldsUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-009"

	if err := updateExchange(db, userID, "binance", true, "old-api", "old-secret", false); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	if err := updateExchange(db, userID, "binance", false, "new-api", "new-secret", true); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	exchanges, _ := db.GetExchanges(userID)
	if exchanges[0].APIKey != "new-api" {
		t.Error("APIKey 应该更新")
	}
	if exchanges[0].SecretKey != "new-secret" {
		t.Error("SecretKey 应该更新")
	}
	if !exchanges[0].Testnet {
		t.Error("Testnet 应该更新为 true")
	}
}

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) (*Database, func()) {
	// 创建临时数据库文件
	tmpFile := t.TempDir() + "/test.db"

	db, err := NewDatabase(tmpFile)
	if err != nil {
		t.Fatalf("创建测试数据库失败: %v", err)
	}

	// 创建测试用户
	testUsers := []string{
		"test-user-001", "test-user-002", "test-user-003", "test-user-004", "test-user-005",
		"test-user-006", "test-user-007", "test-user-008", "test-user-009",
		"test-user-persistence", "user1", "user2",
	}
	for _, userID := range testUsers {
		user := &User{
			ID:           userID,
			Email:        userID + "@test.com",
			PasswordHash: "hash",
			OTPSecret:    "",
			OTPVerified:  false,
		}
		_ = db.CreateUser(user)
	}

	// 设置加密服务（用于测试加密功能）。
	// 密钥从环境变量加载；未配置时降级为无加密模式继续测试。
	cryptoService, err := crypto.NewCryptoService()
	if err != nil {
		// 如果创建失败，继续测试但不使用加密
		t.Logf("警告：无法创建加密服务，将在无加密模式下测试: %v", err)
	} else {
		db.SetCryptoService(cryptoService)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpFile)
	}

	return db, cleanup
}

// TestWALModeEnabled 测试 WAL 模式是否启用
func TestWALModeEnabled(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var journalMode string
	err := db.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("查询 journal_mode 失败: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("期望 journal_mode=wal，实际是 %s", journalMode)
	}
}

// TestSynchronousMode 测试 synchronous 模式设置
func TestSynchronousMode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var synchronous int
	err := db.db.QueryRow("PRAGMA synchronous").Scan(&synchronous)
	if err != nil {
		t.Fatalf("查询 synchronous 失败: %v", err)
	}

	if synchronous != 2 {
		t.Errorf("期望 synchronous=2 (FULL)，实际是 %d", synchronous)
	}
}

// TestDataPersistenceAcrossReopen 测试数据在数据库关闭并重新打开后是否持久化
// 模拟 Docker restart 场景
func TestDataPersistenceAcrossReopen(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_persistence_*.db")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	// 设置加密服务（密钥从环境变量加载；未配置时跳过本测试，
	// 因为加密持久化路径需要真实密钥）
	cryptoService, err := crypto.NewCryptoService()
	if err != nil {
		t.Skipf("加密服务不可用（未配置环境变量），跳过加密持久化测试: %v", err)
	}

	userID := "test-user-persistence"
	testAPIKey := "test-api-key-should-persist"
	testSecretKey := "test-secret-key-should-persist"

	// 第一次打开数据库并写入数据
	{
		db, err := NewDatabase(dbPath)
		if err != nil {
			t.Fatalf("第一次创建数据库失败: %v", err)
		}
		db.SetCryptoService(cryptoService)

		_ = db.CreateUser(&User{
			ID:           userID,
			Email:        userID + "@test.com",
			PasswordHash: "hash",
			OTPSecret:    "",
			OTPVerified:  true,
		})

		if err := updateExchange(db, userID, "binance", true, testAPIKey, testSecretKey, false); err != nil {
			t.Fatalf("写入数据失败: %v", err)
		}

		if err := db.Close(); err != nil {
			t.Fatalf("关闭数据库失败: %v", err)
		}
	}

	// 第二次打开数据库并验证数据是否还在
	{
		db, err := NewDatabase(dbPath)
		if err != nil {
			t.Fatalf("第二次打开数据库失败: %v", err)
		}
		db.SetCryptoService(cryptoService)
		defer db.Close()

		exchanges, err := db.GetExchanges(userID)
		if err != nil {
			t.Fatalf("读取数据失败: %v", err)
		}

		if len(exchanges) == 0 {
			t.Fatal("数据丢失：没有找到任何交易所配置")
		}

		found := false
		for _, ex := range exchanges {
			if ex.ID == "binance" {
				found = true
				if ex.APIKey != testAPIKey {
					t.Errorf("API Key 丢失或损坏，期望 %s，实际 %s", testAPIKey, ex.APIKey)
				}
				if ex.SecretKey != testSecretKey {
					t.Errorf("Secret Key 丢失或损坏，期望 %s，实际 %s", testSecretKey, ex.SecretKey)
				}
			}
		}

		if !found {
			t.Error("数据丢失：找不到 binance 配置")
		}
	}
}

// TestConcurrentWritesWithWAL 测试 WAL 模式下的并发写入
func TestConcurrentWritesWithWAL(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	done := make(chan bool, 2)
	errors := make(chan error, 10)

	go func() {
		for i := 0; i < 3; i++ {
			if err := updateExchange(db, "user1", "binance", true, "key1", "secret1", false); err != nil {
				errors <- err
			}
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 3; i++ {
			if err := updateExchange(db, "user2", "binance", true, "key2", "secret2", false); err != nil {
				errors <- err
			}
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	<-done
	<-done
	close(errors)

	errorCount := 0
	for err := range errors {
		t.Logf("并发写入错误: %v", err)
		errorCount++
	}

	// WAL 模式下应该能处理并发,但可能有少量锁错误
	if errorCount > 2 {
		t.Errorf("并发写入失败次数过多: %d", errorCount)
	}
}
