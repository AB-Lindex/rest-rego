package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AB-Lindex/rest-rego/internal/types"
	dto "github.com/prometheus/client_model/go"
)

func TestSanitizeLabelValue(t *testing.T) {
	tests := []struct {
		name   string
		v      string
		maxLen int
		def    string
		want   string
	}{
		{"already clean input unchanged", "acme-tenant", 20, "-", "acme-tenant"},
		{"empty input returns default", "", 20, "-", "-"},
		{"strips control characters", "abc\ndef\tghi", 20, "-", "abcdefghi"},
		{"strips non-printable characters", "ab\x00\x01cd", 20, "-", "abcd"},
		{"strips non-ascii runes", "café日本語", 20, "-", "caf"},
		{"result empty after sanitisation falls back to default", "\x00\x01\x02", 20, "-", "-"},
		{"truncation at exact maxLen", "abcdefghij", 10, "-", "abcdefghij"},
		{"truncation one over maxLen", "abcdefghijk", 10, "-", "abcdefghij"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeLabelValue(tt.v, tt.maxLen, tt.def)
			if got != tt.want {
				t.Errorf("sanitizeLabelValue(%q, %d, %q) = %q, want %q", tt.v, tt.maxLen, tt.def, got, tt.want)
			}
		})
	}
}

// testStatusWriter is a minimal StatusWriter implementation for tests.
type testStatusWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *testStatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *testStatusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

func (w *testStatusWriter) Status() int { return w.status }
func (w *testStatusWriter) Size() int   { return w.size }

// gatherLabelNames returns the sorted label names attached to the metric samples of
// the given metric family name.
func gatherLabelNames(t *testing.T, family string) []string {
	t.Helper()
	families, err := metrics.reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != family {
			continue
		}
		if len(mf.Metric) == 0 {
			return nil
		}
		var names []string
		for _, lp := range mf.Metric[0].GetLabel() {
			names = append(names, lp.GetName())
		}
		return names
	}
	t.Fatalf("metric family %q not found", family)
	return nil
}

// findMetricByLabels returns the metric whose label set matches want exactly.
func findMetricByLabels(mf *dto.MetricFamily, want map[string]string) *dto.Metric {
	for _, m := range mf.Metric {
		if len(m.GetLabel()) != len(want) {
			continue
		}
		match := true
		for _, lp := range m.GetLabel() {
			if want[lp.GetName()] != lp.GetValue() {
				match = false
				break
			}
		}
		if match {
			return m
		}
	}
	return nil
}

func TestNewAndWrap_NoCustomLabels(t *testing.T) {
	New(nil, 20, "-")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	info := &types.Info{URL: "/test"}
	req = info.RequestWithInfo(req)

	rr := httptest.NewRecorder()
	w := &testStatusWriter{ResponseWriter: rr}

	handler := Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	names := gatherLabelNames(t, "http_requests_total")
	want := []string{"code", "method", "url"} // dto labels are sorted alphabetically
	if len(names) != len(want) {
		t.Fatalf("expected label names %v, got %v", want, names)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("expected label[%d] = %q, got %q", i, n, names[i])
		}
	}
}

func TestNewAndWrap_CustomLabels(t *testing.T) {
	New([]string{"tenant", "client_version"}, 10, "-")

	t.Run("records sanitised values in declared order", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/orders", nil)
		info := &types.Info{
			URL:    "/orders",
			Labels: map[string]string{"tenant": "acme", "client_version": "1.2.3", "unregistered": "ignored"},
		}
		req = info.RequestWithInfo(req)

		rr := httptest.NewRecorder()
		w := &testStatusWriter{ResponseWriter: rr}
		handler := Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		handler.ServeHTTP(w, req)

		mfs, err := metrics.reg.Gather()
		if err != nil {
			t.Fatalf("Gather: %v", err)
		}
		var mf *dto.MetricFamily
		for _, f := range mfs {
			if f.GetName() == "http_requests_total" {
				mf = f
				break
			}
		}
		if mf == nil {
			t.Fatal("http_requests_total metric family not found")
		}

		m := findMetricByLabels(mf, map[string]string{
			"method": "GET", "code": "200", "url": "/orders",
			"tenant": "acme", "client_version": "1.2.3",
		})
		if m == nil {
			t.Fatalf("expected a metric with the recorded custom label values, got families: %+v", mf.Metric)
		}

		// unregistered key must never be emitted as a label
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "unregistered" {
				t.Errorf("unregistered key must not be emitted as a label")
			}
		}
	})

	t.Run("missing per-request value falls back to default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/no-labels", nil)
		info := &types.Info{URL: "/no-labels"}
		req = info.RequestWithInfo(req)

		rr := httptest.NewRecorder()
		w := &testStatusWriter{ResponseWriter: rr}
		handler := Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		handler.ServeHTTP(w, req)

		mfs, err := metrics.reg.Gather()
		if err != nil {
			t.Fatalf("Gather: %v", err)
		}
		var mf *dto.MetricFamily
		for _, f := range mfs {
			if f.GetName() == "http_requests_total" {
				mf = f
				break
			}
		}
		if mf == nil {
			t.Fatal("http_requests_total metric family not found")
		}

		m := findMetricByLabels(mf, map[string]string{
			"method": "GET", "code": "200", "url": "/no-labels",
			"tenant": "-", "client_version": "-",
		})
		if m == nil {
			t.Fatalf("expected default label values for missing info.Labels entries, got families: %+v", mf.Metric)
		}
	})
}
