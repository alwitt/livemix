package common

import (
	"fmt"
	"time"

	"github.com/alwitt/goutils"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/viper"
)

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
	// LogLevel output request logs at this level
	LogLevel goutils.HTTPRequestLogLevel `mapstructure:"logLevel" json:"logLevel" validate:"oneof=warn info debug"`
	// HealthLogLevel output health check logs at this level
	HealthLogLevel goutils.HTTPRequestLogLevel `mapstructure:"healthLogLevel" json:"healthLogLevel" validate:"oneof=warn info debug"`
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

// VideoSourceConfig define a HLS video source being monitored
type VideoSourceConfig struct {
	// Name video source system entry name (as recorded by the system control node)
	Name string `mapstructure:"name" json:"name" validate:"required"`
	// StatusReportIntInSec interval in secs between video source status report to system controller
	StatusReportIntInSec uint32 `mapstructure:"statusReportIntInSec" json:"statusReportIntInSec" validate:"gte=10"`
}

// StatusReportInt convert StatusReportIntInSec to time.Duration
func (c VideoSourceConfig) StatusReportInt() time.Duration {
	return time.Second * time.Duration(c.StatusReportIntInSec)
}

// HTTPClientAuthConfig HTTP client OAuth middleware configuration
//
// Currently only support client-credential OAuth flow configuration
type HTTPClientAuthConfig struct {
	// IssuerURL OpenID provider issuer URL
	IssuerURL string `mapstructure:"issuerURL" json:"issuerURL" validate:"required,url"`
	// ClientID OAuth client ID
	ClientID string `json:"clientID" validate:"required"`
	// ClientSecret OAuth client secret
	ClientSecret string `json:"clientSecret" validate:"required"`
	// TargetAudience target audience `aud` to acquire a token for
	TargetAudience string `mapstructure:"targetAudience" json:"targetAudience" validate:"required,url"`
}

// HTTPClientRetryConfig HTTP client config retry configuration
type HTTPClientRetryConfig struct {
	// MaxAttempts max number of retry attempts
	MaxAttempts int `mapstructure:"maxAttempts" json:"maxAttempts" validate:"gte=0"`
	// InitWaitTimeInSec wait time before the first wait retry
	InitWaitTimeInSec uint32 `mapstructure:"initialWaitTimeInSec" json:"initialWaitTimeInSec" validate:"gte=1"`
	// MaxWaitTimeInSec max wait time
	MaxWaitTimeInSec uint32 `mapstructure:"maxWaitTimeInSec" json:"maxWaitTimeInSec" validate:"gte=1"`
}

// InitWaitTime convert InitWaitTimeInSec to time.Duration
func (c HTTPClientRetryConfig) InitWaitTime() time.Duration {
	return time.Second * time.Duration(c.InitWaitTimeInSec)
}

// MaxWaitTime convert MaxWaitTimeInSec to time.Duration
func (c HTTPClientRetryConfig) MaxWaitTime() time.Duration {
	return time.Second * time.Duration(c.MaxWaitTimeInSec)
}

// HTTPClientConfig HTTP client config targeting `go-resty`
type HTTPClientConfig struct {
	// OAuth OAuth middleware integration configuration
	OAuth *HTTPClientAuthConfig `mapstructure:"oauth,omitempty" json:"oauth,omitempty" validate:"omitempty,dive"`
	// Retry client retry configuration. See https://github.com/go-resty/resty#retries for details
	Retry HTTPClientRetryConfig `mapstructure:"retry" json:"retry" validate:"required,dive"`
}

// MetricsFeatureConfig metrics framework features config
type MetricsFeatureConfig struct {
	// EnableAppMetrics whether to enable Golang application metrics
	EnableAppMetrics bool `mapstructure:"enableAppMetrics" json:"enableAppMetrics"`
	// EnableHTTPMetrics whether to enable HTTP request tracking metrics
	EnableHTTPMetrics bool `mapstructure:"enableHTTPMetrics" json:"enableHTTPMetrics"`
	// EnablePubSubMetrics whether to enable PubSub operational metrics
	EnablePubSubMetrics bool `mapstructure:"enablePubSubMetrics" json:"enablePubSubMetrics"`
}

