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
)

const (
	cloudKMSSubsystem = "cloudkms"
)

var (
	CloudKMSClientOperationalLatencies = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Subsystem: cloudKMSSubsystem,
			Name:      "kms_client_operation_latency_microseconds",
			Help:      "Latency in microseconds of cloud kms operations.",
		},
		[]string{"operation_type"},
	)
)

var registerMetrics sync.Once

func RegisterMetrics() {
	registerMetrics.Do(func() {
		prometheus.MustRegister(CloudKMSClientOperationalLatencies)
	})
}

func RecordCloudKMSOperation(operationType string, start time.Time) {
	CloudKMSClientOperationalLatencies.WithLabelValues(operationType).Observe(sinceInMicroseconds(start))
}

func sinceInMicroseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds() / time.Microsecond.Nanoseconds())
}


