package bin

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/control"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/utils"
	"github.com/alwitt/livemix/vod"
	"github.com/apex/log"
	"gorm.io/gorm/logger"
)

// ControlNode system control node
type ControlNode struct {
	psClient              goutils.PubSubClient
	rrClient              goutils.RequestResponseClient
	wg                    sync.WaitGroup
	workerCtxt            context.Context
	workerCtxtCancel      context.CancelFunc
	liveStreamSegmentMgmt control.LiveStreamSegmentManager
	coreManager           control.SystemManager
	playlistManager       vod.PlaylistManager
	MgmtAPIServer         *http.Server
	VODAPIServer          *http.Server
	MetricsServer         *http.Server
}

/*
Cleanup stop and clean up the system control node

	@param ctxt context.Context - execution context
*/
func (n *ControlNode) Cleanup(ctxt context.Context) error {
	n.workerCtxtCancel()
	if err := n.rrClient.Stop(ctxt); err != nil {
		return err
	}
	if err := n.psClient.Close(ctxt); err != nil {
		return err
	}
	if err := n.liveStreamSegmentMgmt.Stop(ctxt); err != nil {
		return nil
	}
	if err := n.playlistManager.Stop(ctxt); err != nil {
		return nil
	}
	if err := n.coreManager.Stop(ctxt); err != nil {
		return err
	}
	return goutils.TimeBoundedWaitGroupWait(ctxt, &n.wg, time.Second*5)
}

