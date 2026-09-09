package trader

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"

	"nofx/logger"
)

// OrderUpdateEvent is a normalized real-time order lifecycle event pushed by
// the exchange (Binance ORDER_TRADE_UPDATE).
type OrderUpdateEvent struct {
	Symbol          string
	OrderID         string
	ClientOrderID   string
	Status          OrderState
	OrderType       string
	Side            string
	PositionSide    string
	ExecutedQty     float64 // cumulative filled quantity
	AvgPrice        float64 // cumulative average fill price
	Fee             float64 // commission of the triggering trade (may be 0)
	LastFilledQty   float64
	LastFilledPrice float64
	Time            time.Time
}

// OrderUpdateStreamer is implemented by exchanges that can push real-time
// order lifecycle events. Consumers get low-latency fill notifications and no
// longer need to poll order status at high frequency; the returned cancel
// func unsubscribes (the stream shuts down with its last subscriber).
type OrderUpdateStreamer interface {
	SubscribeOrderUpdates() (<-chan OrderUpdateEvent, func())
}

// futuresUserStream maintains a single Binance futures user-data websocket per
// trader instance: listen-key lifecycle, keepalive, automatic reconnect with
// backoff, and fan-out to subscribers.
type futuresUserStream struct {
	client *futures.Client

	mu          sync.Mutex
	subscribers map[chan OrderUpdateEvent]struct{}
	running     bool
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

func newFuturesUserStream(client *futures.Client) *futuresUserStream {
	return &futuresUserStream{
		client:      client,
		subscribers: make(map[chan OrderUpdateEvent]struct{}),
	}
}

// SubscribeOrderUpdates registers a subscriber and lazily starts the stream.
// Events are dropped for slow consumers rather than blocking the websocket
// reader; order-monitor consumers treat silence as a signal to fall back to
// their slow reconciliation poll.
func (s *futuresUserStream) SubscribeOrderUpdates() (<-chan OrderUpdateEvent, func()) {
	ch := make(chan OrderUpdateEvent, 64)
	s.mu.Lock()
	if !s.running {
		s.running = true
		s.stopCh = make(chan struct{})
		s.wg.Add(1)
		go s.run(s.stopCh)
	}
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	unsubscribe := func() {
		s.mu.Lock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
			if len(s.subscribers) == 0 && s.running {
				s.running = false
				close(s.stopCh)
			}
		}
		s.mu.Unlock()
	}
	return ch, unsubscribe
}

// run keeps a healthy connection alive: reconnect with exponential backoff
// after failures, and reset the backoff once a connection has been stable.
func (s *futuresUserStream) run(stopCh chan struct{}) {
	defer s.wg.Done()
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		started := time.Now()
		err := s.streamOnce(stopCh)
		if time.Since(started) > time.Minute {
			backoff = time.Second // connection was stable; reset backoff
		}

		select {
		case <-stopCh:
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		if err != nil {
			logger.Infof("[user-stream] connection ended (%v), reconnecting in %s", err, backoff)
		}
	}
}

// streamOnce serves one websocket connection until it drops or the stream is
// stopped. It owns the listen-key keepalive (required every <60min by Binance).
func (s *futuresUserStream) streamOnce(stopCh chan struct{}) error {
	listenKey, err := s.client.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		return fmt.Errorf("listen key: %w", err)
	}

	handler := func(event *futures.WsUserDataEvent) {
		s.broadcast(normalizeOrderTradeUpdate(event))
	}
	errHandler := func(err error) {
		logger.Infof("[user-stream] websocket error: %v", err)
	}
	doneC, stopC, err := futures.WsUserDataServe(listenKey, handler, errHandler)
	if err != nil {
		return fmt.Errorf("websocket serve: %w", err)
	}
	defer func() {
		select {
		case stopC <- struct{}{}:
		default:
		}
	}()

	keepalive := time.NewTicker(30 * time.Minute)
	defer keepalive.Stop()
	for {
		select {
		case <-stopCh:
			return nil
		case <-doneC:
			return fmt.Errorf("websocket closed")
		case <-keepalive.C:
			if err := s.client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(context.Background()); err != nil {
				return fmt.Errorf("keepalive: %w", err)
			}
		}
	}
}

// broadcast fans one event out to all subscribers without blocking.
func (s *futuresUserStream) broadcast(event *OrderUpdateEvent) {
	if event == nil {
		return
	}
	s.mu.Lock()
	for ch := range s.subscribers {
		select {
		case ch <- *event:
		default: // slow consumer: drop rather than stall the reader
		}
	}
	s.mu.Unlock()
}

// normalizeOrderTradeUpdate converts a raw user-data event into the
// normalized OrderUpdateEvent used across the trader package.
func normalizeOrderTradeUpdate(event *futures.WsUserDataEvent) *OrderUpdateEvent {
	if event == nil || event.Event != futures.UserDataEventTypeOrderTradeUpdate {
		return nil
	}
	ou := event.OrderTradeUpdate
	executedQty, _ := strconv.ParseFloat(ou.AccumulatedFilledQty, 64)
	avgPrice, _ := strconv.ParseFloat(ou.AveragePrice, 64)
	fee, _ := strconv.ParseFloat(ou.Commission, 64)
	lastQty, _ := strconv.ParseFloat(ou.LastFilledQty, 64)
	lastPrice, _ := strconv.ParseFloat(ou.LastFilledPrice, 64)
	ts := time.UnixMilli(ou.TradeTime)
	if ts.IsZero() || ts.UnixMilli() == 0 {
		ts = time.UnixMilli(event.Time)
	}
	return &OrderUpdateEvent{
		Symbol:          ou.Symbol,
		OrderID:         strconv.FormatInt(ou.ID, 10),
		ClientOrderID:   ou.ClientOrderID,
		Status:          normalizeOrderState(string(ou.Status)),
		OrderType:       string(ou.Type),
		Side:            string(ou.Side),
		PositionSide:    string(ou.PositionSide),
		ExecutedQty:     executedQty,
		AvgPrice:        avgPrice,
		Fee:             fee,
		LastFilledQty:   lastQty,
		LastFilledPrice: lastPrice,
		Time:            ts,
	}
}
