package plugin


const (
	TestKeyURI = "projects/cloud-kms-lab/locations/us-central1/keyRings/ring-01/cryptoKeys/key-01"
)

var (
	metricsOfInterest = []string{
		"apiserver_cloudkms_kms_client_operation_latency_microseconds",
		// "apiserver_cloudkms_kms_client_operation_failures_total",
		"go_memstats_alloc_bytes_total",
		"go_memstats_frees_total",
		"process_cpu_seconds_total",
	}
)
