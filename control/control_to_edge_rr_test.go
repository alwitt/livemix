package control_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/control"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestControlToEdgeGetVideoSourceInfoResponse(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)
	mockSystem := mocks.NewPersistenceManager(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for object initialization

	var requestInject goutils.ReqRespMessageHandler
	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Run(func(args mock.Arguments) {
		requestInject = args.Get(1).(goutils.ReqRespMessageHandler)
	}).Return(nil).Once()

	// edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := control.NewEdgeRequestClient(utCtxt, controlName, mockRRClient)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 0: send request with a manager installed
	{
		request := ipc.NewGetVideoSourceByNameRequest("video-00")
		requestStr, err := json.Marshal(&request)
		assert.Nil(err)
		assert.NotNil(requestInject(utCtxt, goutils.ReqRespMessage{
			SenderID:  controlName,
			RequestID: uuid.NewString(),
			Payload:   requestStr,
		}))
	}

	// Install manager
	uut.InstallReferenceToManager(mockSystem)

	// --------------------------------------------------------------------------
	// Case 1: send request for video source

	// Prepare mocks
	var request goutils.ReqRespMessage
	{
		requestCore := ipc.NewGetVideoSourceByNameRequest("video-00")
		requestStr, err := json.Marshal(&requestCore)
		assert.Nil(err)
		request = goutils.ReqRespMessage{
			SenderID:  controlName,
			RequestID: uuid.NewString(),
			Payload:   requestStr,
		}
	}

	videoInfo := common.VideoSource{
		ID:   uuid.NewString(),
		Name: uuid.NewString(),
	}

	mockSystem.On(
		"GetVideoSourceByName",
		mock.AnythingOfType("*context.emptyCtx"),
		"video-00",
	).Return(videoInfo, nil).Once()
	mockRRClient.On(
		"Respond",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("goutils.ReqRespMessage"),
		mock.AnythingOfType("[]uint8"),
		mock.AnythingOfType("map[string]string"),
		false,
	).Run(func(args mock.Arguments) {
		origRequest := args.Get(1).(goutils.ReqRespMessage)
		rawResponse := args.Get(2).([]byte)

		// Verify the original request
		assert.EqualValues(request, origRequest)

		// Parse the response
		parsed, err := ipc.ParseRawMessage(rawResponse)
		assert.Nil(err)
		assert.IsType(ipc.GetVideoSourceByNameResponse{}, parsed)

		asType := parsed.(ipc.GetVideoSourceByNameResponse)
		assert.EqualValues(videoInfo, asType.Source)
	}).Return(nil).Once()

	assert.Nil(requestInject(utCtxt, request))
}