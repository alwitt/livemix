package bin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
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
	liveForwarder         forwarder.LiveStreamSegmentForwarder
	operator              edge.VideoSourceOperator
	PlaylistReceiveServer *http.Server
	VODServer             *http.Server
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
	if err := n.liveForwarder.Stop(ctxt); err != nil {
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
	/*
		Steps for preparing the edge are

		* Prepare persistence sqlite DB
		* Prepare local video segment cache
		* Prepare request-response client for edge node
		* Query control for video source info
			* Load video source info into persistence
		* Prepare video segment reader
		* Prepare forwarder
			* Prepare live segment HTTP forwarder
				* Prepare HTTP client
				* Prepare the forwarder
		* Prepare HLS video source monitor
		* Prepare video playlist receive server
		* Prepare segment manager
		* Prepare VOD playlist builder
		* Prepare local VOD server
	*/

	logTags := log.Fields{
		"module": "global", "component": "edge-node", "instance": nodeName,
	}

	theNode := EdgeNode{}
	theNode.nodeRuntimeCtxt, theNode.ctxtCancel = context.WithCancel(parentCtxt)

	sqlDSN := db.GetSqliteDialector(config.Sqlite.DBFile)

	// Define the persistence manager
	dbManager, err := db.NewManager(sqlDSN, logger.Error)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define persistence manager")
		return theNode, err
	}

	// Define video segment cache
	cache, err := utils.NewLocalVideoSegmentCache(
		parentCtxt, time.Second*time.Duration(config.SegmentCache.RetentionCheckIntInSec),
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define video segment cache")
		return theNode, err
	}

	// Prepare core request-response client
	theNode.psClient, theNode.rrClient, err = buildReqRespClients(
		parentCtxt, nodeName, config.RRClient.ReqRespClientConfig,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("PubSub request-response client initialization failed")
		return theNode, err
	}

	// Define edge-to-controller request-response client
	edgeToCtrlRRClient, err := edge.NewControlRequestClient(
		parentCtxt,
		nodeName,
		config.RRClient.ControlRRTopic,
		theNode.rrClient,
		time.Second*time.Duration(config.RRClient.MaxOutboundRequestDurationInSec),
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create edge-to-controller request-response client")
		return theNode, err
	}

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
	if err := dbManager.RecordKnownVideoSource(
		parentCtxt,
		sourceInfo.ID,
		sourceInfo.Name,
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

	forwardStatusReports := func(
		ctxt context.Context, report ipc.VideoSourceStatusReport,
	) error {
		payload, err := json.Marshal(&report)
		if err != nil {
			return err
		}
		_, err = theNode.psClient.Publish(ctxt, config.BroadcastSystem.PubSub.Topic, payload, nil, true)
		return err
	}

	// Define video source operator
	edgeOperator, err := edge.NewManager(
		parentCtxt,
		sourceInfo,
		config.RRClient.InboudRequestTopic.Topic,
		dbManager,
		forwardStatusReports,
		time.Second*time.Duration(config.VideoSource.StatusReportIncInSec),
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create video source operator")
		return theNode, err
	}
	theNode.operator = edgeOperator

	// Install reference to VideoSourceOperator
	edgeToCtrlRRClient.InstallReferenceToManager(edgeOperator)

	// Define video segment reader
	theNode.segmentReader, err = utils.NewSegmentReader(
		parentCtxt, config.MonitorConfig.SegmentReaderWorkerCount,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create video segment reader")
		return theNode, err
	}

	// Define resty HTTP client for forwarder
	httpClient, err := utils.DefineHTTPClient(config.Forwarder.Live.Remote.Client)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define resty HTTP client")
		return theNode, err
	}

	httpForwardTarget, err := url.Parse(config.Forwarder.Live.Remote.TargetURL)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to parse HTTP forward target URL")
		return theNode, err
	}

	// Define live segment HTTP forwarder
	httpSegmentSender, err := forwarder.NewHTTPSegmentSender(httpForwardTarget, httpClient)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create HTTP segment sender")
		return theNode, err
	}
	theNode.liveForwarder, err = forwarder.NewHTTPLiveStreamSegmentForwarder(
		parentCtxt, dbManager, httpSegmentSender, config.Forwarder.Live.MaxInFlight,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create live stream segment forwarder")
		return theNode, err
	}

	// Define video monitor
	theNode.monitor, err = tracker.NewSourceHLSMonitor(
		parentCtxt,
		sourceInfo,
		dbManager,
		time.Second*time.Duration(config.MonitorConfig.TrackingWindowInSec),
		cache,
		theNode.segmentReader,
		theNode.liveForwarder.ForwardSegment,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create HLS monitor")
		return theNode, err
	}

	// Define playlist receiver HTTP server
	theNode.PlaylistReceiveServer, err = api.BuildPlaylistReceiverServer(
		theNode.nodeRuntimeCtxt, config.MonitorConfig.APIServer, theNode.monitor.Update,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create playlist receiver HTTP server")
		return theNode, err
	}

	// Define segment manager
	segmentMgnt, err := vod.NewSegmentManager(
		cache, theNode.segmentReader, time.Second*time.Duration(config.VODConfig.SegmentCacheTTLInSec),
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create video segment manager")
		return theNode, err
	}

	// Define live VOD playlist builder
	plBuilder, err := vod.NewPlaylistBuilder(
		dbManager,
		time.Second*time.Duration(config.VideoSource.SegmentDurationInSec),
		config.VODConfig.LiveVODSegmentCount,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create VOD playlist builder")
		return theNode, err
	}

	// Define live VOD HTTP server
	theNode.VODServer, err = api.BuildVODServer(
		config.VODConfig.APIServer, dbManager, plBuilder, segmentMgnt,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create live VOD HTTP server")
		return theNode, err
	}

	return theNode, nil
}
