package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const defaultNamespace = "stock_app"

// dbStats is one database's accumulated health-check samples.
type dbStats struct {
	latencySumMs int64 // over successful pings only
	latencyCount int64 // successful pings
	errors       int64 // failed pings
	up           bool  // result of the most recent ping (kept across drains)
}

// Metrics accumulates HTTP counters and per-database health samples in memory.
// It is safe for concurrent use; a background flusher drains it on an interval.
type Metrics struct {
	mu           sync.Mutex
	requests     int64
	errors       int64
	latencySumMs int64
	latencyCount int64
	dbs          map[string]*dbStats
}

func NewMetrics() *Metrics {
	return &Metrics{dbs: make(map[string]*dbStats)}
}

// Observe records one HTTP request by its status code and latency.
func (m *Metrics) Observe(status int, latency time.Duration) {
	m.mu.Lock()
	m.requests++
	if status >= 500 {
		m.errors++
	}
	m.latencySumMs += latency.Milliseconds()
	m.latencyCount++
	m.mu.Unlock()
}

// ObserveDatabase records one database health-check sample for the named
// database. A failed ping (err != nil) counts as an error and marks the database
// down; its latency is not included in the latency average.
func (m *Metrics) ObserveDatabase(database string, latency time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.dbs[database]
	if !ok {
		s = &dbStats{}
		m.dbs[database] = s
	}
	if err != nil {
		s.errors++
		s.up = false
		return
	}
	s.up = true
	s.latencySumMs += latency.Milliseconds()
	s.latencyCount++
}

// snapshot is one flush window of accumulated metrics.
type snapshot struct {
	requests      int64
	requestErrors int64
	latencySumMs  int64
	latencyCount  int64
	dbs           map[string]dbStats
}

// empty reports whether the window holds nothing worth publishing.
func (s snapshot) empty() bool {
	if s.requests > 0 || s.requestErrors > 0 {
		return false
	}
	for _, d := range s.dbs {
		if d.latencyCount > 0 || d.errors > 0 {
			return false
		}
	}
	return true
}

// drain atomically returns the accumulated counters and resets them to zero.
// The per-database `up` flag is deliberately NOT reset, so DatabaseUp keeps
// reporting the last known state even in a window with no samples.
func (m *Metrics) drain() snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap := snapshot{
		requests:      m.requests,
		requestErrors: m.errors,
		latencySumMs:  m.latencySumMs,
		latencyCount:  m.latencyCount,
		dbs:           make(map[string]dbStats, len(m.dbs)),
	}
	for name, s := range m.dbs {
		snap.dbs[name] = *s
		s.latencySumMs, s.latencyCount, s.errors = 0, 0, 0
	}
	m.requests, m.errors, m.latencySumMs, m.latencyCount = 0, 0, 0, 0
	return snap
}

// Publisher writes accumulated metrics to CloudWatch via PutMetricData.
type Publisher struct {
	client    *cloudwatch.Client
	namespace string
	service   string
}

