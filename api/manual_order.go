package api

import (
	"fmt"
	"net/http"
	"strings"

	"nofx/decision"
	"nofx/logger"
	"nofx/market"

	"github.com/gin-gonic/gin"
)

type manualOrderRequest struct {
	Symbol        string  `json:"symbol" binding:"required"`
	Action        string  `json:"action" binding:"required"` // open_long | open_short | close_long | close_short
	StopLossPct   float64 `json:"stop_loss_pct"`             // 开仓必填：相对入场价的百分比
	TakeProfitPct float64 `json:"take_profit_pct"`           // 开仓必填：相对入场价的百分比
}

// handleManualOrder submits a manual order through the same execution
// pipeline (risk controls, backend sizing, protection orders, decision log)
// as AI decisions.
func (s *Server) handleManualOrder(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req manualOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol and action are required"})
		return
	}
	req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))
	req.Action = strings.TrimSpace(req.Action)

	validActions := map[string]bool{
		"open_long": true, "open_short": true,
		"close_long": true, "close_short": true,
	}
	if !validActions[req.Action] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be one of open_long/open_short/close_long/close_short"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	d := &decision.Decision{
		Symbol:    req.Symbol,
		Action:    req.Action,
		Reasoning: "Manual order submitted from web panel",
	}

	// 开仓需要把百分比止损止盈转换为绝对价格（走与 AI 一致的风控校验）
	if req.Action == "open_long" || req.Action == "open_short" {
		if req.StopLossPct <= 0 || req.StopLossPct >= 50 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "stop_loss_pct must be between 0 and 50"})
			return
		}
		if req.TakeProfitPct <= 0 || req.TakeProfitPct >= 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "take_profit_pct must be between 0 and 100"})
			return
		}
		price, err := market.Get(req.Symbol)
		if err != nil || price == nil || price.CurrentPrice <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to get market price for %s", req.Symbol)})
			return
		}
		entryPrice := price.CurrentPrice
		if req.Action == "open_long" {
			d.StopLoss = entryPrice * (1 - req.StopLossPct/100)
			d.TakeProfit = entryPrice * (1 + req.TakeProfitPct/100)
		} else {
			d.StopLoss = entryPrice * (1 + req.StopLossPct/100)
			d.TakeProfit = entryPrice * (1 - req.TakeProfitPct/100)
		}
	}

	logger.Infof("🖐 User %s manual order: trader=%s %s %s (sl%%=%.2f tp%%=%.2f)",
		userID, traderID, req.Action, req.Symbol, req.StopLossPct, req.TakeProfitPct)

	if err := trader.ExecuteDecision(d); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Manual order executed"})
}
