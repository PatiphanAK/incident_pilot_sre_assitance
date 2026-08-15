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

// Metrics accumulates request counters and latencies in memory. It is safe for
// concurrent use; a background flusher drains it on an interval.
type Metrics struct {
	mu           sync.Mutex
	requests     int64
	errors       int64
	latencySumMs int64
	latencyCount int64
}

func NewMetrics() *Metrics { return &Metrics{} }

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

// drain atomically returns the accumulated counters and resets them to zero.
func (m *Metrics) drain() (requests, errors, latencySumMs, latencyCount int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	requests, errors, latencySumMs, latencyCount = m.requests, m.errors, m.latencySumMs, m.latencyCount
	m.requests, m.errors, m.latencySumMs, m.latencyCount = 0, 0, 0, 0
	return
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

// Flush writes one interval's counters to CloudWatch as single data points.
func (p *Publisher) Flush(ctx context.Context, requests, errors, latencySumMs, latencyCount int64) error {
	dim := []types.Dimension{{Name: aws.String("Service"), Value: aws.String(p.service)}}
	datums := []types.MetricDatum{
		{
			MetricName: aws.String("Requests"),
			Value:      aws.Float64(float64(requests)),
			Unit:       types.StandardUnit("Count"),
			Dimensions: dim,
		},
		{
			MetricName: aws.String("RequestErrors"),
			Value:      aws.Float64(float64(errors)),
			Unit:       types.StandardUnit("Count"),
			Dimensions: dim,
		},
	}
	if latencyCount > 0 {
		avg := float64(latencySumMs) / float64(latencyCount)
		datums = append(datums, types.MetricDatum{
			MetricName: aws.String("RequestLatency"),
			Value:      aws.Float64(avg),
			Unit:       types.StandardUnit("Milliseconds"),
			Dimensions: dim,
		})
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
			req, errc, latSum, latCount := m.drain()
			if req == 0 && errc == 0 {
				continue
			}
			switch {
			case p != nil:
				if err := p.Flush(ctx, req, errc, latSum, latCount); err != nil {
					slog.Error("cloudwatch flush", "error", err)
				}
			default:
				avg := 0.0
				if latCount > 0 {
					avg = float64(latSum) / float64(latCount)
				}
				slog.Info("metrics", "requests", req, "errors", errc, "avg_latency_ms", avg)
			}
		}
	}()
}
