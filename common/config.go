package common

import "github.com/spf13/viper"

// ===============================================================================
// Common Submodule Config

// HTTPServerTimeoutConfig defines the timeout settings for HTTP server
type HTTPServerTimeoutConfig struct {
	// ReadTimeout is the maximum duration for reading the entire
	// request, including the body in seconds. A zero or negative
	// value means there will be no timeout.
	ReadTimeout int `mapstructure:"read" json:"read" validate:"gte=0"`
	// WriteTimeout is the maximum duration before timing out
	// writes of the response in seconds. A zero or negative value
	// means there will be no timeout.
	WriteTimeout int `mapstructure:"write" json:"write" validate:"gte=0"`
	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled in seconds. If
	// IdleTimeout is zero, the value of ReadTimeout is used. If
	// both are zero, there is no timeout.
	IdleTimeout int `mapstructure:"idle" json:"idle" validate:"gte=0"`
}

// HTTPServerConfig defines the HTTP server parameters
type HTTPServerConfig struct {
	// ListenOn is the interface the HTTP server will listen on
	ListenOn string `mapstructure:"listenOn" json:"listenOn" validate:"required,ip"`
	// Port is the port the HTTP server will listen on
	Port uint16 `mapstructure:"appPort" json:"appPort" validate:"required,gt=0,lt=65536"`
	// Timeouts sets the HTTP timeout settings
	Timeouts HTTPServerTimeoutConfig `mapstructure:"timeoutSecs" json:"timeoutSecs" validate:"required,dive"`
}

// HTTPRequestLogging defines HTTP request logging parameters
type HTTPRequestLogging struct {
	// RequestIDHeader is the HTTP header containing the API request ID
	RequestIDHeader string `mapstructure:"requestIDHeader" json:"requestIDHeader"`
	// DoNotLogHeaders is the list of headers to not include in logging metadata
	DoNotLogHeaders []string `mapstructure:"skipHeaders" json:"skipHeaders"`
}

// EndpointConfig defines API endpoint config
type EndpointConfig struct {
	// PathPrefix is the end-point path prefix for the APIs
	PathPrefix string `mapstructure:"pathPrefix" json:"pathPrefix" validate:"required"`
}

// APIConfig defines API settings for a submodule
type APIConfig struct {
	// Endpoint sets API endpoint related parameters
	Endpoint EndpointConfig `mapstructure:"endPoint" json:"endPoint" validate:"required,dive"`
	// RequestLogging sets API request logging parameters
	RequestLogging HTTPRequestLogging `mapstructure:"requestLogging" json:"requestLogging" validate:"required,dive"`
}

// APIServerConfig defines HTTP API / server parameters
type APIServerConfig struct {
	// Enabled whether this API is enabled
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	// Server defines HTTP server parameters
	Server HTTPServerConfig `mapstructure:"service" json:"service" validate:"required_with=Enabled,dive"`
	// APIs defines API settings for a submodule
	APIs APIConfig `mapstructure:"apis" json:"apis" validate:"required_with=Enabled,dive"`
}

// ===============================================================================
// Persistence Configuration Structures

// PostgresSSLConfig Postgres connection SSL config
type PostgresSSLConfig struct {
	// Enabled whether to enable SSL when connecting to Postgres
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	// CAFile the CA cert file to challenge remote with
	CAFile *string `mapstructure:"caFile" json:"caFile,omitempty" validate:"omitempty,file"`
}

// PostgresConfig Postgres connection config
type PostgresConfig struct {
	// Host Postgres server host
	Host string `mapstructure:"host" json:"host" validate:"required"`
	// Port Postgres server port
	Port uint16 `mapstructure:"port" json:"port" validate:"lte=65535,gte=0"`
	// Database the specific database to use
	Database string `mapstructure:"db" json:"db" validate:"required"`
	// User the user to connect with
	User string `mapstructure:"user" json:"user" validate:"required"`
	// SSL the connection SSL settings
	SSL PostgresSSLConfig `mapstructure:"ssl" json:"ssl" validate:"required,dive"`
}

