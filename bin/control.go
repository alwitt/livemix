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
	psClient        goutils.PubSubClient
	rrClient        goutils.RequestResponseClient
	wg              sync.WaitGroup
	psSubCtxt       context.Context
	psSubCtxtCancel context.CancelFunc
	segmentMgmt     control.CentralSegmentManager
	coreManager     control.SystemManager
	MgmtAPIServer   *http.Server
	VODAPIServer    *http.Server
}

/*
Cleanup stop and clean up the system control node

	@param ctxt context.Context - execution context
*/
func (n *ControlNode) Cleanup(ctxt context.Context) error {
	n.psSubCtxtCancel()
	if err := n.segmentMgmt.Stop(ctxt); err != nil {
		return nil
	}
	if err := n.rrClient.Stop(ctxt); err != nil {
		return err
	}
	if err := n.coreManager.Stop(ctxt); err != nil {
		return err
	}
	if err := goutils.TimeBoundedWaitGroupWait(ctxt, &n.wg, time.Second*5); err != nil {
		return err
	}
	return n.psClient.Close(ctxt)
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

	sqlDSN, err := db.GetPostgresDialector(config.Postgres, psqlPassword)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define Postgres connection DSN")
		return nil, err
	}

	dbConns, err := db.NewSQLConnection(sqlDSN, logger.Error)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define SQL connection manager")
		return nil, err
	}

	// Prepare core request-response client
	theNode.psClient, theNode.rrClient, err = buildReqRespClients(
		parentCtxt, nodeName, config.Management.RRClient,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("PubSub request-response client initialization failed")
		return nil, err
	}

	// Build subscription to listen to broadcast events
	theNode.psSubCtxt, theNode.psSubCtxtCancel = context.WithCancel(parentCtxt)
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

	// Define S3 client
	s3Client, err := utils.NewS3Client(config.VODConfig.RecordingStorage.S3)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create S3 client")
		return nil, err
	}

	// Define the system manager
	theNode.coreManager, err = control.NewManager(
		parentCtxt,
		dbConns,
		ctrlToEdgeRRClient,
		s3Client,
		config.Management.SourceManagement.StatusReportMaxDelay(),
		config.Management.RecordingManagement.SegmentCleanupInt(),
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define system manager")
		return nil, err
	}

	// Link manager with request-response client
	ctrlToEdgeRRClient.InstallReferenceToManager(theNode.coreManager)

	// Build memcached segment cache
	cache, err := utils.NewMemcachedVideoSegmentCache(config.VODConfig.Cache.Servers)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define memcached segment cache")
		return nil, err
	}

	// Build segment manager
	theNode.segmentMgmt, err = control.NewSegmentManager(
		parentCtxt,
		dbConns,
		cache,
		config.VODConfig.SegReceiverTrackingWindow(),
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define segment manager")
		return nil, err
	}

	// Define manager API HTTP server
	theNode.MgmtAPIServer, err = api.BuildSystemManagementServer(
		config.Management.APIServer, theNode.coreManager,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create system management API HTTP server")
		return nil, err
	}

	// Define segment reader
	segmentReader, err := utils.NewSegmentReader(
		parentCtxt, config.VODConfig.SegmentReaderWorkerCount, s3Client,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create segment reader")
		return nil, err
	}

	// Define segment manager
	segmentMgnt, err := vod.NewSegmentManager(cache, segmentReader, config.VODConfig.SegmentCacheTTL())
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create video segment manager")
		return nil, err
	}

	// Define live VOD playlist builder
	plBuilder, err := vod.NewPlaylistBuilder(dbConns, config.VODConfig.LiveVODSegmentCount)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to create VOD playlist builder")
		return nil, err
	}

	// Define VOD API HTTP server
	theNode.VODAPIServer, err = api.BuildCentralVODServer(
		parentCtxt,
		config.VODConfig.APIServer,
		theNode.segmentMgmt.RegisterLiveStreamSegment,
		dbConns,
		plBuilder,
		segmentMgnt,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create VOD API HTTP server")
		return nil, err
	}

	// Register manager to process broadcast message
	theNode.wg.Add(1)
	go func() {
		defer theNode.wg.Done()
		if err := theNode.psClient.Subscribe(
			theNode.psSubCtxt, broadcastSubName, theNode.coreManager.ProcessBroadcastMsgs,
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