/*
DefineControlNode setup new system control node

	@param parentCtxt context.Context - parent execution context
	@param nodeName string - system control node name
	@param config common.ControlNodeConfig - system control node configuration
	@param psqlPassword string - Postgres SQL user password
	@returns new system control node
*/
func DefineControlNode(
	parentCtxt context.Context,
	nodeName string,
	config common.ControlNodeConfig,
	psqlPassword string,
) (*ControlNode, error) {
	/*
		Steps for preparing the control are

		* Prepare database
		* Prepare system manager
		* Prepare VOD support systems
		  * Prepare memcached segment cache
			* Prepare segment manager
		* Prepare request-response client for manager
		* Prepare HTTP server for manager
		* Prepare HTTP server for VOD
	*/

	logTags := log.Fields{
		"module": "global", "component": "system-control-node", "instance": nodeName,
	}

	theNode := ControlNode{wg: sync.WaitGroup{}}
	theNode.workerCtxt, theNode.workerCtxtCancel = context.WithCancel(parentCtxt)

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
		return nil, err
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
		return nil, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized metrics framework")
	initStep++

	// ====================================================================================
	// Prepare base layer - Persistence

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initializing persistence layer")

	sqlDSN, err := db.GetPostgresDialector(config.Postgres, psqlPassword)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define Postgres connection DSN")
		return nil, err
	}

	dbConns, err := db.NewSQLConnection(sqlDSN, logger.Error, false)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define SQL connection manager")
		return nil, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized persistence layer")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing memcached video cache")

	// Build memcached segment cache
	cache, err := utils.NewMemcachedVideoSegmentCache(
		parentCtxt, config.VODConfig.Cache.Servers, metrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define memcached segment cache")
		return nil, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized memcached video cache")
	initStep++

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initializing S3 client")

	s3Client, err := utils.NewS3Client(config.VODConfig.RecordingStorage.S3)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create S3 client")
		return nil, err
	}

	log.WithFields(logTags).WithField("initialize", initStep).Info("Initialized S3 client")
	initStep++

	// ====================================================================================
	// Prepare segment management

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing live stream segment manager")

	// Build segment manager
	theNode.liveStreamSegmentMgmt, err = control.NewLiveStreamSegmentManager(
		parentCtxt,
		dbConns,
		cache,
		config.VODConfig.SegReceiverTrackingWindow(),
		metrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define segment manager")
		return nil, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized live stream segment manager")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing VOD server segment reader")

	// Define segment reader
	vodSegmentReader, err := utils.NewSegmentReader(
		parentCtxt,
		config.VODConfig.SegmentReaderWorkerCount,
		config.VODConfig.SegmentReadMaxWaitTime(),
		s3Client,
		metrics,
		taskProcessorMetricsAgent,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create segment reader")
		return nil, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized VOD server segment reader")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing VOD server segment manager")

	// Define segment manager
	vodSegmentMgnt, err := vod.NewSegmentManager(
		parentCtxt, cache, vodSegmentReader, config.VODConfig.SegmentCacheTTL(), metrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create video segment manager")
		return nil, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized VOD server segment reader")
	initStep++

	// ====================================================================================
	// Prepare base layer - RPC clients

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing RPC clients")

	// Prepare core request-response client
	theNode.psClient, theNode.rrClient, err = buildReqRespClients(
		parentCtxt, nodeName, config.Management.RRClient, pubsubMetricsAgent,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("PubSub request-response client initialization failed")
		return nil, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized RPC clients")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Preparing needed PubSub topic and subscriptions")

	// Build subscription to listen to broadcast events
	broadcastSubName := fmt.Sprintf("ctrl-node.%s.%s", nodeName, config.BroadcastSystem.PubSub.Topic)
	{
		// Create broadcast topic if necessary
		_, err := theNode.psClient.GetTopic(parentCtxt, config.BroadcastSystem.PubSub.Topic)
		if err != nil {
			err := theNode.psClient.CreateTopic(
				parentCtxt,
				config.BroadcastSystem.PubSub.Topic,
				&pubsub.TopicConfig{
					RetentionDuration: config.BroadcastSystem.PubSub.MsgTTL(),
				},
			)
			if err != nil {
				log.
					WithError(err).
					WithFields(logTags).
					WithField("topic", config.BroadcastSystem.PubSub.Topic).
					Error("Failed to create system broadcast PubSub topic")
				return nil, err
			}
		}

		// Create broadcast subscription if necessary
		_, err = theNode.psClient.GetSubscription(parentCtxt, broadcastSubName)
		if err != nil {
			err := theNode.psClient.CreateSubscription(
				parentCtxt,
				config.BroadcastSystem.PubSub.Topic,
				broadcastSubName,
				pubsub.SubscriptionConfig{
					RetentionDuration: config.BroadcastSystem.PubSub.MsgTTL(),
				},
			)
			if err != nil {
				log.
					WithError(err).
					WithFields(logTags).
					WithField("topic", config.BroadcastSystem.PubSub.Topic).
					WithField("subscription", broadcastSubName).
					Error("Failed to create system broadcast PubSub topic subscription")
				return nil, err
			}
		}
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Prepared needed PubSub topic and subscriptions")
	initStep++

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing RPC driver")

	// Define controller-to-edge request-response client
	ctrlToEdgeRRClient, err := control.NewEdgeRequestClient(
		parentCtxt,
		nodeName,
		theNode.rrClient,
		config.Management.RRClient.MaxOutboundRequestDuration(),
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create controller-to-edge request-response client")
		return nil, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized RPC driver")
	initStep++

	// ====================================================================================
	// Prepare control unit

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing system manager")

	// Define the system manager
	theNode.coreManager, err = control.NewManager(
		parentCtxt,
		dbConns,
		ctrlToEdgeRRClient,
		config.Management.RecordingManagement.StartRecordIgnoreRRError,
		s3Client,
		config.Management.SourceManagement.StatusReportMaxDelay(),
		config.Management.RecordingManagement.SegmentCleanupInt(),
		metrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define system manager")
		return nil, err
	}

	// Link manager with request-response client
	ctrlToEdgeRRClient.InstallReferenceToManager(theNode.coreManager)

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized system manager")
	initStep++

	// ====================================================================================
	// Prepare admin REST API

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing system manager REST API server")

	theNode.MgmtAPIServer, err = api.BuildSystemManagementServer(
		config.Management.APIServer, theNode.coreManager, httpMetricsAgent,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create system management API HTTP server")
		return nil, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized system manager REST API server")
	initStep++

	// ====================================================================================
	// Prepare control VOD server

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initializing VOD REST API server")

	// Define live VOD playlist builder
	plBuilder, err := vod.NewPlaylistBuilder(
		parentCtxt, dbConns, config.VODConfig.LiveVODSegmentCount, metrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create VOD playlist builder")
		return nil, err
	}

	theNode.playlistManager, err = vod.NewPlaylistManager(
		parentCtxt,
		dbConns,
		config.VODConfig.SegmentReaderWorkerCount,
		plBuilder,
		vodSegmentMgnt,
		taskProcessorMetricsAgent,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create video playlist manager")
		return nil, err
	}

	// Define VOD API HTTP server
	theNode.VODAPIServer, err = api.BuildCentralVODServer(
		parentCtxt,
		config.VODConfig.APIServer,
		theNode.liveStreamSegmentMgmt.RegisterLiveStreamSegment,
		dbConns,
		theNode.playlistManager,
		httpMetricsAgent,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create VOD API HTTP server")
		return nil, err
	}

	log.
		WithFields(logTags).
		WithField("initialize", initStep).
		Info("Initialized VOD REST API server")

	// ====================================================================================
	// Register manager to process broadcast message

	theNode.wg.Add(1)
	go func() {
		defer theNode.wg.Done()
		log.WithFields(logTags).Info("Starting broadcast message subscription task")
		defer log.WithFields(logTags).Info("Broadcast message subscription task stopped")
		if err := theNode.psClient.Subscribe(
			theNode.workerCtxt, broadcastSubName, theNode.coreManager.ProcessBroadcastMsgs,
		); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("topic", config.BroadcastSystem.PubSub.Topic).
				WithField("subscription", broadcastSubName).
				Error("Broadcast channel subscription failure")
			panic(err)
		}
	}()

	return &theNode, nil
}
