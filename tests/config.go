package tests


const (
	TestKeyURI = "projects/cloud-kms-lab/locations/us-central1/keyRings/ring-01/cryptoKeys/key-01"
)

var (
	MetricsOfInterest = []string{
		"apiserver_kms_kms_plugin_roundtrip_latencies",
		// "apiserver_kms_kms_plugin_failures_total",
		"go_memstats_alloc_bytes_total",
		"go_memstats_frees_total",
		"process_cpu_seconds_total",
	}
)
