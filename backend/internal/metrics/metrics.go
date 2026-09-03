package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var Registry = prometheus.NewRegistry()

var (
	TotalBuilds = promauto.With(Registry).NewCounter(prometheus.CounterOpts{
		Name: "moduforge_builds_total", Help: "Total builds",
	})
	BuildsByTarget = promauto.With(Registry).NewCounterVec(prometheus.CounterOpts{
		Name: "moduforge_builds_by_target_total", Help: "Builds by target",
	}, []string{"target"})
	BuildsByArch = promauto.With(Registry).NewCounterVec(prometheus.CounterOpts{
		Name: "moduforge_builds_by_arch_total", Help: "Builds by arch",
	}, []string{"arch"})
	BuildSuccesses = promauto.With(Registry).NewCounter(prometheus.CounterOpts{
		Name: "moduforge_build_successes_total", Help: "Successful builds",
	})
	BuildFailures = promauto.With(Registry).NewCounter(prometheus.CounterOpts{
		Name: "moduforge_build_failures_total", Help: "Failed builds",
	})
	BuildDuration = promauto.With(Registry).NewHistogram(prometheus.HistogramOpts{
		Name:    "moduforge_build_duration_seconds",
		Help:    "Build duration in seconds",
		Buckets: prometheus.DefBuckets,
	})
	CacheHits = promauto.With(Registry).NewCounter(prometheus.CounterOpts{
		Name: "moduforge_cache_hits_total", Help: "Cache hits during builds",
	})
	CacheMisses = promauto.With(Registry).NewCounter(prometheus.CounterOpts{
		Name: "moduforge_cache_misses_total", Help: "Cache misses during builds",
	})
)

func init() {
	// Ensure global registry also gets these
	prometheus.MustRegister(BuildDuration, CacheHits, CacheMisses)

	// Register all custom-metric counters with the world-wide default Registry.
	globalCounters := []prometheus.Collector{
		TotalBuilds, BuildsByTarget, BuildsByArch, BuildSuccesses, BuildFailures,
	}
	for _, c := range globalCounters {
		prometheus.MustRegister(c)
	}
}
