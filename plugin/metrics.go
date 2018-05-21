/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import (
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "apiserver"
	subsystem = "kms"
)

var (
	cloudKMSOperationalLatencies = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "kms_plugin_roundtrip_latencies",
			Help:      "Latencies in milliseconds of cloud kms operations.",
		},
		[]string{"operation_type"},
	)

	cloudKMSOperationalFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "kms_plugin_failures_count",
			Help:      "Total number of failed kms operations.",
		},
		[]string{"operation_type"},
	)
)

func init() {
	prometheus.MustRegister(cloudKMSOperationalLatencies)
	prometheus.MustRegister(cloudKMSOperationalFailuresTotal)
}

// recordCloudKMSOperation records kms operational latencies.
func recordCloudKMSOperation(operationType string, start time.Time) {
	cloudKMSOperationalLatencies.WithLabelValues(operationType).Observe(sinceInMilliseconds(start))
}

func sinceInMilliseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds() / int64(time.Millisecond))
}

// metrics handles metrics related functionality of the plugin,including healthz and performance.
type metrics struct {
	healthzPath string
	healthzPort string
	metricsPath string
	metricsPort string
}

// newMetrics constructs Metrics
func newMetrics(healthzPath, healthzPort, metricsPath, metricsPort string) *metrics {
	return &metrics{
		healthzPath: healthzPath,
		healthzPort: healthzPort,
		metricsPath: metricsPath,
		metricsPort: metricsPort,
	}
}

// mustServeHealthzAndMetrics serves healthz and performance metrics or dies.
func (m *metrics) mustServeHealthzAndMetrics() {
	go m.mustServeHealthz()
	go m.mustServeMetrics()
}

func (m *metrics) mustServeHealthz() {
	serverHealthz := http.NewServeMux()
	serverHealthz.HandleFunc(m.healthzPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	glog.Infof("Registering healthz listener at http://localhost:%s%s", m.healthzPort, m.healthzPath)
	glog.Fatal(http.ListenAndServe(m.healthzPort, serverHealthz))
}

func (m *metrics) mustServeMetrics() {
	serverMetrics := http.NewServeMux()
	serverMetrics.Handle(m.metricsPath, promhttp.Handler())
	glog.Infof("Registering metrics listener at http://localhost:%s%s", m.metricsPort, m.metricsPath)
	glog.Fatal(http.ListenAndServe(m.metricsPort, serverMetrics))
}
