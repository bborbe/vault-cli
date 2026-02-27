// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	libtime "github.com/bborbe/time"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	buildInfo = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "trading",
			Name:      "build_info",
			Help:      "Build timestamp as Unix time. Service identified by Prometheus job label.",
		},
	)
)

func init() {
	prometheus.MustRegister(buildInfo)
}

// BuildInfoMetrics provides metrics tracking for build information.
// It records build timestamps to Prometheus for monitoring deployment history.
//
//counterfeiter:generate -o ../mocks/build-info-metrics.go --fake-name BuildInfoMetrics . BuildInfoMetrics
type BuildInfoMetrics interface {
	SetBuildInfo(buildDate *libtime.DateTime)
}

// NewBuildInfoMetrics creates a new BuildInfoMetrics implementation.
func NewBuildInfoMetrics() BuildInfoMetrics {
	return &buildInfoMetrics{}
}

// buildInfoMetrics implements BuildInfoMetrics interface using Prometheus gauges.
type buildInfoMetrics struct{}

// SetBuildInfo records the build timestamp as a Unix timestamp metric.
// If buildDate is nil, no metric is recorded.
func (m *buildInfoMetrics) SetBuildInfo(buildDate *libtime.DateTime) {
	if buildDate == nil {
		return // Skip metric recording when build date is not provided
	}
	buildInfo.Set(float64(buildDate.Unix()))
}