// ===============================================================================
// GCP PubSub Configuration Structures

// PubSubSubcriptionConfig PubSub subscription config
type PubSubSubcriptionConfig struct {
	// Topic the pubsub topic to subscribe to
	Topic string `mapstructure:"topic" json:"topic" validate:"required"`
	// MsgTTLInSec message retention within the subscription in secs
	MsgTTLInSec uint32 `mapstructure:"msgTTL" json:"msgTTL" validate:"gte=600,lte=604800"`
}

// ReqRespClientConfig PubSub request-response client config
type ReqRespClientConfig struct {
	// GCPProject the GCP project operate in
	GCPProject string `mapstructure:"gcpProject" json:"gcpProject" validate:"required"`
	// InboudRequestTopic the PubSub topic this client listen on for inboud request
	InboudRequestTopic PubSubSubcriptionConfig `mapstructure:"self" json:"self" validate:"required,dive"`
	// SupportWorkerCount number of support workers to spawn to process inbound requests
	SupportWorkerCount int `mapstructure:"supportWorkerCount" json:"supportWorkerCount" validate:"required"`
	// RequestTimeoutEnforceIntInSec outbound request timeout enforcement check interval in secs
	RequestTimeoutEnforceIntInSec uint32 `mapstructure:"requestTimeoutEnforceIntInSec" json:"requestTimeoutEnforceIntInSec" validate:"gte=15,lte=120"`
}

// ===============================================================================
// Major System Component Configuration Structures

// SystemManagementConfig define control node management sub-module config
type SystemManagementConfig struct {
	// APIServer management REST API server config
	APIServer APIServerConfig `mapstructure:"api" json:"api" validate:"required,dive"`
	// RRClient PibSub request-response client config
	RRClient ReqRespClientConfig `mapstructure:"requestResponse" json:"requestResponse" validate:"required,dive"`
}

// ===============================================================================
// Complete Configuration Structures

// ControlNodeConfig define control node settings and behavior
type ControlNodeConfig struct {
	// Postgres postgres DB configuration
	Postgres PostgresConfig `mapstructure:"postgres" json:"postgres" validate:"required,dive"`
	// Management management
	Management SystemManagementConfig `mapstructure:"management" json:"management" validate:"required,dive"`
}

// EdgeNodeConfig define edge node settings and behavior
type EdgeNodeConfig struct {
	// LiveStreamServer local live stream server config
	LiveStreamServer APIServerConfig `mapstructure:"localLiveStream" json:"localLiveStream" validate:"required,dive"`
}

// ===============================================================================
// Default Configuration Setter

// InstallDefaultControlNodeConfigValues installs default config parameters in viper for
// control node
func InstallDefaultControlNodeConfigValues() {
	// Default Postgres config
	viper.SetDefault("postgres.port", 5432)
	viper.SetDefault("postgres.ssl.enabled", false)

	// Default management sub-component
	// Default management HTTP server config
	viper.SetDefault("management.api.enabled", true)
	viper.SetDefault("management.api.service.listenOn", "0.0.0.0")
	viper.SetDefault("management.api.service.appPort", 8080)
	viper.SetDefault("management.api.service.timeoutSecs.read", 60)
	viper.SetDefault("management.api.service.timeoutSecs.write", 60)
	viper.SetDefault("management.api.service.timeoutSecs.idle", 60)
	viper.SetDefault("management.api.apis.endPoint.pathPrefix", "/")
	viper.SetDefault("management.api.apis.requestLogging.requestIDHeader", "X-Request-ID")
	viper.SetDefault("management.api.apis.requestLogging.skipHeaders", []string{
		"WWW-Authenticate", "Authorization", "Proxy-Authenticate", "Proxy-Authorization",
	})
	// Default system manager request-response client config
	viper.SetDefault("management.requestResponse.self.msgTTL", 600)
	viper.SetDefault("management.requestResponse.supportWorkerCount", 4)
	viper.SetDefault("management.requestResponse.requestTimeoutEnforceIntInSec", 30)
}
