package utils

import (
	"context"
	"encoding/json"

	"github.com/alwitt/goutils"
	"github.com/apex/log"
)

// Broadcaster message broadcasting client
type Broadcaster interface {
	/*
		Broadcast broadcast a message

			@param ctxt context.Context - execution context
			@param message interface{} - message to broadcast
	*/
	Broadcast(ctxt context.Context, message interface{}) error
}

// pubsubBroadcasterImpl implements Broadcaster
type pubsubBroadcasterImpl struct {
	goutils.Component
	psClient       goutils.PubSubClient
	broadcastTopic string
}

/*
NewPubSubBroadcaster define new PubSub message broadcast client

	@param psClient goutils.PubSubClient - PubSub client
	@param broadcastTopic string - message broadcast PubSub topic
	@returns new client
*/
func NewPubSubBroadcaster(
	psClient goutils.PubSubClient, broadcastTopic string,
) (Broadcaster, error) {
	return &pubsubBroadcasterImpl{
		Component: goutils.Component{
			LogTags: log.Fields{
				"module":          "utils",
				"component":       "pubsub-broadcaster",
				"broadcast-topic": broadcastTopic,
			},
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		}, psClient: psClient, broadcastTopic: broadcastTopic,
	}, nil
}

func (b *pubsubBroadcasterImpl) Broadcast(ctxt context.Context, message interface{}) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = b.psClient.Publish(ctxt, b.broadcastTopic, payload, nil, true)
	return err
}
