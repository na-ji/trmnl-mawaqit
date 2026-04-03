package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	registeredUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "trmnl_registered_users",
		Help: "Current number of registered users.",
	})

	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trmnl_http_requests_total",
		Help: "Total number of HTTP requests by method, path, and status.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "trmnl_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	markupRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "trmnl_markup_requests_total",
		Help: "Total number of markup requests.",
	})

	markupCacheHitsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "trmnl_markup_cache_hits_total",
		Help: "Total number of markup requests served from cache.",
	})

	mawaqitAPICallsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "trmnl_mawaqit_api_calls_total",
		Help: "Total number of calls to the Mawaqit API.",
	})

	mawaqitAPICacheHitsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "trmnl_mawaqit_api_cache_hits_total",
		Help: "Total number of Mawaqit API calls served from cache.",
	})

	mawaqitAPIErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "trmnl_mawaqit_api_errors_total",
		Help: "Total number of failed Mawaqit API calls.",
	})
)
