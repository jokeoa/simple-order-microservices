package metrics

import (
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewGoRuntimeCollector(service string) Collector {
	return NewGaugeFunc(
		"ads2_go_goroutines",
		"Current number of goroutines in the service process.",
		[]string{"service"},
		func() []GaugeSample {
			return []GaugeSample{{
				LabelValues: []string{service},
				Value:       float64(runtime.NumGoroutine()),
			}}
		},
	)
}

func NewPostgresPoolConnectionsCollector(service string, pool *pgxpool.Pool) Collector {
	return NewGaugeFunc(
		"ads2_postgres_pool_connections",
		"Current Postgres pool connection counts by state.",
		[]string{"service", "state"},
		func() []GaugeSample {
			stats := pool.Stat()

			return []GaugeSample{
				{LabelValues: []string{service, "acquired"}, Value: float64(stats.AcquiredConns())},
				{LabelValues: []string{service, "constructing"}, Value: float64(stats.ConstructingConns())},
				{LabelValues: []string{service, "idle"}, Value: float64(stats.IdleConns())},
				{LabelValues: []string{service, "total"}, Value: float64(stats.TotalConns())},
				{LabelValues: []string{service, "max"}, Value: float64(stats.MaxConns())},
			}
		},
	)
}

func NewPostgresPoolUtilizationCollector(service string, pool *pgxpool.Pool) Collector {
	return NewGaugeFunc(
		"ads2_postgres_pool_utilization_ratio",
		"Current ratio of acquired Postgres connections to configured pool maximum.",
		[]string{"service"},
		func() []GaugeSample {
			stats := pool.Stat()
			maxConns := stats.MaxConns()
			utilization := 0.0
			if maxConns > 0 {
				utilization = float64(stats.AcquiredConns()) / float64(maxConns)
			}

			return []GaugeSample{{
				LabelValues: []string{service},
				Value:       utilization,
			}}
		},
	)
}
