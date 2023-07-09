package common

import (
	"fmt"
	"time"

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
	// SegmentDurationInSec the expected duration of each video segment in secs
	SegmentDurationInSec uint32 `mapstructure:"segmentDurationInSec" json:"segmentDurationInSec" validate:"gte=2"`
	// StatusReportIncInSec interval in secs between video source status report to system controller
	StatusReportIncInSec uint32 `mapstructure:"statusReportIntInSec" json:"statusReportIntInSec" validate:"gte=10"`
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

// MemcachedSegementCacheConfig memcached video segment cache config
type MemcachedSegementCacheConfig struct {
	// Servers list of memcached servers to establish connection with
	Servers []string `mapstructure:"servers" json:"servers" validate:"required,gte=1"`
}

// ===============================================================================
// Major System Component Configuration Structures

// VideoSourceManagementConfig video source management settings
type VideoSourceManagementConfig struct {
	// StatusReportMaxDelayInSec maximum delay a source status report can be before the
	// control node marks source as no longer connected
	StatusReportMaxDelayInSec uint32 `mapstructure:"statusReportMaxDelayInSec" json:"statusReportMaxDelayInSec" validate:"required,gte=20"`
}

// SystemManagementConfig define control node management sub-module config
type SystemManagementConfig struct {
	// APIServer management REST API server config
	APIServer APIServerConfig `mapstructure:"api" json:"api" validate:"required,dive"`
	// RRClient PubSub request-response client config
	RRClient ReqRespClientConfig `mapstructure:"requestResponse" json:"requestResponse" validate:"required,dive"`
	// SourceManagment video source management settings
	SourceManagment VideoSourceManagementConfig `mapstructure:"videoSourceManagement" json:"videoSourceManagement" validate:"required,dive"`
}

// HLSMonitorConfig HLS video source monitoring config
type HLSMonitorConfig struct {
	// APIServer HLS playlist receiver REST API server config
	APIServer APIServerConfig `mapstructure:"api" json:"api" validate:"required,dive"`
	// TrackingWindowInSec tracking window is the duration in time a video segment is tracked.
	// After observing a new segment, that segment is remembered for the duration of a
	// tracking window, and forgotten after that.
	TrackingWindowInSec uint32 `mapstructure:"trackingWindow" json:"trackingWindow" validate:"gte=10,lte=300"`
	// SegmentReaderWorkerCount number of worker threads in the video segment reader
	SegmentReaderWorkerCount int `mapstructure:"segmentReaderWorkerCount" json:"segmentReaderWorkerCount" validate:"gte=2,lte=64"`
}

// VODServerConfig HLS VOD server config
type VODServerConfig struct {
	// APIServer HLS VOD REST API server config
	APIServer APIServerConfig `mapstructure:"api" json:"api" validate:"required,dive"`
	// LiveVODSegmentCount number of video segments to include when building a live
	// VOD playlist.
	LiveVODSegmentCount int `mapstructure:"liveVODSegmentCount" json:"liveVODSegmentCount" validate:"gte=1"`
	// SegmentCacheTTLInSec when a segment is read from cold storage, store it in
	// local cache with this TTL in secs.
	SegmentCacheTTLInSec uint32 `mapstructure:"segmentCacheTTLInSec" json:"segmentCacheTTLInSec" validate:"gte=30,lte=7200"`
}

// CentralVODServerConfig VOD server running on the control node
type CentralVODServerConfig struct {
	VODServerConfig `mapstructure:",squash"`
	// Cache memcached based video segment cache
	Cache MemcachedSegementCacheConfig `mapstructure:"cache" json:"cache" validate:"required,dive"`
	// SegReceiverTrackingWindowInSec Tracking window is the duration in time a video segment is
	// tracked. Recorded segments are forgotten after this tracking window.
	SegReceiverTrackingWindowInSec uint32 `mapstructure:"segmentReceiverTrackingWindow" json:"segmentReceiverTrackingWindow" validate:"gte=10,lte=300"`
}

// ===============================================================================
// Complete Configuration Structures

// ControlNodeConfig define control node settings and behavior
type ControlNodeConfig struct {
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
	// VideoSource the video source this edge is monitoring
	VideoSource VideoSourceConfig `mapstructure:"videoSource" json:"videoSource" validate:"required,dive"`
	// SegmentCache video segment cache config
	SegmentCache RAMSegmentCacheConfig `mapstructure:"cache" json:"cache" validate:"required,dive"`
	// Sqlite sqlite DB configuration
	Sqlite SqliteConfig `mapstructure:"sqlite" json:"sqlite" validate:"required,dive"`
	// MonitorConfig HLS video source monitoring config
	MonitorConfig HLSMonitorConfig `mapstructure:"monitor" json:"monitor" validate:"required,dive"`
	// VODConfig HLS VOD server config
	VODConfig VODServerConfig `mapstructure:"vod" json:"vod" validate:"required,dive"`
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
	viper.SetDefault("management.requestResponse.outboundRequestDurationInSec", 15)
	viper.SetDefault("management.requestResponse.requestTimeoutEnforceIntInSec", 30)
	// Default video source management config
	viper.SetDefault("management.videoSourceManagement.statusReportMaxDelayInSec", 60)

	// Default VOD server config
	// Default REST API server config
	viper.SetDefault("vod.api.enabled", true)
	viper.SetDefault("vod.api.service.listenOn", "0.0.0.0")
	viper.SetDefault("vod.api.service.appPort", 8081)
	viper.SetDefault("vod.api.service.timeoutSecs.read", 60)
	viper.SetDefault("vod.api.service.timeoutSecs.write", 60)
	viper.SetDefault("vod.api.service.timeoutSecs.idle", 60)
	viper.SetDefault("vod.api.apis.endPoint.pathPrefix", "/")
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

	// Default broadcast channel config
	viper.SetDefault("broadcast.pubsub.msgTTL", 600)
}

// InstallDefaultEdgeNodeConfigValues installs default config parameters in viper for edge node
func InstallDefaultEdgeNodeConfigValues() {
	// Default video source config
	viper.SetDefault("videoSource.statusReportIntInSec", 60)

	// Default cache config
	viper.SetDefault("cache.retentionCheckIntInSec", 60)

	// Default sqlite config
	viper.SetDefault("sqlite.db", fmt.Sprintf("/tmp/livemix-edge-%d.db", time.Now().Unix()))

	// Default video source monitor config
	viper.SetDefault("monitor.segmentReaderWorkerCount", 4)
	// Default playlist receiver REST API server config
	viper.SetDefault("monitor.api.enabled", true)
	viper.SetDefault("monitor.api.service.listenOn", "0.0.0.0")
	viper.SetDefault("monitor.api.service.appPort", 8080)
	viper.SetDefault("monitor.api.service.timeoutSecs.read", 60)
	viper.SetDefault("monitor.api.service.timeoutSecs.write", 60)
	viper.SetDefault("monitor.api.service.timeoutSecs.idle", 60)
	viper.SetDefault("monitor.api.apis.endPoint.pathPrefix", "/")
	viper.SetDefault("monitor.api.apis.requestLogging.requestIDHeader", "X-Request-ID")
	viper.SetDefault("monitor.api.apis.requestLogging.skipHeaders", []string{
		"WWW-Authenticate", "Authorization", "Proxy-Authenticate", "Proxy-Authorization",
	})

	// Default VOD server config
	// Default REST API server config
	viper.SetDefault("vod.api.enabled", true)
	viper.SetDefault("vod.api.service.listenOn", "0.0.0.0")
	viper.SetDefault("vod.api.service.appPort", 8081)
	viper.SetDefault("vod.api.service.timeoutSecs.read", 60)
	viper.SetDefault("vod.api.service.timeoutSecs.write", 60)
	viper.SetDefault("vod.api.service.timeoutSecs.idle", 60)
	viper.SetDefault("vod.api.apis.endPoint.pathPrefix", "/")
	viper.SetDefault("vod.api.apis.requestLogging.requestIDHeader", "X-Request-ID")
	viper.SetDefault("vod.api.apis.requestLogging.skipHeaders", []string{
		"WWW-Authenticate", "Authorization", "Proxy-Authenticate", "Proxy-Authorization",
	})
	// Default number of segments to include a live VOD playlist
	viper.SetDefault("vod.liveVODSegmentCount", 2)
	// Default TTL for storing segment fetched from cold storage in local cache
	viper.SetDefault("vod.segmentCacheTTLInSec", 300)

	// Default edge node request-response client config
	viper.SetDefault("requestResponse.self.msgTTL", 600)
	viper.SetDefault("requestResponse.supportWorkerCount", 4)
	viper.SetDefault("requestResponse.requestTimeoutEnforceIntInSec", 30)
	viper.SetDefault("requestResponse.systemControlTopic", "system-control-node")
	viper.SetDefault("requestResponse.outboundRequestDurationInSec", 15)

	// Default broadcast channel config
	viper.SetDefault("broadcast.pubsub.msgTTL", 600)
}
