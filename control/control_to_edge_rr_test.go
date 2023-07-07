package control_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

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
	mockSystem := mocks.NewSystemManager(t)

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

	controlName := "ut-controller"
	uut, err := control.NewEdgeRequestClient(utCtxt, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 0: send request without a manager installed
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

	// --------------------------------------------------------------------------
	// Case 1: send request for video source, but failed

	mockSystem.On(
		"GetVideoSourceByName",
		mock.AnythingOfType("*context.emptyCtx"),
		"video-00",
	).Return(videoInfo, fmt.Errorf("dummy error")).Once()
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
		assert.IsType(ipc.GeneralResponse{}, parsed)

		asType := parsed.(ipc.GeneralResponse)
		assert.False(asType.Success)
		assert.Equal("dummy error", asType.ErrorMsg)
	}).Return(nil).Once()

	assert.Nil(requestInject(utCtxt, request))
}

func TestControlToEdgeChangeVideoStreamingState(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for object initialization

	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Return(nil).Once()

	controlName := "ut-controller"
	uut, err := control.NewEdgeRequestClient(utCtxt, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 0: video source does not contain a target request response ID

	assert.NotNil(uut.ChangeVideoStreamingState(utCtxt, common.VideoSource{}, 0))

	// --------------------------------------------------------------------------
	// Case 1: video source does not contain a target request response ID

	testSourceID := uuid.NewString()
	targetReqRespID := uuid.NewString()

	testResponse := ipc.NewGeneralResponse(false, "dummy error")

	// Prepare mocks for request
	mockRRClient.On(
		"Request",
		mock.AnythingOfType("*context.emptyCtx"),
		targetReqRespID,
		mock.AnythingOfType("[]uint8"),
		mock.AnythingOfType("map[string]string"),
		mock.AnythingOfType("goutils.RequestCallParam"),
	).Run(func(args mock.Arguments) {
		requestRaw := args.Get(2).([]byte)
		requestParam := args.Get(4).(goutils.RequestCallParam)

		// Parse the request
		p, err := ipc.ParseRawMessage(requestRaw)
		assert.Nil(err)
		assert.IsType(ipc.ChangeSourceStreamingStateRequest{}, p)
		request, ok := p.(ipc.ChangeSourceStreamingStateRequest)
		assert.True(ok)
		assert.Equal(testSourceID, request.SourceID)
		assert.Equal(1, request.NewState)

		// Send a response back
		t, err := json.Marshal(&testResponse)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	err = uut.ChangeVideoStreamingState(
		utCtxt, common.VideoSource{ID: testSourceID, ReqRespTargetID: &targetReqRespID}, 1,
	)
	assert.NotNil(err)
	assert.Equal("dummy error", err.Error())
}
