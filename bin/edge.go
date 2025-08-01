package bin

import (
	"context"
	"net/http"
	"net/url"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/edge"
	"github.com/alwitt/livemix/forwarder"
	"github.com/alwitt/livemix/tracker"
	"github.com/alwitt/livemix/utils"
	"github.com/alwitt/livemix/vod"
	"github.com/apex/log"
	"gorm.io/gorm/logger"
)

// EdgeNode edge node monitoring one video source
type EdgeNode struct {
	nodeRuntimeCtxt context.Context
	ctxtCancel      context.CancelFunc
	psClient        goutils.PubSubClient
	rrClient        goutils.RequestResponseClient
	segmentReader   utils.SegmentReader
	monitor         tracker.SourceHLSMonitor
	operator        edge.VideoSourceOperator
	playlistManager vod.PlaylistManager
	MetricsServer   *http.Server
	APIServer       *http.Server
}

/*
Cleanup stop and clean up the edge node

	@param ctxt context.Context - execution context
*/
func (n EdgeNode) Cleanup(ctxt context.Context) error {
	n.ctxtCancel()
	if err := n.monitor.Stop(ctxt); err != nil {
		return err
	}
	if err := n.segmentReader.Stop(ctxt); err != nil {
		return err
	}
	if err := n.playlistManager.Stop(ctxt); err != nil {
		return err
	}
	if err := n.rrClient.Stop(ctxt); err != nil {
		return err
	}
	if err := n.psClient.Close(ctxt); err != nil {
		return err
	}
	return n.operator.Stop(ctxt)
}

