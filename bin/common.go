package bin

import (
	"context"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
)

// buildReqRespClients helper function for defining all the PubSub request-response client parts
func buildReqRespClients(
	ctxt context.Context,
	nodeName string,
	config common.ReqRespClientConfig,
	metrics goutils.PubSubMetricHelper,
) (goutils.PubSubClient, goutils.RequestResponseClient, error) {
	var psClient goutils.PubSubClient
	var rrClient goutils.RequestResponseClient
	rawPSClient, err := goutils.CreateBasicGCPPubSubClient(
		ctxt, config.GCPProject,
	)
	if err != nil {
		log.WithError(err).Error("Failed to create core PubSub client")
		return psClient, rrClient, err
	}

	// Define PubSub client
	psClient, err = goutils.GetNewPubSubClientInstance(rawPSClient, log.Fields{
		"module": "go-utils", "component": "pubsub-client", "project": config.GCPProject,
	}, metrics)
	if err != nil {
		log.WithError(err).Error("Failed to create PubSub client")
		return psClient, rrClient, err
	}

	// Sync PubSub client with currently existing topics and subscriptions
	if err := psClient.UpdateLocalTopicCache(ctxt); err != nil {
		log.WithError(err).Error("Errored when syncing existing topics in GCP project")
		return psClient, rrClient, err
	}
	if err := psClient.UpdateLocalSubscriptionCache(ctxt); err != nil {
		log.WithError(err).Error("Errored when syncing existing subscriptions in GCP project")
		return psClient, rrClient, err
	}

	// Define PubSub request-response client
	rrClient, err = goutils.GetNewPubSubRequestResponseClientInstance(
		ctxt, goutils.PubSubRequestResponseClientParam{
			TargetID:        config.InboudRequestTopic.Topic,
			Name:            nodeName,
			PSClient:        psClient,
			MsgRetentionTTL: config.InboudRequestTopic.MsgTTL(),
			LogTags: log.Fields{
				"module":    "go-utils",
				"component": "pubsub-req-resp-client",
				"project":   config.GCPProject,
			},
			CustomLogModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
			TimeoutEnforceInt:  config.RequestTimeoutEnforceInt(),
			SupportWorkerCount: config.SupportWorkerCount,
		},
	)
	if err != nil {
		log.WithError(err).Error("Failed to create PubSub request-response client")
		return psClient, rrClient, err
	}

	return psClient, rrClient, nil
}

/*
NewMetricsCollector define metrics collector

	@param config common.MetricsFeatureConfig - metrics framework config
	@returns new metrics collector
*/
func NewMetricsCollector(config common.MetricsFeatureConfig) (goutils.MetricsCollector, error) {
	framework, err := goutils.GetNewMetricsCollector(
		log.Fields{"module": "utils", "component": "metrics-core"}, []goutils.LogMetadataModifier{},
	)
	if err != nil {
		return nil, err
	}
	if config.EnableAppMetrics {
		framework.InstallApplicationMetrics()
	}
	return framework, nil
}
