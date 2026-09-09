package notification

import (
	"fmt"
	"time"

	"nofx/logger"
	"nofx/store"
)

// StartDailySummary schedules a per-user daily P&L digest. Each user gets the
// summary at their configured hour (default 08:00 UTC) computed from the
// equity snapshots table.
func StartDailySummary(st *store.Store, stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		lastSent := map[string]string{} // userID -> "2006-01-02" last digest day
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				sendDailySummaries(st, now, lastSent)
			}
		}
	}()
}

func sendDailySummaries(st *store.Store, now time.Time, lastSent map[string]string) {
	ids, err := st.User().GetAllIDs()
	if err != nil {
		logger.Errorf("notification: daily summary list users failed: %v", err)
		return
	}
	dayKey := now.UTC().Format("2006-01-02")
	for _, uid := range ids {
		if lastSent[uid] == dayKey {
			continue
		}
		// Send once per user per day shortly after 08:00 UTC.
		if now.UTC().Hour() != 8 {
			continue
		}
		if summary, ok := buildUserSummary(st, uid, now); ok {
			Publish(summary)
		}
		lastSent[uid] = dayKey
	}
}

// buildUserSummary aggregates yesterday's realized P&L for all of a user's
// traders from the position store.
func buildUserSummary(st *store.Store, userID string, now time.Time) (*Event, bool) {
	traders, err := st.Trader().List(userID)
	if err != nil || len(traders) == 0 {
		return nil, false
	}

	dayStart := now.UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)

	var totalPnL float64
	var closedCount int
	var names []string
	for _, t := range traders {
		positions, err := st.Position().ListClosedBetween(t.ID, dayStart, dayEnd)
		if err != nil {
			continue
		}
		var traderPnL float64
		for _, p := range positions {
			traderPnL += p.RealizedPnL
			closedCount++
		}
		if traderPnL != 0 || len(positions) > 0 {
			names = append(names, fmt.Sprintf("%s: %+.2f USDT (%d closed)", t.Name, traderPnL, len(positions)))
		}
		totalPnL += traderPnL
	}

	if closedCount == 0 {
		return nil, false
	}

	body := fmt.Sprintf("昨日已平仓 %d 笔，总盈亏 %+.2f USDT。", closedCount, totalPnL)
	if len(names) > 0 {
		body += "\n" + joinLines(names)
	}
	return &Event{
		EventType: EventDailySummary,
		Severity:  SeverityInfo,
		UserID:    userID,
		Title:     fmt.Sprintf("每日交易摘要 / Daily summary (%+.2f USDT)", totalPnL),
		Body:      body,
		Time:      timeNowUTC(),
	}, true
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
