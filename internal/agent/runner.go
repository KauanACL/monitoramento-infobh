package agent

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Runner struct {
	cfg       Config
	client    *Client
	collector *Collector
	log       *slog.Logger
}

func NewRunner(cfg Config, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	normalizeDurations(&cfg)
	return &Runner{
		cfg:       cfg,
		client:    NewClient(cfg),
		collector: NewCollector(),
		log:       logger,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Add(3)
	go r.loop(ctx, &wg, "heartbeat", r.cfg.HeartbeatEvery, r.sendHeartbeat)
	go r.loop(ctx, &wg, "metrics", r.cfg.MetricsEvery, r.sendMetrics)
	go r.loop(ctx, &wg, "devices", r.cfg.DevicesEvery, r.sendDevices)
	wg.Wait()
	return ctx.Err()
}

func (r *Runner) Once(ctx context.Context) error {
	if err := r.sendHeartbeat(ctx); err != nil {
		return err
	}
	if err := r.sendMetrics(ctx); err != nil {
		return err
	}
	return r.sendDevices(ctx)
}

func (r *Runner) loop(ctx context.Context, wg *sync.WaitGroup, name string, interval time.Duration, fn func(context.Context) error) {
	defer wg.Done()
	if err := fn(ctx); err != nil {
		r.log.Warn("agent collection failed", "kind", name, "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				r.log.Warn("agent collection failed", "kind", name, "error", err)
			}
		}
	}
}

func (r *Runner) sendHeartbeat(ctx context.Context) error {
	return r.client.SendHeartbeat(ctx, r.collector.Heartbeat(ctx))
}

func (r *Runner) sendMetrics(ctx context.Context) error {
	return r.client.SendMetrics(ctx, r.collector.Metrics(ctx))
}

func (r *Runner) sendDevices(ctx context.Context) error {
	return r.client.SendDevices(ctx, r.collector.Devices(ctx))
}
