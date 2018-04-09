package plugin

const  (
	MetricsPort = ":8081"
	MetricsPath = "/metrics"

	HealthzPort = ":8082"
	HealthzPath = "/healthz"

	PathToUnixSocket = "/tmp/kms-plugin.socket"

    KeyURIPattern = `^projects\/[-a-zA-Z0-9_]*\/locations\/[-a-zA-Z0-9_]*\/keyRings\/[-a-zA-Z0-9_]*\/cryptoKeys\/[-a-zA-Z0-9_]*`

	// Unix Domain Socket
	netProtocol    = "unix"
	APIVersion     = "v1beta1"
	runtime        = "CloudKMS"
	runtimeVersion = "0.0.1"
)