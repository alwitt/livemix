package bin

import (
	"context"
	"net/http"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/edge"
	"github.com/alwitt/livemix/tracker"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
	"gorm.io/gorm/logger"
)

// EdgeNode edge node monitoring one video source
type EdgeNode struct {
	psClient              goutils.PubSubClient
	rrClient              goutils.RequestResponseClient
	segmentReader         utils.SegmentReader
	monitor               tracker.SourceHLSMonitor
	PlaylistReceiveServer *http.Server
	VODServer             *http.Server
}

/*
Cleanup stop and clean up the edge node

	@param ctxt context.Context - execution context
*/
func (n EdgeNode) Cleanup(ctxt context.Context) error {
	if err := n.rrClient.Stop(ctxt); err != nil {
		return err
	}
	if err := n.psClient.Close(ctxt); err != nil {
		return err
	}
	if err := n.segmentReader.Stop(ctxt); err != nil {
		return err
	}
	return n.monitor.Stop(ctxt)
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
		* Prepare HLS video source monitor
		* (i.e. forwarder, etc.)
		* Prepare video playlist receive server
		* Prepare local VOD server
	*/

	theNode := EdgeNode{}

	sqlDSN := db.GetSqliteDialector(config.Sqlite.DBFile)

	// Define the persistence manager
	dbManager, err := db.NewManager(sqlDSN, logger.Error)
	if err != nil {
		log.WithError(err).Error("Failed to define persistence manager")
		return theNode, err
	}

	// Define video segment cache
	cache, err := utils.NewLocalVideoSegmentCache(
		parentCtxt, time.Second*time.Duration(config.SegmentCache.RetentionCheckIntInSec),
	)
	if err != nil {
		log.WithError(err).Error("Failed to define video segment cache")
		return theNode, err
	}

	// Prepare core request-response client
	theNode.psClient, theNode.rrClient, err = buildReqRespClients(
		parentCtxt, nodeName, config.RRClient.ReqRespClientConfig,
	)
	if err != nil {
		log.WithError(err).Error("PubSub request-response client initialization failed")
		return theNode, err
	}

	// Define edge-to-controller request-response client
	edgeToCtrlRRClient, err := edge.NewControlRequestClient(
		nodeName,
		config.RRClient.ControlRRTopic,
		theNode.rrClient,
		time.Second*time.Duration(config.RRClient.MaxOutboundRequestDurationInSec),
	)
	if err != nil {
		log.WithError(err).Error("Failed to create edge-to-controller request-response client")
		return theNode, err
	}

	// Query control for target video source info
	sourceInfo, err := edgeToCtrlRRClient.GetVideoSourceInfo(parentCtxt, config.VideoSourceName)
	if err != nil {
		log.WithError(err).Errorf("Fetching info for video source '%s' failed", config.VideoSourceName)
		return theNode, err
	}
	// Record this in persistence
	if err := dbManager.RecordKnownVideoSource(
		parentCtxt, sourceInfo.ID, sourceInfo.Name, sourceInfo.PlaylistURI, sourceInfo.Description,
	); err != nil {
		log.WithError(err).Errorf("Recording video source '%s' failed", config.VideoSourceName)
		return theNode, err
	}

	// Define video segment reader
	theNode.segmentReader, err = utils.NewSegmentReader(
		parentCtxt, config.MonitorConfig.SegmentReaderWorkerCount,
	)
	if err != nil {
		log.WithError(err).Error("Failed to create video segment reader")
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
		func(ctxt context.Context, segment common.VideoSegmentWithData) error {
			// TODO FIXME: once forwarders are implemented replace this
			log.WithField("segment", segment.String()).Debug("Processed new segment")
			return nil
		},
	)
	if err != nil {
		log.WithError(err).Error("Failed to create HLS monitor")
		return theNode, err
	}

	// Define playlist receiver HTTP server
	theNode.PlaylistReceiveServer, err = api.BuildPlaylistReceiverServer(
		config.MonitorConfig.APIServer, theNode.monitor.Update,
	)
	if err != nil {
		log.WithError(err).Error("Failed to create playlist receiver HTTP server")
		return theNode, err
	}

	return theNode, nil
}