// NewPublisher builds a CloudWatch publisher. When region is empty it returns
// (nil, nil) so the app can run without AWS (e.g. local dev); metrics then go to
// the log stream instead of CloudWatch.
func NewPublisher(ctx context.Context, region, namespace, service string) (*Publisher, error) {
	if region == "" {
		return nil, nil
	}
	if namespace == "" {
		namespace = defaultNamespace
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &Publisher{client: cloudwatch.NewFromConfig(cfg), namespace: namespace, service: service}, nil
}

// Flush writes one window to CloudWatch as single data points. HTTP metrics
// carry the Service dimension; database metrics additionally carry a Database
// dimension so each CockroachDB database (target_app / stock_db / order_db) can
// be watched on its own.
func (p *Publisher) Flush(ctx context.Context, snap snapshot) error {
	svcDim := types.Dimension{Name: aws.String("Service"), Value: aws.String(p.service)}

	var datums []types.MetricDatum
	if snap.requests > 0 {
		datums = append(datums,
			types.MetricDatum{
				MetricName: aws.String("Requests"),
				Value:      aws.Float64(float64(snap.requests)),
				Unit:       types.StandardUnit("Count"),
				Dimensions: []types.Dimension{svcDim},
			},
			types.MetricDatum{
				MetricName: aws.String("RequestErrors"),
				Value:      aws.Float64(float64(snap.requestErrors)),
				Unit:       types.StandardUnit("Count"),
				Dimensions: []types.Dimension{svcDim},
			},
		)
		if snap.latencyCount > 0 {
			datums = append(datums, types.MetricDatum{
				MetricName: aws.String("RequestLatency"),
				Value:      aws.Float64(avgMs(snap.latencySumMs, snap.latencyCount)),
				Unit:       types.StandardUnit("Milliseconds"),
				Dimensions: []types.Dimension{svcDim},
			})
		}
	}
	for name, d := range snap.dbs {
		dbDims := []types.Dimension{
			svcDim,
			{Name: aws.String("Database"), Value: aws.String(name)},
		}
		if d.latencyCount > 0 {
			datums = append(datums, types.MetricDatum{
				MetricName: aws.String("DatabaseLatency"),
				Value:      aws.Float64(avgMs(d.latencySumMs, d.latencyCount)),
				Unit:       types.StandardUnit("Milliseconds"),
				Dimensions: dbDims,
			})
		}
		if d.errors > 0 {
			datums = append(datums, types.MetricDatum{
				MetricName: aws.String("DatabaseErrors"),
				Value:      aws.Float64(float64(d.errors)),
				Unit:       types.StandardUnit("Count"),
				Dimensions: dbDims,
			})
		}
		up := 0.0
		if d.up {
			up = 1.0
		}
		datums = append(datums, types.MetricDatum{
			MetricName: aws.String("DatabaseUp"),
			Value:      aws.Float64(up),
			Unit:       types.StandardUnit("Count"),
			Dimensions: dbDims,
		})
	}
	if len(datums) == 0 {
		return nil
	}
	_, err := p.client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(p.namespace),
		MetricData: datums,
	})
	return err
}

// StartFlusher drains the metrics every interval and sends them — to CloudWatch
// when a publisher is set, otherwise to the log stream (so local dev still sees
// them in CloudWatch Logs). It runs until ctx is cancelled.
func StartFlusher(ctx context.Context, m *Metrics, p *Publisher, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			snap := m.drain()
			if snap.empty() {
				continue
			}
			if p != nil {
				if err := p.Flush(ctx, snap); err != nil {
					slog.Error("cloudwatch flush", "error", err)
				}
			} else {
				slog.Info("metrics",
					"requests", snap.requests,
					"errors", snap.requestErrors,
					"avg_latency_ms", avgMs(snap.latencySumMs, snap.latencyCount),
					"databases", dbLogValue(snap.dbs),
				)
			}
		}
	}()
}

// Pinger is the minimal dependency for a database health check; *pgxpool.Pool
// satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NamedPinger pairs a database name with its pinger, so per-database metrics are
// published with a Database dimension.
type NamedPinger struct {
	Database string
	Pinger   Pinger
}

// StartDBSampler pings each database every interval and records the result, so
// get_metric() can watch the databases (CockroachDB) themselves — not just HTTP.
// A hung database cannot stall the sampler: each ping gets a 5s timeout.
// Runs until ctx is cancelled.
func StartDBSampler(ctx context.Context, dbs []NamedPinger, m *Metrics, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			for _, db := range dbs {
				pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				start := time.Now()
				err := db.Pinger.Ping(pingCtx)
				cancel()
				m.ObserveDatabase(db.Database, time.Since(start), err)
			}
		}
	}()
}

func avgMs(sum, count int64) float64 {
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count)
}

// dbLogValue renders the per-database window for the log-stream fallback.
func dbLogValue(dbs map[string]dbStats) map[string]any {
	out := make(map[string]any, len(dbs))
	for name, d := range dbs {
		out[name] = map[string]any{
			"up":            d.up,
			"errors":        d.errors,
			"avg_latency_ms": avgMs(d.latencySumMs, d.latencyCount),
		}
	}
	return out
}
