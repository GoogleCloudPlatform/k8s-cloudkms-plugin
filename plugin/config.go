package plugin

const  (

	HealthzPort = ":8081"
	HealthzPath = "/healthz"

	MetricsPort = ":8082"
	MetricsPath = "/metrics"

	PathToUnixSocket = "/tmp/kms-plugin.socket"

    KeyURIPattern = `^projects\/[-a-zA-Z0-9_]*\/locations\/[-a-zA-Z0-9_]*\/keyRings\/[-a-zA-Z0-9_]*\/cryptoKeys\/[-a-zA-Z0-9_]*`

	// Unix Domain Socket
	netProtocol    = "unix"
	APIVersion     = "v1beta1"
	runtime        = "CloudKMS"
	runtimeVersion = "0.0.1"
)