/*
DefineEdgeNode setup new edge node

	@param parentCtxt context.Context - parent execution context
	@param nodeName string - edge node name
	@param config common.EdgeNodeConfig - edge node configuration
	@returns new edge node
*/
func DefineEdgeNode(
	parentCtxt context.Context, nodeName string, config common.EdgeNodeConfig,
) (EdgeNode, error) {
	logTags := log.Fields{
		"module": "global", "component": "edge-node", "instance": nodeName,
	}

	theNode := EdgeNode{}
	theNode.nodeRuntimeCtxt, theNode.ctxtCancel = context.WithCancel(parentCtxt)

	initStep := 0

	// ====================================================================================
	// Prepare base layer - Metrics

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing metrics framework")

	metrics, err := NewMetricsCollector(config.Metrics.Features)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create metrics collector")
		return theNode, err
	}

	var httpMetricsAgent goutils.HTTPRequestMetricHelper
	if config.Metrics.Features.EnableHTTPMetrics {
		httpMetricsAgent = metrics.InstallHTTPMetrics()
	}

	var pubsubMetricsAgent goutils.PubSubMetricHelper
	if config.Metrics.Features.EnablePubSubMetrics {
		pubsubMetricsAgent = metrics.InstallPubSubMetrics()
	}

	var taskProcessorMetricsAgent goutils.TaskProcessorMetricHelper
	if config.Metrics.Features.EnableTaskProcessorMetrics {
		taskProcessorMetricsAgent = metrics.InstallTaskProcessorMetrics()
	}

	// Create server to host metrics collection endpoint
	theNode.MetricsServer, err = api.BuildMetricsCollectionServer(
		config.Metrics.Server, metrics, config.Metrics.MetricsEndpoint, config.Metrics.MaxRequests,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create metrics collection hosting server")
		return theNode, err
	}

	// Define the segment forwarding tracking metrics right now
	segmentForwarderMetrics, err := utils.NewSegmentMetricsAgent(
		parentCtxt,
		metrics,
		utils.MetricsNameForwarderSenderSegmentForwardLen,
		"Tracking total bytes forward by segment forwarder",
		utils.MetricsNameForwarderSenderSegmentForwardCount,
		"Tracking total segments forwarded by segment forwarder",
		[]string{"type", "source"},
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to define segment forwarding metrics helper agent")
		return theNode, err
	}
	forwarderLatencyMetrics, err := metrics.InstallCustomCounterVecMetrics(
		parentCtxt,
		utils.MetricsNameForwarderSenderSegmentForwardLatency,
		"Tracking segment forward latency by segment forwarder",
		[]string{"type"},
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to define segment forward latency metrics")
		return theNode, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized metrics framework")
	initStep++

	// ====================================================================================
	// Prepare base layer - Persistence

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initializing persistence layer")

	// Setup database connection manager
	sqlDSN := db.GetSqliteDialector(config.Sqlite.DBFile, config.Sqlite.BusyTimeoutMSec)
	dbConns, err := db.NewSQLConnection(sqlDSN, logger.Error, config.Sqlite.NoTransactions)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define SQL connection manager")
		return theNode, err
	}
	if err := dbConns.ApplySQLitePragmas(config.Sqlite); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to directly set SQLite runtime PRAGMAs")
		return theNode, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized persistence layer")
	initStep++

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initializing local video cache")

	// Define video segment cache
	cache, err := utils.NewLocalVideoSegmentCache(
		parentCtxt, config.SegmentCache.RetentionCheckInt(), metrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define video segment cache")
		return theNode, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized local video cache")
	initStep++

	// ====================================================================================
	// Prepare base layer - RPC clients

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing PubSub RPC clients")

	// Define PubSub and request-response clients
	theNode.psClient, theNode.rrClient, err = buildReqRespClients(
		parentCtxt, nodeName, config.ReqResp.PubSub.PubSubReqRespClientConfig, pubsubMetricsAgent,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("PubSub request-response client initialization failed")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized RPC clients")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing PubSub request-response client")

	// Define PubSub request-response client
	pubSubReqRespClient, err := edge.NewPubSubControlRequestClient(
		parentCtxt,
		nodeName,
		config.ReqResp.PubSub.ControlRRTopic,
		theNode.rrClient,
		config.ReqResp.PubSub.MaxOutboundRequestDuration(),
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create PubSub request-response client")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized PubSub request-response client")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing REST request-response client")

	// Define resty HTTP client for request-response
	var reqRespHTTPClientAuthConfig *goutils.HTTPClientAuthConfig
	if config.ReqResp.REST.Client.OAuth != nil {
		reqRespHTTPClientAuthConfig = &goutils.HTTPClientAuthConfig{
			IssuerURL:      config.ReqResp.REST.Client.OAuth.IssuerURL,
			ClientID:       config.ReqResp.REST.Client.OAuth.ClientID,
			ClientSecret:   config.ReqResp.REST.Client.OAuth.ClientSecret,
			TargetAudience: config.ReqResp.REST.Client.OAuth.TargetAudience,
			LogTags: log.Fields{
				"module": "go-utils", "component": "oauth-token-manager", "instance": "client-cred-flow",
			},
		}
	} else {
		reqRespHTTPClientAuthConfig = nil
	}

	reqRespHTTPClient, err := goutils.DefineHTTPClient(
		theNode.nodeRuntimeCtxt,
		goutils.HTTPClientRetryConfig{
			MaxAttempts:  config.ReqResp.REST.Client.Retry.MaxAttempts,
			InitWaitTime: config.ReqResp.REST.Client.Retry.InitWaitTime(),
			MaxWaitTime:  config.ReqResp.REST.Client.Retry.MaxWaitTime(),
		},
		reqRespHTTPClientAuthConfig,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define resty HTTP client")
		return theNode, err
	}

	// Parse the control edge API base URL
	ctrlEdgeAPIBase, err := url.Parse(config.ReqResp.REST.BaseURL)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to parse control edge API base URL")
		return theNode, err
	}

	// Define REST request-response client
	restReqRespClient, err := edge.NewRestControlRequestClient(
		parentCtxt,
		ctrlEdgeAPIBase,
		config.ReqResp.REST.RequestIDHeader,
		reqRespHTTPClient,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create REST request-response client")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized REST request-response client")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing message broadcast client")

	// Define PubSub message broadcaster client
	psBroadcast, err := utils.NewPubSubBroadcaster(
		theNode.psClient, config.BroadcastSystem.PubSub.Topic,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create PubSub message broadcast client")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized message broadcast client")
	initStep++

	// ====================================================================================
	// Fetch video source info from system control node

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Querying system control node regarding video source under management")

	// Query control for target video source info
	sourceInfo, err := restReqRespClient.GetVideoSourceInfo(parentCtxt, config.VideoSource.Name)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Errorf("Fetching info for video source '%s' failed", config.VideoSource.Name)
		return theNode, err
	}
	// Record this in persistence
	{
		dbClient := dbConns.NewPersistanceManager()
		if err := dbClient.RecordKnownVideoSource(
			parentCtxt,
			sourceInfo.ID,
			sourceInfo.Name,
			sourceInfo.TargetSegmentLength,
			sourceInfo.PlaylistURI,
			sourceInfo.Description,
			sourceInfo.Streaming,
		); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				Errorf("Recording video source '%s' failed", config.VideoSource.Name)
			return theNode, err
		}
		dbClient.Close()
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Received parameters regarding video source under management")
	initStep++

	// ====================================================================================
	// Prepare output - HTTP live stream forwarder

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing HTTP client for segment forwarding")

	// Define resty HTTP client for forwarder
	var forwarderHTTPClientAuthConfig *goutils.HTTPClientAuthConfig
	if config.Forwarder.Live.Remote.Client.OAuth != nil {
		forwarderHTTPClientAuthConfig = &goutils.HTTPClientAuthConfig{
			IssuerURL:      config.Forwarder.Live.Remote.Client.OAuth.IssuerURL,
			ClientID:       config.Forwarder.Live.Remote.Client.OAuth.ClientID,
			ClientSecret:   config.Forwarder.Live.Remote.Client.OAuth.ClientSecret,
			TargetAudience: config.Forwarder.Live.Remote.Client.OAuth.TargetAudience,
			LogTags: log.Fields{
				"module": "go-utils", "component": "oauth-token-manager", "instance": "client-cred-flow",
			},
		}
	} else {
		forwarderHTTPClientAuthConfig = nil
	}

	forwarderHTTPClient, err := goutils.DefineHTTPClient(
		theNode.nodeRuntimeCtxt,
		goutils.HTTPClientRetryConfig{
			MaxAttempts:  config.Forwarder.Live.Remote.Client.Retry.MaxAttempts,
			InitWaitTime: config.Forwarder.Live.Remote.Client.Retry.InitWaitTime(),
			MaxWaitTime:  config.Forwarder.Live.Remote.Client.Retry.MaxWaitTime(),
		},
		forwarderHTTPClientAuthConfig,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define resty HTTP client")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized HTTP client for segment forwarding")
	initStep++

	// Parse the forwarder target URL
	httpForwardTarget, err := url.Parse(config.Forwarder.Live.Remote.TargetURL)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to parse HTTP forward target URL")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing live stream segment forwarder")

	// Define live segment HTTP forwarder
	httpSegmentSender, err := forwarder.NewHTTPSegmentSender(
		parentCtxt,
		httpForwardTarget,
		config.Forwarder.Live.Remote.RequestIDHeader,
		forwarderHTTPClient,
		segmentForwarderMetrics,
		forwarderLatencyMetrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create HTTP segment sender")
		return theNode, err
	}
	liveForwarder, err := forwarder.NewHTTPLiveStreamSegmentForwarder(
		parentCtxt,
		dbConns,
		httpSegmentSender,
		config.Forwarder.Live.MaxInFlight,
		taskProcessorMetricsAgent,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create live stream segment forwarder")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized live stream segment forwarder")
	initStep++

	// ====================================================================================
	// Prepare output - S3 recording forwarder

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing S3 client for segment forwarding")

	// Define S3 client
	s3Client, err := utils.NewS3Client(config.Forwarder.Recording.RecordingStorage.S3)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create S3 client")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized S3 client for segment forwarding")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing recording session segment forwarder")

	// Define recordings segment forwarder
	s3SegmentSender, err := forwarder.NewS3SegmentSender(
		parentCtxt, s3Client, dbConns, segmentForwarderMetrics, forwarderLatencyMetrics,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create S3 segment sender")
		return theNode, err
	}
	recordingForwarder, err := forwarder.NewS3RecordingSegmentForwarder(
		parentCtxt,
		config.Forwarder.Recording.RecordingStorage,
		s3SegmentSender,
		psBroadcast,
		config.Forwarder.Recording.MaxInFlight,
		taskProcessorMetricsAgent,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create recording segment forwarder")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized recording session segment forwarder")
	initStep++

	// ====================================================================================
	// Prepare control unit

	edgeOperatorConfig := edge.VideoSourceOperatorConfig{
		Self:                       sourceInfo,
		SelfReqRespTargetID:        config.ReqResp.PubSub.InboudRequestTopic.Topic,
		DBConns:                    dbConns,
		VideoCache:                 cache,
		RecordingSegmentForwarder:  recordingForwarder,
		LiveStreamSegmentForwarder: liveForwarder,
		StatusReportInterval:       config.VideoSource.StatusReportInt(),
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initializing node operator")

	// Define video source operator
	theNode.operator, err = edge.NewManager(
		parentCtxt, edgeOperatorConfig, restReqRespClient, metrics, taskProcessorMetricsAgent,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create video source operator")
		return theNode, err
	}

	// Install reference to VideoSourceOperator
	pubSubReqRespClient.InstallReferenceToManager(theNode.operator)

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized node operator")
	initStep++

	// ====================================================================================
	// Prepare input - HLS source playlist monitor

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing local file system segment reader")

	// Define video segment reader
	theNode.segmentReader, err = utils.NewSegmentReader(
		parentCtxt,
		config.MonitorConfig.SegmentReaderWorkerCount,
		0,
		nil,
		metrics,
		taskProcessorMetricsAgent,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create video segment reader")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized local file system segment reader")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing video source monitor")

	// Define video monitor
	theNode.monitor, err = tracker.NewSourceHLSMonitor(
		parentCtxt,
		sourceInfo,
		dbConns,
		config.MonitorConfig.TrackingWindow(),
		cache,
		theNode.segmentReader,
		theNode.operator.NewSegmentFromSource,
		metrics,
		taskProcessorMetricsAgent,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create HLS monitor")
		return theNode, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized video source monitor")
	initStep++

	// ====================================================================================
	// Prepare output - Local HTTP VOD server

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing HTTP VOD server support components")

	// Define live VOD playlist builder
	plBuilder, err := vod.NewPlaylistBuilder(
		parentCtxt, dbConns, config.VODConfig.LiveVODSegmentCount, metrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create VOD playlist builder")
		return theNode, err
	}

	// Define segment manager
	segmentMgnt, err := vod.NewSegmentManager(
		parentCtxt, cache, theNode.segmentReader, config.VODConfig.SegmentCacheTTL(), metrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create video segment manager")
		return theNode, err
	}

	theNode.playlistManager, err = vod.NewPlaylistManager(
		parentCtxt, dbConns, 2, plBuilder, segmentMgnt, taskProcessorMetricsAgent,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create video playlist manager")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized HTTP VOD server support components")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing HTTP VOD server")

	// ====================================================================================
	// Prepare REST API server

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing HLS playlist HTTP receiver")

	theNode.APIServer, err = api.BuildEdgeAPIServer(
		theNode.nodeRuntimeCtxt,
		config.VideoSource,
		config.MonitorConfig.DefaultSegmentURIPrefix,
		dbConns,
		theNode.operator,
		config.APIServer,
		theNode.monitor.Update,
		theNode.playlistManager,
		httpMetricsAgent,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create playlist receiver HTTP server")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized HLS playlist HTTP receiver")

	return theNode, nil
}
