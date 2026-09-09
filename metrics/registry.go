// Package metrics provides a lightweight, dependency-free metrics registry
// exposing counters, gauges and histograms in the Prometheus text exposition
// format. It is intentionally minimal: NOFX only needs to scrape a handful of
// HTTP / runtime / trading metrics, so pulling in the full client_golang
// dependency is not justified.
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// MetricType enumerates supported metric kinds.
type MetricType string

const (
	TypeCounter   MetricType = "counter"
	TypeGauge     MetricType = "gauge"
	TypeHistogram MetricType = "histogram"
)

var defaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

// labels are stored sorted by key so that the canonical series name is stable.
type labelPairs map[string]string

func (l labelPairs) key() string {
	if len(l) == 0 {
		return ""
	}
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(l[k])
		b.WriteByte(',')
	}
	return b.String()
}

func (l labelPairs) render() string {
	if len(l) == 0 {
		return ""
	}
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(escapeLabelValue(l[k]))
		b.WriteString(`"`)
	}
	b.WriteByte('}')
	return b.String()
}

func escapeLabelValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return v
}

// Registry holds all registered metrics.
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:   map[string]*Counter{},
		gauges:     map[string]*Gauge{},
		histograms: map[string]*Histogram{},
	}
}

var global = NewRegistry()

// Default returns the process-wide registry.
func Default() *Registry { return global }

func validName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		ok := r == '_' || r == ':' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9' && i > 0)
		if !ok {
			return false
		}
	}
	return true
}

// Counter is a monotonically increasing metric.
type Counter struct {
	name string
	help string
	mu   sync.Mutex
	val  float64
}

// Inc adds 1.
func (c *Counter) Inc() { c.Add(1) }

// Add adds v (v >= 0).
func (c *Counter) Add(v float64) {
	if v < 0 {
		return
	}
	c.mu.Lock()
	c.val += v
	c.mu.Unlock()
}

// Value returns current value.
func (c *Counter) Value() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.val
}

// Gauge is a metric that can go up and down.
type Gauge struct {
	name string
	help string
	mu   sync.Mutex
	val  float64
}

// Set stores v.
func (g *Gauge) Set(v float64) {
	g.mu.Lock()
	g.val = v
	g.mu.Unlock()
}

// Add increments by v (may be negative).
func (g *Gauge) Add(v float64) {
	g.mu.Lock()
	g.val += v
	g.mu.Unlock()
}

// Value returns current value.
func (g *Gauge) Value() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.val
}

// Histogram records value distributions into fixed buckets.
type Histogram struct {
	name    string
	help    string
	buckets []float64
	mu      sync.Mutex
	counts  []uint64
	sum     float64
	count   uint64
}

// Observe records one value.
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, ub := range h.buckets {
		if v <= ub {
			h.counts[i]++
		}
	}
	h.sum += v
	h.count++
}

// Counter registers (or fetches) a counter with fixed empty labels.
func (r *Registry) Counter(name, help string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !validName(name) {
		panic(fmt.Sprintf("metrics: invalid metric name %q", name))
	}
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &Counter{name: name, help: help}
	r.counters[name] = c
	return c
}

// Gauge registers (or fetches) a gauge.
func (r *Registry) Gauge(name, help string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !validName(name) {
		panic(fmt.Sprintf("metrics: invalid metric name %q", name))
	}
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := &Gauge{name: name, help: help}
	r.gauges[name] = g
	return g
}

// Histogram registers (or fetches) a histogram with default buckets.
func (r *Registry) Histogram(name, help string) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !validName(name) {
		panic(fmt.Sprintf("metrics: invalid metric name %q", name))
	}
	if h, ok := r.histograms[name]; ok {
		return h
	}
	h := &Histogram{name: name, help: help, buckets: defaultBuckets, counts: make([]uint64, len(defaultBuckets))}
	r.histograms[name] = h
	return h
}

// Gather renders the registry in Prometheus text exposition format.
func (r *Registry) Gather() string {
	r.mu.RLock()
	counters := make([]*Counter, 0, len(r.counters))
	for _, c := range r.counters {
		counters = append(counters, c)
	}
	gauges := make([]*Gauge, 0, len(r.gauges))
	for _, g := range r.gauges {
		gauges = append(gauges, g)
	}
	histograms := make([]*Histogram, 0, len(r.histograms))
	for _, h := range r.histograms {
		histograms = append(histograms, h)
	}
	r.mu.RUnlock()

	sort.Slice(counters, func(i, j int) bool { return counters[i].name < counters[j].name })
	sort.Slice(gauges, func(i, j int) bool { return gauges[i].name < gauges[j].name })
	sort.Slice(histograms, func(i, j int) bool { return histograms[i].name < histograms[j].name })

	var b strings.Builder
	for _, c := range counters {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s counter\n%s %v\n", c.name, c.help, c.name, c.name, c.Value())
	}
	for _, g := range gauges {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n%s %v\n", g.name, g.help, g.name, g.name, g.Value())
	}
	for _, h := range histograms {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s histogram\n", h.name, h.help, h.name)
		h.mu.Lock()
		for i, ub := range h.buckets {
			fmt.Fprintf(&b, "%s_bucket{le=\"%v\"} %d\n", h.name, ub, h.counts[i])
		}
		fmt.Fprintf(&b, "%s_bucket{le=\"+Inf\"} %d\n", h.name, h.count)
		fmt.Fprintf(&b, "%s_sum %v\n", h.name, h.sum)
		fmt.Fprintf(&b, "%s_count %d\n", h.name, h.count)
		h.mu.Unlock()
	}
	return b.String()
}

// Convenience accessors on the default registry ------------------------

// GetCounter fetches or creates a counter on the default registry.
func GetCounter(name, help string) *Counter { return global.Counter(name, help) }

// GetGauge fetches or creates a gauge on the default registry.
func GetGauge(name, help string) *Gauge { return global.Gauge(name, help) }

// GetHistogram fetches or creates a histogram on the default registry.
func GetHistogram(name, help string) *Histogram { return global.Histogram(name, help) }
