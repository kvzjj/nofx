package api

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// adminMiddleware guards admin-only routes. The bootstrap "admin" account
// (userID == "admin") is the only administrator in this deployment mode.
func (s *Server) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// handleAdminListUsers returns all users with per-user trader statistics.
func (s *Server) handleAdminListUsers(c *gin.Context) {
	users, err := s.store.User().ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]gin.H, 0, len(users))
	for _, u := range users {
		traders, _ := s.store.Trader().List(u.ID)
		running := 0
		for _, t := range traders {
			if t.IsRunning {
				running++
			}
		}
		result = append(result, gin.H{
			"user_id":       u.ID,
			"email":         MaskEmail(u.Email),
			"otp_verified":  u.OTPVerified,
			"created_at":    u.CreatedAt,
			"trader_count":  len(traders),
			"running_count": running,
		})
	}
	c.JSON(http.StatusOK, result)
}

// handleAdminSystemStatus returns runtime/system health information.
func (s *Server) handleAdminSystemStatus(c *gin.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	dbSize := int64(0)
	if info, err := os.Stat("data/data.db"); err == nil {
		dbSize = info.Size()
	}

	allUserIDs, _ := s.store.User().GetAllIDs()
	totalTraders := 0
	runningTraders := 0
	for _, uid := range allUserIDs {
		traders, err := s.store.Trader().List(uid)
		if err != nil {
			continue
		}
		totalTraders += len(traders)
		for _, t := range traders {
			if t.IsRunning {
				runningTraders++
			}
		}
	}

	verified, _ := s.store.User().CountVerified()

	c.JSON(http.StatusOK, gin.H{
		"users_total":     len(allUserIDs),
		"users_verified":  verified,
		"traders_total":   totalTraders,
		"traders_running": runningTraders,
		"goroutines":      runtime.NumGoroutine(),
		"heap_alloc_mb":   float64(mem.HeapAlloc) / 1024 / 1024,
		"sys_mem_mb":      float64(mem.Sys) / 1024 / 1024,
		"db_size_mb":      float64(dbSize) / 1024 / 1024,
		"cpu_count":       runtime.NumCPU(),
		"go_version":      runtime.Version(),
		"server_time":     time.Now().Format(time.RFC3339),
	})
}

// handleAdminRecentDecisions returns the latest decision records across all
// traders (system-wide activity feed).
func (s *Server) handleAdminRecentDecisions(c *gin.Context) {
	allUserIDs, _ := s.store.User().GetAllIDs()
	type feedItem struct {
		TraderID string    `json:"trader_id"`
		Success  bool      `json:"success"`
		Error    string    `json:"error,omitempty"`
		Time     time.Time `json:"timestamp"`
		Cycle    int       `json:"cycle_number"`
	}

	feed := make([]feedItem, 0, 50)
	for _, uid := range allUserIDs {
		traders, err := s.store.Trader().List(uid)
		if err != nil {
			continue
		}
		for _, t := range traders {
			records, err := s.store.Decision().GetLatestRecords(t.ID, 5)
			if err != nil {
				continue
			}
			for _, r := range records {
				feed = append(feed, feedItem{
					TraderID: t.Name + " (" + t.ID[:min(8, len(t.ID))] + ")",
					Success:  r.Success,
					Error:    r.ErrorMessage,
					Time:     r.Timestamp,
					Cycle:    r.CycleNumber,
				})
			}
		}
	}

	// 按时间倒序取最近 50 条
	for i, j := 0, len(feed)-1; i < j; i, j = i+1, j-1 {
		feed[i], feed[j] = feed[j], feed[i]
	}
	if len(feed) > 50 {
		feed = feed[:50]
	}
	c.JSON(http.StatusOK, feed)
}
