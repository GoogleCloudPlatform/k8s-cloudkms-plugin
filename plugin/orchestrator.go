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

import "time"

type Orchestrator struct {
	*Plugin
	healthzPath, healthzPort, metricsPath, metricsPort string
}

func NewOrchestrator(p *Plugin, healthzPath, healthzPort, metricsPath, metricsPort string) *Orchestrator {
	return &Orchestrator{
		Plugin: p,
		healthzPath: healthzPath,
		healthzPort: healthzPort,
		metricsPath: metricsPath,
		metricsPort: metricsPort,
	}
}
func (o *Orchestrator) Run() {
	v := NewValidator(o.Plugin)
	m := NewMetrics(o.healthzPath, o.healthzPort, o.metricsPath, o.metricsPort)

	v.mustValidatePrerequisites()

	o.mustServeKMSRequests()

	// Giving some time for kmsPlugin to start Serving.
	// TODO: Must be a better way than to sleep.
	time.Sleep(3 * time.Millisecond)

	v.mustPingRPC()

	m.MustServeMetrics()
}
