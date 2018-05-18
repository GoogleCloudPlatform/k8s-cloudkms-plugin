package plugin

const  (

	// HealthzPort port on which healthz status will be reported.
	HealthzPort = ":8081"
	// HealthzPath path on which healthz status will be reported.
	HealthzPath = "/healthz"

	// MetricsPort port on which metrics will be reported.
	MetricsPort = ":8082"
	// MetricsPath path on which metrics will be reported.
	MetricsPath = "/metrics"

	// KeyURIPattern regex for validating kms' key resource id.
    KeyURIPattern = `^projects\/[-a-zA-Z0-9_]*\/locations\/[-a-zA-Z0-9_]*\/keyRings\/[-a-zA-Z0-9_]*\/cryptoKeys\/[-a-zA-Z0-9_]*`

	// Unix Domain Socket
	netProtocol    = "unix"
	apiVersion     = "v1beta1"
	runtime        = "CloudKMS"
	runtimeVersion = "0.0.1"
)