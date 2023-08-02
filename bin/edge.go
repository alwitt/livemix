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
	nodeRuntimeCtxt       context.Context
	ctxtCancel            context.CancelFunc
	psClient              goutils.PubSubClient
	rrClient              goutils.RequestResponseClient
	segmentReader         utils.SegmentReader
	monitor               tracker.SourceHLSMonitor
	operator              edge.VideoSourceOperator
	playlistManager       vod.PlaylistManager
	PlaylistReceiveServer *http.Server
	VODServer             *http.Server
	MetricsServer         *http.Server
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
	sqlDSN := db.GetSqliteDialector(config.Sqlite.DBFile)
	dbConns, err := db.NewSQLConnection(sqlDSN, logger.Error)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define SQL connection manager")
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

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initializing RPC clients")

	// Define PubSub and request-response clients
	theNode.psClient, theNode.rrClient, err = buildReqRespClients(
		parentCtxt, nodeName, config.RRClient.ReqRespClientConfig, pubsubMetricsAgent,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("PubSub request-response client initialization failed")
		return theNode, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized RPC clients")
	initStep++

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initializing RPC driver")

	// Define edge-to-controller request-response client
	edgeToCtrlRRClient, err := edge.NewControlRequestClient(
		parentCtxt,
		nodeName,
		config.RRClient.ControlRRTopic,
		theNode.rrClient,
		config.RRClient.MaxOutboundRequestDuration(),
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create edge-to-controller request-response client")
		return theNode, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized RPC driver")
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
	sourceInfo, err := edgeToCtrlRRClient.GetVideoSourceInfo(parentCtxt, config.VideoSource.Name)
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
	httpClient, err := utils.DefineHTTPClient(
		theNode.nodeRuntimeCtxt, config.Forwarder.Live.Remote.Client,
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
		httpClient,
		segmentForwarderMetrics,
		forwarderLatencyMetrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create HTTP segment sender")
		return theNode, err
	}
	liveForwarder, err := forwarder.NewHTTPLiveStreamSegmentForwarder(
		parentCtxt, dbConns, httpSegmentSender, config.Forwarder.Live.MaxInFlight,
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
		SelfReqRespTargetID:        config.RRClient.InboudRequestTopic.Topic,
		DBConns:                    dbConns,
		VideoCache:                 cache,
		BroadcastClient:            psBroadcast,
		RecordingSegmentForwarder:  recordingForwarder,
		LiveStreamSegmentForwarder: liveForwarder,
		StatusReportInterval:       config.VideoSource.StatusReportInt(),
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initializing node operator")

	// Define video source operator
	theNode.operator, err = edge.NewManager(parentCtxt, edgeOperatorConfig, metrics)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create video source operator")
		return theNode, err
	}

	// Install reference to VideoSourceOperator
	edgeToCtrlRRClient.InstallReferenceToManager(theNode.operator)

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
		parentCtxt, config.MonitorConfig.SegmentReaderWorkerCount, nil, metrics,
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
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create HLS monitor")
		return theNode, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized video source monitor")
	initStep++

	// ====================================================================================
	// Prepare input - HTTP HLS playlist receiver

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing HLS playlist HTTP receiver")

	theNode.PlaylistReceiveServer, err = api.BuildPlaylistReceiverServer(
		theNode.nodeRuntimeCtxt,
		config.MonitorConfig.APIServer,
		theNode.monitor.Update,
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
		parentCtxt, dbConns, 2, plBuilder, segmentMgnt,
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

	// Define live VOD HTTP server
	theNode.VODServer, err = api.BuildVODServer(
		config.VODConfig.APIServer, dbConns, theNode.playlistManager, httpMetricsAgent,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create live VOD HTTP server")
		return theNode, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized HTTP VOD server")
	initStep++

	// ====================================================================================
	// Perform support tasks

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Populating local persistence of active recordings associated with this source")

	// Query system control for active recordings
	recordings, err := edgeToCtrlRRClient.ListActiveRecordingsOfSource(parentCtxt, sourceInfo.ID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to fetch list of active recordings from system control node")
		return theNode, err
	}
	for _, recording := range recordings {
		if err := theNode.operator.StartRecording(parentCtxt, recording); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("recording-id", recording.ID).
				Error("Failed to install ongoing recording")
		}
	}

	log.
		WithField("initialize", initStep).
		Info("Updated local persistence of active recordings associated with this source")

	return theNode, nil
}