// MetricsConfig application metrics config
type MetricsConfig struct {
	// Server defines HTTP server parameters
	Server HTTPServerConfig `mapstructure:"service" json:"service" validate:"required_with=Enabled,dive"`
	// MetricsEndpoint path to host the Prometheus metrics endpoint
	MetricsEndpoint string `mapstructure:"metricsEndpoint" json:"metricsEndpoint" validate:"required"`
	// MaxRequests max number of metrics requests in parallel to support
	MaxRequests int `mapstructure:"maxRequests" json:"maxRequests" validate:"gte=1"`
	// Features metrics framework features to enable
	Features MetricsFeatureConfig `mapstructure:"features" json:"features" validate:"gte=1"`
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

// SqliteConfig sqlite config
type SqliteConfig struct {
	// DBFile the sqlite DB file path
	DBFile string `mapstructure:"db" json:"db" validate:"required"`
}

// S3Credentials S3 credentials
type S3Credentials struct {
	// AccessKey user access key
	AccessKey string
	// SecretAccessKey user secret access key
	SecretAccessKey string
}

// S3Config S3 object store config
type S3Config struct {
	// ServerEndpoint S3 server endpoint
	ServerEndpoint string `mapstructure:"endpoint" json:"endpoint" validate:"required"`
	// UseTLS whether to TLS when connecting
	UseTLS bool `mapstructure:"useTLS" json:"useTLS"`
	// Creds S3 credentials
	Creds *S3Credentials `mapstructure:"creds" json:"creds,omitempty" validate:"omitempty,dive"`
}

// RecordingStorageConfig video recording storage config
type RecordingStorageConfig struct {
	// S3 object store config
	S3 S3Config `mapstructure:"s3" json:"s3" validate:"required,dive"`
	// StorageBucket the storage bucket to place all the recording video segments in
	StorageBucket string `mapstructure:"bucket" json:"bucket" validate:"required"`
	// StorageObjectPrefix the prefix used when defining the object key to store a segment with
	StorageObjectPrefix string `mapstructure:"objectPrefix" json:"objectPrefix" validate:"required"`
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

// MsgTTL convert MsgTTLInSec to time.Duration
func (c PubSubSubcriptionConfig) MsgTTL() time.Duration {
	return time.Second * time.Duration(c.MsgTTLInSec)
}

// ReqRespClientConfig PubSub request-response client config
type ReqRespClientConfig struct {
	// GCPProject the GCP project to operate in
	GCPProject string `mapstructure:"gcpProject" json:"gcpProject" validate:"required"`
	// InboudRequestTopic the PubSub topic this client listen on for inbound request
	InboudRequestTopic PubSubSubcriptionConfig `mapstructure:"self" json:"self" validate:"required,dive"`
	// SupportWorkerCount number of support workers to spawn to process inbound requests
	SupportWorkerCount int `mapstructure:"supportWorkerCount" json:"supportWorkerCount" validate:"required"`
	// MaxOutboundRequestDurationInSec the max duration for outbound request in secs
	MaxOutboundRequestDurationInSec uint32 `mapstructure:"outboundRequestDurationInSec" json:"outboundRequestDurationInSec" validate:"gte=5,lte=60"`
	// RequestTimeoutEnforceIntInSec outbound request timeout enforcement check interval in secs
	RequestTimeoutEnforceIntInSec uint32 `mapstructure:"requestTimeoutEnforceIntInSec" json:"requestTimeoutEnforceIntInSec" validate:"gte=15,lte=120"`
}

// MaxOutboundRequestDuration convert MaxOutboundRequestDurationInSec to time.Duration
func (c ReqRespClientConfig) MaxOutboundRequestDuration() time.Duration {
	return time.Second * time.Duration(c.MaxOutboundRequestDurationInSec)
}

// RequestTimeoutEnforceInt convert RequestTimeoutEnforceIntInSec to time.Duration
func (c ReqRespClientConfig) RequestTimeoutEnforceInt() time.Duration {
	return time.Second * time.Duration(c.RequestTimeoutEnforceIntInSec)
}

// EdgeReqRespClientConfig PubSub request-response client config for edge-to-control requests
type EdgeReqRespClientConfig struct {
	ReqRespClientConfig `mapstructure:",squash"`
	// ControlRRTopic the PubSub topic for reaching system control
	ControlRRTopic string `mapstructure:"systemControlTopic" json:"systemControlTopic" validate:"required"`
}

// BroadcastSystemConfig system broadcast channel configuration
type BroadcastSystemConfig struct {
	// PubSub broadcast PubSub settings
	PubSub PubSubSubcriptionConfig `mapstructure:"pubsub" json:"pubsub" validate:"required,dive"`
}

// ===============================================================================
// Video Segment Cache Configuration Structures

// RAMSegmentCacheConfig in-memory video segment cache config
type RAMSegmentCacheConfig struct {
	// RetentionCheckIntInSec cache entry retention check interval in secs
	RetentionCheckIntInSec uint32 `mapstructure:"retentionCheckIntInSec" json:"retentionCheckIntInSec" validate:"gte=10,lte=300"`
}

// RetentionCheckInt convert RetentionCheckIntInSec to time.Duration
func (c RAMSegmentCacheConfig) RetentionCheckInt() time.Duration {
	return time.Second * time.Duration(c.RetentionCheckIntInSec)
}

// MemcachedSegementCacheConfig memcached video segment cache config
type MemcachedSegementCacheConfig struct {
	// Servers list of memcached servers to establish connection with
	Servers []string `mapstructure:"servers" json:"servers" validate:"required,gte=1"`
}

// ===============================================================================
// Video Segment Forwarding Configuration Structures

// HTTPForwarderTargetConfig HTTP video forwarder target config
type HTTPForwarderTargetConfig struct {
	// TargetURL URL to send new segments to
	TargetURL string `mapstructure:"targetURL" json:"targetURL" validate:"required,url"`
	// RequestIDHeader request ID header name to set when forwarding
	RequestIDHeader string `mapstructure:"requestIDHeader" json:"requestIDHeader" validate:"required"`
	// Client HTTP client config. This is designed to support `go-resty`
	Client HTTPClientConfig `mapstructure:"client" json:"client" validate:"required,dive"`
}

// LiveStreamVideoForwarderConfig HTTP video forward supporting live stream through control node
type LiveStreamVideoForwarderConfig struct {
	// MaxInFlight max number of segments to forward in parallel
	MaxInFlight int `mapstructure:"maxInFlightSegments" json:"maxInFlightSegments" validate:"required,gte=1"`
	// Remote HTTP forwarder target config
	Remote HTTPForwarderTargetConfig `mapstructure:"remote" json:"remote" validate:"required,dive"`
}

// RecordingVideoForwarderConfig S3 video forward supporting recording video segments
type RecordingVideoForwarderConfig struct {
	// MaxInFlight max number of segments to forward in parallel
	MaxInFlight int `mapstructure:"maxInFlightSegments" json:"maxInFlightSegments" validate:"required,gte=1"`
	// RecordingStorage video recording storage config
	RecordingStorage RecordingStorageConfig `mapstructure:"storage" json:"storage" validate:"required,dive"`
}

// VideoForwarderConfig video segment forwarding config
type VideoForwarderConfig struct {
	// Live HTTP video forward supporting live stream through control node
	Live LiveStreamVideoForwarderConfig `mapstructure:"live" json:"live" validate:"required,dive"`
	// Recording support video recording video segment forwarding to object store
	Recording RecordingVideoForwarderConfig `mapstructure:"recording" json:"recording" validate:"required,dive"`
}

// ===============================================================================
// Major System Component Configuration Structures

// VideoSourceManagementConfig video source management settings
type VideoSourceManagementConfig struct {
	// StatusReportMaxDelayInSec maximum delay a source status report can be before the
	// control node marks source as no longer connected
	StatusReportMaxDelayInSec uint32 `mapstructure:"statusReportMaxDelayInSec" json:"statusReportMaxDelayInSec" validate:"required,gte=20"`
}

// StatusReportMaxDelay convert StatusReportMaxDelayInSec to time.Duration
func (c VideoSourceManagementConfig) StatusReportMaxDelay() time.Duration {
	return time.Second * time.Duration(c.StatusReportMaxDelayInSec)
}

// RecordingManagementConfig recording session management settings
type RecordingManagementConfig struct {
	// SegmentCleanupIntInSec The number of seconds between recording segment cleanup runs.
	// During each run, recording segments not associated with any recordings are purged.
	SegmentCleanupIntInSec uint32 `mapstructure:"segmentCleanupIntInSec" json:"segmentCleanupIntInSec" validate:"required,gte=60"`
}

// SegmentCleanupInt convert SegmentCleanupIntInSec ti time.Duration
func (c RecordingManagementConfig) SegmentCleanupInt() time.Duration {
	return time.Second * time.Duration(c.SegmentCleanupIntInSec)
}

// SystemManagementConfig define control node management sub-module config
type SystemManagementConfig struct {
	// APIServer management REST API server config
	APIServer APIServerConfig `mapstructure:"api" json:"api" validate:"required,dive"`
	// RRClient PubSub request-response client config
	RRClient ReqRespClientConfig `mapstructure:"requestResponse" json:"requestResponse" validate:"required,dive"`
	// SourceManagement video source management settings
	SourceManagement VideoSourceManagementConfig `mapstructure:"videoSourceManagement" json:"videoSourceManagement" validate:"required,dive"`
	// RecordingManagement recording session management settings
	RecordingManagement RecordingManagementConfig `mapstructure:"recordingManagement" json:"recordingManagement" validate:"required,dive"`
}

// HLSMonitorConfig HLS video source monitoring config
type HLSMonitorConfig struct {
	// DefaultSegmentURIPrefix optionally set default video segment URI prefix
	DefaultSegmentURIPrefix *string `mapstructure:"defaultSegmentURIPrefix,omitempty" json:"defaultSegmentURIPrefix,omitempty" validate:"omitempty,uri"`
	// TrackingWindowInSec tracking window is the duration in time a video segment is tracked.
	// After observing a new segment, that segment is remembered for the duration of a
	// tracking window, and forgotten after that.
	TrackingWindowInSec uint32 `mapstructure:"trackingWindow" json:"trackingWindow" validate:"gte=10,lte=300"`
	// SegmentReaderWorkerCount number of worker threads in the video segment reader
	SegmentReaderWorkerCount int `mapstructure:"segmentReaderWorkerCount" json:"segmentReaderWorkerCount" validate:"gte=2,lte=64"`
}

// TrackingWindow convert TrackingWindowInSec to time.Duration
func (c HLSMonitorConfig) TrackingWindow() time.Duration {
	return time.Second * time.Duration(c.TrackingWindowInSec)
}

// VODServerConfig HLS VOD server config
type VODServerConfig struct {
	// LiveVODSegmentCount number of video segments to include when building a live
	// VOD playlist.
	LiveVODSegmentCount int `mapstructure:"liveVODSegmentCount" json:"liveVODSegmentCount" validate:"gte=1"`
	// SegmentCacheTTLInSec when a segment is read from cold storage, store it in
	// local cache with this TTL in secs.
	SegmentCacheTTLInSec uint32 `mapstructure:"segmentCacheTTLInSec" json:"segmentCacheTTLInSec" validate:"gte=30,lte=7200"`
}

// SegmentCacheTTL convert SegmentCacheTTLInSec to time.Duration
func (c VODServerConfig) SegmentCacheTTL() time.Duration {
	return time.Second * time.Duration(c.SegmentCacheTTLInSec)
}

// CentralVODServerConfig VOD server running on the control node
type CentralVODServerConfig struct {
	VODServerConfig `mapstructure:",squash"`
	// APIServer HLS VOD REST API server config
	APIServer APIServerConfig `mapstructure:"api" json:"api" validate:"required,dive"`
	// Cache memcached based video segment cache
	Cache MemcachedSegementCacheConfig `mapstructure:"cache" json:"cache" validate:"required,dive"`
	// SegReceiverTrackingWindowInSec Tracking window is the duration in time a video segment is
	// tracked. Recorded segments are forgotten after this tracking window.
	SegReceiverTrackingWindowInSec uint32 `mapstructure:"segmentReceiverTrackingWindow" json:"segmentReceiverTrackingWindow" validate:"gte=10,lte=300"`
	// RecordingStorage video recording storage config
	RecordingStorage RecordingStorageConfig `mapstructure:"recordingStorage" json:"recordingStorage" validate:"required,dive"`
	// SegmentReaderWorkerCount number of worker threads in the video segment reader
	SegmentReaderWorkerCount int `mapstructure:"segmentReaderWorkerCount" json:"segmentReaderWorkerCount" validate:"gte=2,lte=64"`
}

// SegReceiverTrackingWindow convert SegReceiverTrackingWindowInSec to time.Duration
func (c CentralVODServerConfig) SegReceiverTrackingWindow() time.Duration {
	return time.Second * time.Duration(c.SegReceiverTrackingWindowInSec)
}

// ===============================================================================
// Complete Configuration Structures

// ControlNodeConfig define control node settings and behavior
type ControlNodeConfig struct {
	// Metrics metrics framework configuration
	Metrics MetricsConfig `mapstructure:"metrics" json:"metrics" validate:"required,dive"`
	// Postgres postgres DB configuration
	Postgres PostgresConfig `mapstructure:"postgres" json:"postgres" validate:"required,dive"`
	// Management management
	Management SystemManagementConfig `mapstructure:"management" json:"management" validate:"required,dive"`
	// VODConfig control node VOD server config
	VODConfig CentralVODServerConfig `mapstructure:"vod" json:"vod" validate:"required,dive"`
	// BroadcastSystem system broadcast channel configuration
	BroadcastSystem BroadcastSystemConfig `mapstructure:"broadcast" json:"broadcast" validate:"required,dive"`
}

// EdgeNodeConfig define edge node settings and behavior
type EdgeNodeConfig struct {
	// Metrics metrics framework configuration
	Metrics MetricsConfig `mapstructure:"metrics" json:"metrics" validate:"required,dive"`
	// VideoSource the video source this edge is monitoring
	VideoSource VideoSourceConfig `mapstructure:"videoSource" json:"videoSource" validate:"required,dive"`
	// SegmentCache video segment cache config
	SegmentCache RAMSegmentCacheConfig `mapstructure:"cache" json:"cache" validate:"required,dive"`
	// Sqlite sqlite DB configuration
	Sqlite SqliteConfig `mapstructure:"sqlite" json:"sqlite" validate:"required,dive"`
	// MonitorConfig HLS video source monitoring config
	MonitorConfig HLSMonitorConfig `mapstructure:"monitor" json:"monitor" validate:"required,dive"`
	// Forwarder video segment forwarding config
	Forwarder VideoForwarderConfig `mapstructure:"forwarder" json:"forwarder" validate:"required,dive"`
	// VODConfig HLS VOD server config
	VODConfig VODServerConfig `mapstructure:"vod" json:"vod" validate:"required,dive"`
	// APIServer local REST API config
	APIServer APIServerConfig `mapstructure:"api" json:"api" validate:"required,dive"`
	// RRClient PubSub request-response client config
	RRClient EdgeReqRespClientConfig `mapstructure:"requestResponse" json:"requestResponse" validate:"required,dive"`
	// BroadcastSystem system broadcast channel configuration
	BroadcastSystem BroadcastSystemConfig `mapstructure:"broadcast" json:"broadcast" validate:"required,dive"`
}

// ===============================================================================
// Default Configuration Setter

// InstallDefaultControlNodeConfigValues installs default config parameters in viper for
// control node
func InstallDefaultControlNodeConfigValues() {
	// Default metrics config
	viper.SetDefault("metrics.metricsEndpoint", "/metrics")
	viper.SetDefault("metrics.maxRequests", 4)
	// Default metrics features config
	viper.SetDefault("metrics.features.enableAppMetrics", false)
	viper.SetDefault("metrics.features.enableHTTPMetrics", true)
	viper.SetDefault("metrics.features.enablePubSubMetrics", true)
	// Default metrics HTTP server config
	viper.SetDefault("metrics.service.listenOn", "0.0.0.0")
	viper.SetDefault("metrics.service.appPort", 3001)
	viper.SetDefault("metrics.service.timeoutSecs.read", 60)
	viper.SetDefault("metrics.service.timeoutSecs.write", 60)
	viper.SetDefault("metrics.service.timeoutSecs.idle", 60)

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
	viper.SetDefault("management.api.apis.requestLogging.logLevel", "warn")
	viper.SetDefault("management.api.apis.requestLogging.healthLogLevel", "debug")
	viper.SetDefault("management.api.apis.requestLogging.requestIDHeader", "X-Request-ID")
	viper.SetDefault("management.api.apis.requestLogging.skipHeaders", []string{
		"WWW-Authenticate", "Authorization", "Proxy-Authenticate", "Proxy-Authorization",
	})
	// Default system manager request-response client config
	viper.SetDefault("management.requestResponse.self.msgTTL", 600)
	viper.SetDefault("management.requestResponse.supportWorkerCount", 4)
	viper.SetDefault("management.requestResponse.outboundRequestDurationInSec", 15)
	viper.SetDefault("management.requestResponse.requestTimeoutEnforceIntInSec", 30)
	// Default video source management config
	viper.SetDefault("management.videoSourceManagement.statusReportMaxDelayInSec", 60)
	// Default recording management config
	viper.SetDefault("management.recordingManagement.segmentCleanupIntInSec", 60)

	// Default VOD server config
	// Default REST API server config
	viper.SetDefault("vod.api.enabled", true)
	viper.SetDefault("vod.api.service.listenOn", "0.0.0.0")
	viper.SetDefault("vod.api.service.appPort", 8081)
	viper.SetDefault("vod.api.service.timeoutSecs.read", 60)
	viper.SetDefault("vod.api.service.timeoutSecs.write", 60)
	viper.SetDefault("vod.api.service.timeoutSecs.idle", 60)
	viper.SetDefault("vod.api.apis.endPoint.pathPrefix", "/")
	viper.SetDefault("vod.api.apis.requestLogging.logLevel", "debug")
	viper.SetDefault("vod.api.apis.requestLogging.healthLogLevel", "debug")
	viper.SetDefault("vod.api.apis.requestLogging.requestIDHeader", "X-Request-ID")
	viper.SetDefault("vod.api.apis.requestLogging.skipHeaders", []string{
		"WWW-Authenticate", "Authorization", "Proxy-Authenticate", "Proxy-Authorization",
	})
	// Default number of segments to include a live VOD playlist
	viper.SetDefault("vod.liveVODSegmentCount", 2)
	// Default TTL for storing segment fetched from cold storage in local cache
	viper.SetDefault("vod.segmentCacheTTLInSec", 300)
	// Default segment receiver tracking window
	viper.SetDefault("vod.segmentReceiverTrackingWindow", 60)
	// Default segment reader worker threads
	viper.SetDefault("vod.segmentReaderWorkerCount", 4)

	// Default broadcast channel config
	viper.SetDefault("broadcast.pubsub.msgTTL", 600)
}

// InstallDefaultEdgeNodeConfigValues installs default config parameters in viper for edge node
func InstallDefaultEdgeNodeConfigValues() {
	// Default metrics config
	viper.SetDefault("metrics.metricsEndpoint", "/metrics")
	viper.SetDefault("metrics.maxRequests", 4)
	// Default metrics features config
	viper.SetDefault("metrics.features.enableAppMetrics", false)
	viper.SetDefault("metrics.features.enableHTTPMetrics", true)
	viper.SetDefault("metrics.features.enablePubSubMetrics", true)
	// Default metrics HTTP server config
	viper.SetDefault("metrics.service.listenOn", "0.0.0.0")
	viper.SetDefault("metrics.service.appPort", 3001)
	viper.SetDefault("metrics.service.timeoutSecs.read", 60)
	viper.SetDefault("metrics.service.timeoutSecs.write", 60)
	viper.SetDefault("metrics.service.timeoutSecs.idle", 60)

	// Default video source config
	viper.SetDefault("videoSource.statusReportIntInSec", 60)

	// Default cache config
	viper.SetDefault("cache.retentionCheckIntInSec", 60)

	// Default sqlite config
	viper.SetDefault("sqlite.db", fmt.Sprintf("/tmp/livemix-edge-%s.db", ulid.Make().String()))

	// Default video source monitor config
	viper.SetDefault("monitor.segmentReaderWorkerCount", 4)

	// Default forwarder config
	// Default live video segment forwarder config
	viper.SetDefault("forwarder.live.maxInFlightSegments", 4)
	// Default recording video segment forwarder config
	viper.SetDefault("forwarder.recording.maxInFlightSegments", 4)
	// Default live segment forwarder HTTP config
	viper.SetDefault("forwarder.live.remote.requestIDHeader", "X-Request-ID")
	viper.SetDefault("forwarder.live.remote.client.retry.maxAttempts", 5)
	viper.SetDefault("forwarder.live.remote.client.retry.initialWaitTimeInSec", 2)
	viper.SetDefault("forwarder.live.remote.client.retry.maxWaitTimeInSec", 30)

	// Default VOD server config
	// Default number of segments to include a live VOD playlist
	viper.SetDefault("vod.liveVODSegmentCount", 2)
	// Default TTL for storing segment fetched from cold storage in local cache
	viper.SetDefault("vod.segmentCacheTTLInSec", 300)

	// Default query API server config
	viper.SetDefault("api.enabled", true)
	viper.SetDefault("api.service.listenOn", "0.0.0.0")
	viper.SetDefault("api.service.appPort", 8080)
	viper.SetDefault("api.service.timeoutSecs.read", 60)
	viper.SetDefault("api.service.timeoutSecs.write", 60)
	viper.SetDefault("api.service.timeoutSecs.idle", 60)
	viper.SetDefault("api.apis.endPoint.pathPrefix", "/")
	viper.SetDefault("api.apis.requestLogging.logLevel", "warn")
	viper.SetDefault("api.apis.requestLogging.healthLogLevel", "debug")
	viper.SetDefault("api.apis.requestLogging.requestIDHeader", "X-Request-ID")
	viper.SetDefault("api.apis.requestLogging.skipHeaders", []string{
		"WWW-Authenticate", "Authorization", "Proxy-Authenticate", "Proxy-Authorization",
	})

	// Default edge node request-response client config
	viper.SetDefault("requestResponse.self.msgTTL", 600)
	viper.SetDefault("requestResponse.supportWorkerCount", 4)
	viper.SetDefault("requestResponse.requestTimeoutEnforceIntInSec", 30)
	viper.SetDefault("requestResponse.systemControlTopic", "system-control-node")
	viper.SetDefault("requestResponse.outboundRequestDurationInSec", 15)

	// Default broadcast channel config
	viper.SetDefault("broadcast.pubsub.msgTTL", 600)
}
