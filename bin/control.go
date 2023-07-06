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
	MgmtAPIServer   *http.Server
}

/*
Cleanup stop and clean up the system control node

	@param ctxt context.Context - execution context
*/
func (n *ControlNode) Cleanup(ctxt context.Context) error {
	n.psSubCtxtCancel()
	if err := n.rrClient.Stop(ctxt); err != nil {
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
		* Prepare request-response client for manager
		* Prepare HTTP server for manager
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

	// Define the persistence manager
	dbManager, err := db.NewManager(sqlDSN, logger.Error)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define persistence manager")
		return nil, err
	}

	// Define the system manager
	systemManager, err := control.NewManager(dbManager)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define system manager")
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
					RetentionDuration: time.Second * time.Duration(config.BroadcastSystem.PubSub.MsgTTLInSec),
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
					RetentionDuration: time.Second * time.Duration(config.BroadcastSystem.PubSub.MsgTTLInSec),
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
		time.Second*time.Duration(config.Management.RRClient.MaxOutboundRequestDurationInSec),
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create controller-to-edge request-response client")
		return nil, err
	}
	// Link manager with request-response client
	ctrlToEdgeRRClient.InstallReferenceToManager(systemManager)

	// Define manager API HTTP server
	mgmtAPIServer, err := api.BuildSystemManagementServer(config.Management.APIServer, systemManager)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create system management API HTTP server")
		return nil, err
	}
	theNode.MgmtAPIServer = mgmtAPIServer

	// Register manager to process broadcast message
	theNode.wg.Add(1)
	go func() {
		defer theNode.wg.Done()
		if err := theNode.psClient.Subscribe(
			theNode.psSubCtxt, broadcastSubName, systemManager.ProcessBroadcastMsgs,
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
