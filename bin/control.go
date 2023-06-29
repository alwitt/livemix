package bin

import (
	"context"
	"net/http"

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
	psClient      goutils.PubSubClient
	rrClient      goutils.RequestResponseClient
	MgmtAPIServer *http.Server
}

/*
Cleanup stop and clean up the system control node

	@param ctxt context.Context - execution context
*/
func (n ControlNode) Cleanup(ctxt context.Context) error {
	if err := n.rrClient.Stop(ctxt); err != nil {
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
) (ControlNode, error) {
	/*
		Steps for preparing the control are

		* Prepare database
		* Prepare system manager
		* Prepare request-response client for manager
		* Prepare HTTP server for manager
	*/

	theNode := ControlNode{}

	sqlDSN, err := db.GetPostgresDialector(config.Postgres, psqlPassword)
	if err != nil {
		log.WithError(err).Error("Failed to define Postgres connection DSN")
		return theNode, err
	}

	// Define the persistence manager
	dbManager, err := db.NewManager(sqlDSN, logger.Error)
	if err != nil {
		log.WithError(err).Error("Failed to define persistence manager")
		return theNode, err
	}

	// IMPORTANT: for now, the persistence manager will function as the system manager.
	// This will change in the future.

	// Prepare core request-response client
	theNode.psClient, theNode.rrClient, err = buildReqRespClients(
		parentCtxt, nodeName, config.Management.RRClient,
	)
	if err != nil {
		log.WithError(err).Error("PubSub request-response client initialization failed")
		return theNode, err
	}

	// Define controller-to-edge request-response client
	ctrlToEdgeRRClient, err := control.NewEdgeRequestClient(parentCtxt, nodeName, theNode.rrClient)
	if err != nil {
		log.WithError(err).Error("Failed to create controller-to-edge request-response client")
		return theNode, err
	}
	// Link manager with request-response client
	ctrlToEdgeRRClient.InstallReferenceToManager(dbManager)

	// Define manager API HTTP server
	mgmtAPIServer, err := api.BuildSystemManagementServer(config.Management.APIServer, dbManager)
	if err != nil {
		log.WithError(err).Error("Failed to create system management API HTTP server")
		return theNode, err
	}
	theNode.MgmtAPIServer = mgmtAPIServer

	return theNode, nil
}
