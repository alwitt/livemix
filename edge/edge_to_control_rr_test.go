package edge_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/edge"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEdgeToControlGetVideoSourceInfoRequest(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for initialization

	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Return(nil).Once()

	edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := edge.NewControlRequestClient(utCtxt, edgeName, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Prepare mocks for the request

	targetSource := "video-00"

	testResponse := ipc.NewGetVideoSourceByNameResponse(common.VideoSource{
		ID:          uuid.NewString(),
		Name:        uuid.NewString(),
		PlaylistURI: nil,
	})

	mockRRClient.On(
		"Request",
		mock.AnythingOfType("*context.emptyCtx"),
		controlName,
		mock.AnythingOfType("[]uint8"),
		mock.AnythingOfType("map[string]string"),
		mock.AnythingOfType("goutils.RequestCallParam"),
	).Run(func(args mock.Arguments) {
		requestRaw := args.Get(2).([]byte)
		requestParam := args.Get(4).(goutils.RequestCallParam)

		// Parse the request
		p, err := ipc.ParseRawMessage(requestRaw)
		assert.Nil(err)
		assert.IsType(ipc.GetVideoSourceByNameRequest{}, p)
		request, ok := p.(ipc.GetVideoSourceByNameRequest)
		assert.True(ok)
		assert.Equal(targetSource, request.TargetName)

		// Send a response back
		t, err := json.Marshal(&testResponse)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	// --------------------------------------------------------------------------
	// Make request

	recerivedInfo, err := uut.GetVideoSourceInfo(utCtxt, targetSource)
	assert.Nil(err)
	assert.EqualValues(testResponse.Source, recerivedInfo)
}

func TestEdgeToControlGetVideoSourceInfoErrorResponse(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for initialization

	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Return(nil).Once()

	edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := edge.NewControlRequestClient(utCtxt, edgeName, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Prepare mocks for the request

	targetSource := "video-00"

	testResponse := ipc.NewGetGeneralResponse(false, "dummy error")

	mockRRClient.On(
		"Request",
		mock.AnythingOfType("*context.emptyCtx"),
		controlName,
		mock.AnythingOfType("[]uint8"),
		mock.AnythingOfType("map[string]string"),
		mock.AnythingOfType("goutils.RequestCallParam"),
	).Run(func(args mock.Arguments) {
		requestRaw := args.Get(2).([]byte)
		requestParam := args.Get(4).(goutils.RequestCallParam)

		// Parse the request
		p, err := ipc.ParseRawMessage(requestRaw)
		assert.Nil(err)
		assert.IsType(ipc.GetVideoSourceByNameRequest{}, p)
		request, ok := p.(ipc.GetVideoSourceByNameRequest)
		assert.True(ok)
		assert.Equal(targetSource, request.TargetName)

		// Send a response back
		t, err := json.Marshal(&testResponse)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	// --------------------------------------------------------------------------
	// Make request

	_, err = uut.GetVideoSourceInfo(utCtxt, targetSource)
	assert.NotNil(err)
}

func TestEdgeToControlGetVideoSourceInfoRequestTimeout(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for initialization

	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Return(nil).Once()

	edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := edge.NewControlRequestClient(utCtxt, edgeName, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Prepare mocks for the request

	targetSource := "video-00"

	mockRRClient.On(
		"Request",
		mock.AnythingOfType("*context.emptyCtx"),
		controlName,
		mock.AnythingOfType("[]uint8"),
		mock.AnythingOfType("map[string]string"),
		mock.AnythingOfType("goutils.RequestCallParam"),
	).Run(func(args mock.Arguments) {
		requestRaw := args.Get(2).([]byte)
		requestParam := args.Get(4).(goutils.RequestCallParam)

		// Parse the request
		p, err := ipc.ParseRawMessage(requestRaw)
		assert.Nil(err)
		assert.IsType(ipc.GetVideoSourceByNameRequest{}, p)
		request, ok := p.(ipc.GetVideoSourceByNameRequest)
		assert.True(ok)
		assert.Equal(targetSource, request.TargetName)

		// Send a response back
		assert.Nil(requestParam.TimeoutHandler(utCtxt))
	}).Return(uuid.NewString(), nil).Once()

	// --------------------------------------------------------------------------
	// Make request

	_, err = uut.GetVideoSourceInfo(utCtxt, targetSource)
	assert.NotNil(err)
}

func TestEdgeToControlChangeStreamingState(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)
	mockEdge := mocks.NewVideoSourceOperator(t)

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

	edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := edge.NewControlRequestClient(utCtxt, edgeName, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 0: send request without a manager installed
	{
		request := ipc.NewChangeSourceStreamingStateRequest(uuid.NewString(), false)
		requestStr, err := json.Marshal(&request)
		assert.Nil(err)
		assert.NotNil(requestInject(utCtxt, goutils.ReqRespMessage{
			SenderID:  controlName,
			RequestID: uuid.NewString(),
			Payload:   requestStr,
		}))
	}

	// Install manager
	uut.InstallReferenceToManager(mockEdge)

	// --------------------------------------------------------------------------
	// Case 1: send request to change streaming state

	// Prepare mocks
	sourceID := uuid.NewString()
	var request goutils.ReqRespMessage
	{
		requestCore := ipc.NewChangeSourceStreamingStateRequest(sourceID, true)
		requestStr, err := json.Marshal(&requestCore)
		assert.Nil(err)
		request = goutils.ReqRespMessage{
			SenderID:  controlName,
			RequestID: uuid.NewString(),
			Payload:   requestStr,
		}
	}

	mockEdge.On(
		"ChangeVideoSourceStreamState",
		mock.AnythingOfType("*context.emptyCtx"),
		sourceID,
		true,
	).Return(nil).Once()
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
		assert.True(asType.Success)
	}).Return(nil).Once()

	assert.Nil(requestInject(utCtxt, request))

	// --------------------------------------------------------------------------
	// Case 2: send request to change streaming state, but failed

	mockEdge.On(
		"ChangeVideoSourceStreamState",
		mock.AnythingOfType("*context.emptyCtx"),
		sourceID,
		true,
	).Return(fmt.Errorf("dummy error")).Once()
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
