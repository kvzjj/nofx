package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// streamInterval 实时推送快照的间隔。
// 账户/持仓接口在交易员实例内部有缓存，5s 推送不会压垮交易所 API，
// 同时相比前端 15s 轮询，界面上的标记价与未实现盈亏更新更及时。
const streamInterval = 5 * time.Second

// handleTraderStream 通过 SSE 向前端推送账户与持仓快照。
//
// GET /api/traders/:id/stream?token=<JWT>
//
// 说明：浏览器的 EventSource 无法设置 Authorization 请求头，
// 因此该端点额外支持通过 token 查询参数认证（authMiddleware 中处理），
// JWT 校验逻辑与 Bearer 方式完全一致。
func (s *Server) handleTraderStream(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 确保该用户的交易员已加载到内存
	if err := s.traderManager.LoadUserTradersFromStore(s.store, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load traders"})
		return
	}

	// 校验交易员归属，防止越权订阅他人数据
	userTraders, err := s.store.Trader().List(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trader list"})
		return
	}
	owned := false
	for _, t := range userTraders {
		if t.ID == traderID {
			owned = true
			break
		}
	}
	if !owned {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// 明确告知 nginx 反代不要缓冲该响应（否则事件会被攒着不推）
	c.Header("X-Accel-Buffering", "no")

	clientGone := c.Request.Context().Done()

	// sendSnapshot 推送一帧快照；返回 false 表示连接已断开
	sendSnapshot := func() bool {
		snapshot := gin.H{
			"type": "snapshot",
			"ts":   time.Now().UnixMilli(),
		}
		if account, err := trader.GetAccountInfo(); err == nil {
			snapshot["account"] = account
		}
		if positions, err := trader.GetPositions(); err == nil {
			snapshot["positions"] = positions
		}

		data, err := json.Marshal(snapshot)
		if err != nil {
			return true // 序列化失败跳过这一帧，不断开连接
		}

		if _, err := fmt.Fprintf(c.Writer, "event: snapshot\ndata: %s\n\n", data); err != nil {
			return false
		}
		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}
		return true
	}

	// 先推一帧，让前端立即拿到数据
	if !sendSnapshot() {
		return
	}

	ticker := time.NewTicker(streamInterval)
	defer ticker.Stop()

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			if !sendSnapshot() {
				return
			}
		}
	}
}
