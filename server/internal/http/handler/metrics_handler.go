package handler

import (
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector holds application-level metrics for Prometheus exposition.
type MetricsCollector struct {
	// HTTP request metrics
	httpRequestsTotal   map[string]*uint64 // method:path → count
	httpRequestDuration map[string]*uint64 // method:path → cumulative ns
	httpMu             sync.RWMutex

	// Reminder scan metrics
	reminderScansTotal    uint64
	reminderEmailsSent    uint64
	reminderWebhooksSent  uint64
	accountReactivations  uint64

	// Credential validation metrics
	validationTotal   uint64
	validationFailures uint64
}

var collector = &MetricsCollector{
	httpRequestsTotal:   make(map[string]*uint64),
	httpRequestDuration: make(map[string]*uint64),
}

func RecordHTTPRequest(method, path string, duration time.Duration) {
	collector.httpMu.Lock()
	key := method + ":" + path
	if collector.httpRequestsTotal[key] == nil {
		collector.httpRequestsTotal[key] = new(uint64)
		collector.httpRequestDuration[key] = new(uint64)
	}
	atomic.AddUint64(collector.httpRequestsTotal[key], 1)
	atomic.AddUint64(collector.httpRequestDuration[key], uint64(duration))
	collector.httpMu.Unlock()
}

func recordReminderScan()                          { atomic.AddUint64(&collector.reminderScansTotal, 1) }
func recordReminderEmail()                         { atomic.AddUint64(&collector.reminderEmailsSent, 1) }
func recordReminderWebhook()                       { atomic.AddUint64(&collector.reminderWebhooksSent, 1) }
func recordAccountReactivation()                    { atomic.AddUint64(&collector.accountReactivations, 1) }
func recordValidation(success bool)                {
	atomic.AddUint64(&collector.validationTotal, 1)
	if !success {
		atomic.AddUint64(&collector.validationFailures, 1)
	}
}

// MetricsHandler exposes metrics in Prometheus text format at GET /metrics.
type MetricsHandler struct{}

func NewMetricsHandler() *MetricsHandler { return &MetricsHandler{} }

func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Build metrics output
	var lines []string
	lines = append(lines, "# HELP dns_hub_http_requests_total Total HTTP requests")
	lines = append(lines, "# TYPE dns_hub_http_requests_total counter")
	collector.httpMu.RLock()
	for key, count := range collector.httpRequestsTotal {
		parts := split2(key, ":")
		lines = append(lines, "dns_hub_http_requests_total{method=\""+parts[0]+"\",path=\""+parts[1]+"\"} "+strconv.FormatUint(atomic.LoadUint64(count), 10))
	}
	collector.httpMu.RUnlock()

	lines = append(lines, "")
	lines = append(lines, "# HELP dns_hub_reminder_scans_total Total reminder scans")
	lines = append(lines, "# TYPE dns_hub_reminder_scans_total counter")
	lines = append(lines, "dns_hub_reminder_scans_total "+strconv.FormatUint(atomic.LoadUint64(&collector.reminderScansTotal), 10))

	lines = append(lines, "# HELP dns_hub_reminder_emails_sent_total Total reminder emails sent")
	lines = append(lines, "# TYPE dns_hub_reminder_emails_sent_total counter")
	lines = append(lines, "dns_hub_reminder_emails_sent_total "+strconv.FormatUint(atomic.LoadUint64(&collector.reminderEmailsSent), 10))

	lines = append(lines, "# HELP dns_hub_reminder_webhooks_sent_total Total reminder webhooks sent")
	lines = append(lines, "# TYPE dns_hub_reminder_webhooks_sent_total counter")
	lines = append(lines, "dns_hub_reminder_webhooks_sent_total "+strconv.FormatUint(atomic.LoadUint64(&collector.reminderWebhooksSent), 10))

	lines = append(lines, "# HELP dns_hub_account_reactivations_total Total account reactivations triggered")
	lines = append(lines, "# TYPE dns_hub_account_reactivations_total counter")
	lines = append(lines, "dns_hub_account_reactivations_total "+strconv.FormatUint(atomic.LoadUint64(&collector.accountReactivations), 10))

	lines = append(lines, "# HELP dns_hub_validations_total Total credential validations")
	lines = append(lines, "# TYPE dns_hub_validations_total counter")
	lines = append(lines, "dns_hub_validations_total "+strconv.FormatUint(atomic.LoadUint64(&collector.validationTotal), 10))

	lines = append(lines, "# HELP dns_hub_validation_failures_total Total credential validation failures")
	lines = append(lines, "# TYPE dns_hub_validation_failures_total counter")
	lines = append(lines, "dns_hub_validation_failures_total "+strconv.FormatUint(atomic.LoadUint64(&collector.validationFailures), 10))

	for _, line := range lines {
		w.Write([]byte(line + "\n"))
	}
}

func split2(s, sep string) [2]string {
	for i := 0; i < len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return [2]string{s[:i], s[i+len(sep):]}
		}
	}
	return [2]string{s, ""}
}
