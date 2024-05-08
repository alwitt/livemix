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
	"github.com/oklog/ulid/v2"
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
		mock.AnythingOfType("context.backgroundCtx"),
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
		ID:                  uuid.NewString(),
		Name:                uuid.NewString(),
		TargetSegmentLength: 4,
		PlaylistURI:         nil,
		Streaming:           -1,
	})

	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
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
		mock.AnythingOfType("context.backgroundCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Return(nil).Once()

	edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := edge.NewControlRequestClient(utCtxt, edgeName, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Prepare mocks for the request

	targetSource := "video-00"

	testResponse := ipc.NewGeneralResponse(false, "dummy error")

	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
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
		mock.AnythingOfType("context.backgroundCtx"),
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
		mock.AnythingOfType("context.backgroundCtx"),
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
		mock.AnythingOfType("context.backgroundCtx"),
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
		request := ipc.NewChangeSourceStreamingStateRequest(uuid.NewString(), 0)
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
		requestCore := ipc.NewChangeSourceStreamingStateRequest(sourceID, 1)
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
		mock.AnythingOfType("context.backgroundCtx"),
		sourceID,
		1,
	).Return(nil).Once()
	mockRRClient.On(
		"Respond",
		mock.AnythingOfType("context.backgroundCtx"),
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
		mock.AnythingOfType("context.backgroundCtx"),
		sourceID,
		1,
	).Return(fmt.Errorf("dummy error")).Once()
	mockRRClient.On(
		"Respond",
		mock.AnythingOfType("context.backgroundCtx"),
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

func TestEdgeToControlStartRecording(t *testing.T) {
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
		mock.AnythingOfType("context.backgroundCtx"),
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
		request := ipc.NewStartVideoRecordingSessionRequest(common.Recording{})
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
	// Case 1: send request to start recording

	requestCore := ipc.NewStartVideoRecordingSessionRequest(common.Recording{
		ID: uuid.NewString(), SourceID: uuid.NewString(),
	})
	requestCoreStr, err := json.Marshal(&requestCore)
	assert.Nil(err)
	request := goutils.ReqRespMessage{
		SenderID:  controlName,
		RequestID: uuid.NewString(),
		Payload:   requestCoreStr,
	}

	{
		// Prepare mocks
		mockEdge.On(
			"StartRecording",
			mock.AnythingOfType("context.backgroundCtx"),
			requestCore.Session,
		).Return(nil).Once()
		mockRRClient.On(
			"Respond",
			mock.AnythingOfType("context.backgroundCtx"),
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
	}

	// --------------------------------------------------------------------------
	// Case 2: send request to start recording, but failed

	{
		// Prepare mocks
		mockEdge.On(
			"StartRecording",
			mock.AnythingOfType("context.backgroundCtx"),
			requestCore.Session,
		).Return(fmt.Errorf("dummy error")).Once()
		mockRRClient.On(
			"Respond",
			mock.AnythingOfType("context.backgroundCtx"),
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
}

func TestEdgeToControlStopRecording(t *testing.T) {
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
		mock.AnythingOfType("context.backgroundCtx"),
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
		request := ipc.NewStopVideoRecordingSessionRequest(uuid.NewString(), time.Now())
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
	// Case 1: send request to stop recording

	requestCore := ipc.NewStopVideoRecordingSessionRequest(uuid.NewString(), time.Now().UTC())
	requestCoreStr, err := json.Marshal(&requestCore)
	assert.Nil(err)
	request := goutils.ReqRespMessage{
		SenderID:  controlName,
		RequestID: uuid.NewString(),
		Payload:   requestCoreStr,
	}

	{
		// Prepare mocks
		mockEdge.On(
			"StopRecording",
			mock.AnythingOfType("context.backgroundCtx"),
			requestCore.RecordingID,
			requestCore.EndTime,
		).Return(nil).Once()
		mockRRClient.On(
			"Respond",
			mock.AnythingOfType("context.backgroundCtx"),
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
	}

	// --------------------------------------------------------------------------
	// Case 2: send request to stop recording, but failed

	{
		// Prepare mocks
		mockEdge.On(
			"StopRecording",
			mock.AnythingOfType("context.backgroundCtx"),
			requestCore.RecordingID,
			requestCore.EndTime,
		).Return(fmt.Errorf("dummy error")).Once()
		mockRRClient.On(
			"Respond",
			mock.AnythingOfType("context.backgroundCtx"),
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
}

func TestEdgeToControlStopAllRecordings(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for object initialization

	// var requestInject goutils.ReqRespMessageHandler
	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("context.backgroundCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Return(nil).Once()

	edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := edge.NewControlRequestClient(utCtxt, edgeName, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 0: success response

	testSourceID := uuid.NewString()
	testResponse := ipc.NewGeneralResponse(true, "")

	// Prepare mocks for the request
	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
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
		assert.IsType(ipc.CloseAllActiveRecordingRequest{}, p)
		request, ok := p.(ipc.CloseAllActiveRecordingRequest)
		assert.True(ok)
		assert.Equal(testSourceID, request.SourceID)

		// Send a response back
		t, err := json.Marshal(&testResponse)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	// Make request
	err = uut.StopAllAssociatedRecordings(utCtxt, testSourceID)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 1: failure response

	testSourceID = uuid.NewString()
	testResponse = ipc.NewGeneralResponse(false, "dummy response")

	// Prepare mocks for the request
	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
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
		assert.IsType(ipc.CloseAllActiveRecordingRequest{}, p)
		request, ok := p.(ipc.CloseAllActiveRecordingRequest)
		assert.True(ok)
		assert.Equal(testSourceID, request.SourceID)

		// Send a response back
		t, err := json.Marshal(&testResponse)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	// Make request
	err = uut.StopAllAssociatedRecordings(utCtxt, testSourceID)
	assert.NotNil(err)
	assert.Equal("dummy response", err.Error())
}

func TestEdgeToControlListActiveRecordingsOfSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for object initialization

	// var requestInject goutils.ReqRespMessageHandler
	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("context.backgroundCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Return(nil).Once()

	edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := edge.NewControlRequestClient(utCtxt, edgeName, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 0: success response

	testSourceID := uuid.NewString()
	testResponse0 := ipc.NewListActiveRecordingsResponse(
		[]common.Recording{
			{ID: ulid.Make().String(), SourceID: testSourceID, Active: 1},
			{ID: ulid.Make().String(), SourceID: testSourceID, Active: 1},
			{ID: ulid.Make().String(), SourceID: testSourceID, Active: 1},
		},
	)

	// Prepare mocks for the request
	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
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
		assert.IsType(ipc.ListActiveRecordingsRequest{}, p)
		request, ok := p.(ipc.ListActiveRecordingsRequest)
		assert.True(ok)
		assert.Equal(testSourceID, request.SourceID)

		// Send a response back
		t, err := json.Marshal(&testResponse0)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	received, err := uut.ListActiveRecordingsOfSource(utCtxt, testSourceID)
	assert.Nil(err)
	assert.Len(received, len(testResponse0.Recordings))
	for idx, entry := range received {
		assert.Equal(testResponse0.Recordings[idx].ID, entry.ID)
		assert.Equal(testResponse0.Recordings[idx].SourceID, entry.SourceID)
	}

	// --------------------------------------------------------------------------
	// Case 1: failure response

	testSourceID = uuid.NewString()
	testResponse1 := ipc.NewGeneralResponse(false, "dummy error")

	// Prepare mocks for the request
	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
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
		assert.IsType(ipc.ListActiveRecordingsRequest{}, p)
		request, ok := p.(ipc.ListActiveRecordingsRequest)
		assert.True(ok)
		assert.Equal(testSourceID, request.SourceID)

		// Send a response back
		t, err := json.Marshal(&testResponse1)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	_, err = uut.ListActiveRecordingsOfSource(utCtxt, testSourceID)
	assert.NotNil(err)
}
