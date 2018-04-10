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
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

const (
	namespace = "apiserver"
	subsystem = "cloudkms"
)

var (
	CloudKMSOperationalLatencies = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "kms_client_operation_latency_microseconds",
			Help:      "Latency in microseconds of cloud kms operations.",
		},
		[]string{"operation_type"},
	)

	CloudKMSOperationalFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "kms_client_operation_failures_total",
			Help:      "Total number of failed kms operations.",
		},
		[]string{"operation_type"},
	)
)

var registerMetrics sync.Once

func RegisterMetrics() {
	registerMetrics.Do(func() {
		prometheus.MustRegister(CloudKMSOperationalLatencies)
	})
}

func RecordCloudKMSOperation(operationType string, start time.Time) {
	CloudKMSOperationalLatencies.WithLabelValues(operationType).Observe(sinceInMicroseconds(start))
}

func sinceInMicroseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds() / time.Microsecond.Nanoseconds())
}

func MustServeHealthz(healthzPath , healthzPort string) {
	serverHealthz := http.NewServeMux()
	serverHealthz.HandleFunc(healthzPath, func (w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	glog.Infof("Registering healthz listener at http://localhost:%s%s", healthzPort, healthzPath)
	glog.Fatal(http.ListenAndServe(healthzPort, serverHealthz))
}

func MustServeMetrics(metricsPath, metricsPort string) {
	serverMetrics := http.NewServeMux()
	serverMetrics.Handle(metricsPath, promhttp.Handler())
	glog.Infof("Registering metrics listener at http://localhost:%s%s", metricsPort, metricsPath)
	glog.Fatal(http.ListenAndServe(metricsPort, serverMetrics))
}



