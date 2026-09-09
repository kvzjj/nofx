package metrics

import (
	"strings"
	"testing"
)

func TestCounterGather(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("test_requests_total", "Test counter")
	c.Inc()
	c.Add(4)
	out := r.Gather()
	if !strings.Contains(out, "test_requests_total 5") {
		t.Fatalf("counter value missing in output:\n%s", out)
	}
	if !strings.Contains(out, "# TYPE test_requests_total counter") {
		t.Fatalf("TYPE line missing:\n%s", out)
	}
}

func TestGaugeSetAndAdd(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("test_gauge", "Test gauge")
	g.Set(10)
	g.Add(-3)
	if got := g.Value(); got != 7 {
		t.Fatalf("gauge = %v, want 7", got)
	}
}

func TestHistogramBuckets(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("test_latency", "Latency")
	h.Observe(0.01)
	h.Observe(0.5)
	h.Observe(2)
	out := r.Gather()
	if !strings.Contains(out, "test_latency_count 3") {
		t.Fatalf("count missing:\n%s", out)
	}
	if !strings.Contains(out, "test_latency_sum 2.51") {
		t.Fatalf("sum missing:\n%s", out)
	}
}

func TestSameMetricReturnsSameInstance(t *testing.T) {
	r := NewRegistry()
	a := r.Counter("dup_metric", "one")
	b := r.Counter("dup_metric", "two")
	a.Inc()
	if b.Value() != 1 {
		t.Fatal("registry must return the same counter instance")
	}
}

func TestGatherEscapesLabels(t *testing.T) {
	// Smoke: escaping helper behaves
	if got := escapeLabelValue(`a"b\c`); got != `a\"b\\c` {
		t.Fatalf("escapeLabelValue = %q", got)
	}
}
