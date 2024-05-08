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
	"github.com/oklog/ulid/v2"
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
		mock.AnythingOfType("context.backgroundCtx"),
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
		mock.AnythingOfType("context.backgroundCtx"),
		"video-00",
	).Return(videoInfo, nil).Once()
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
		assert.IsType(ipc.GetVideoSourceByNameResponse{}, parsed)

		asType := parsed.(ipc.GetVideoSourceByNameResponse)
		assert.EqualValues(videoInfo, asType.Source)
	}).Return(nil).Once()

	assert.Nil(requestInject(utCtxt, request))

	// --------------------------------------------------------------------------
	// Case 2: send request for video source, but failed

	mockSystem.On(
		"GetVideoSourceByName",
		mock.AnythingOfType("context.backgroundCtx"),
		"video-00",
	).Return(videoInfo, fmt.Errorf("dummy error")).Once()
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

func TestControlToEdgeListActiveRecordingOfSource(t *testing.T) {
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
		mock.AnythingOfType("context.backgroundCtx"),
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
		request := ipc.NewListActiveRecordingsRequest(uuid.NewString())
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
	// Case 1: send request for active recording of source

	testSourceID := uuid.NewString()
	var request goutils.ReqRespMessage
	{
		requestCore := ipc.NewListActiveRecordingsRequest(testSourceID)
		requestStr, err := json.Marshal(&requestCore)
		assert.Nil(err)
		request = goutils.ReqRespMessage{
			SenderID:  controlName,
			RequestID: uuid.NewString(),
			Payload:   requestStr,
		}
	}
	recordings := []common.Recording{
		{ID: ulid.Make().String(), SourceID: testSourceID, Active: 1},
		{ID: ulid.Make().String(), SourceID: testSourceID, Active: 1},
		{ID: ulid.Make().String(), SourceID: testSourceID, Active: 1},
	}

	mockSystem.On(
		"ListRecordingSessionsOfSource",
		mock.AnythingOfType("context.backgroundCtx"),
		testSourceID,
		true,
	).Return(recordings, nil).Once()
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
		assert.IsType(ipc.ListActiveRecordingsResponse{}, parsed)

		asType := parsed.(ipc.ListActiveRecordingsResponse)
		assert.Len(asType.Recordings, len(recordings))
		for idx, entry := range asType.Recordings {
			assert.Equal(recordings[idx].ID, entry.ID)
			assert.Equal(recordings[idx].SourceID, entry.SourceID)
		}
	}).Return(nil).Once()

	assert.Nil(requestInject(utCtxt, request))

	// --------------------------------------------------------------------------
	// Case 1: send request for active recording of source, but failed

	mockSystem.On(
		"ListRecordingSessionsOfSource",
		mock.AnythingOfType("context.backgroundCtx"),
		testSourceID,
		true,
	).Return(recordings, fmt.Errorf("dummy error")).Once()
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

func TestControlToEdgeChangeVideoStreamingState(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for object initialization

	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("context.backgroundCtx"),
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
		mock.AnythingOfType("context.backgroundCtx"),
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

func TestControlToEdgeStartRecording(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for object initialization

	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("context.backgroundCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Return(nil).Once()

	controlName := "ut-controller"
	uut, err := control.NewEdgeRequestClient(utCtxt, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 0: successful request

	testRRTargetID := uuid.NewString()
	testSource := common.VideoSource{ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID}
	testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSource.ID}

	testResponse := ipc.NewGeneralResponse(true, "")

	// Prepare mocks for request
	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
		testRRTargetID,
		mock.AnythingOfType("[]uint8"),
		mock.AnythingOfType("map[string]string"),
		mock.AnythingOfType("goutils.RequestCallParam"),
	).Run(func(args mock.Arguments) {
		requestRaw := args.Get(2).([]byte)
		requestParam := args.Get(4).(goutils.RequestCallParam)

		// Parse the request
		p, err := ipc.ParseRawMessage(requestRaw)
		assert.Nil(err)
		assert.IsType(ipc.StartVideoRecordingRequest{}, p)
		request, ok := p.(ipc.StartVideoRecordingRequest)
		assert.True(ok)
		assert.Equal(testRecording.ID, request.Session.ID)
		assert.Equal(testRecording.SourceID, request.Session.SourceID)

		// Send a response back
		t, err := json.Marshal(&testResponse)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	err = uut.StartRecordingSession(utCtxt, testSource, testRecording)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 1: failed request

	testSource = common.VideoSource{ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID}
	testRecording = common.Recording{ID: uuid.NewString(), SourceID: testSource.ID}

	testResponse = ipc.NewGeneralResponse(false, "dummy error")

	// Prepare mocks for request
	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
		testRRTargetID,
		mock.AnythingOfType("[]uint8"),
		mock.AnythingOfType("map[string]string"),
		mock.AnythingOfType("goutils.RequestCallParam"),
	).Run(func(args mock.Arguments) {
		requestRaw := args.Get(2).([]byte)
		requestParam := args.Get(4).(goutils.RequestCallParam)

		// Parse the request
		p, err := ipc.ParseRawMessage(requestRaw)
		assert.Nil(err)
		assert.IsType(ipc.StartVideoRecordingRequest{}, p)
		request, ok := p.(ipc.StartVideoRecordingRequest)
		assert.True(ok)
		assert.Equal(testRecording.ID, request.Session.ID)
		assert.Equal(testRecording.SourceID, request.Session.SourceID)

		// Send a response back
		t, err := json.Marshal(&testResponse)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	err = uut.StartRecordingSession(utCtxt, testSource, testRecording)
	assert.NotNil(err)
	assert.Equal("dummy error", err.Error())
}

func TestControlToEdgeStopRecording(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	// --------------------------------------------------------------------------
	// Prepare mocks for object initialization

	mockRRClient.On(
		"SetInboundRequestHandler",
		mock.AnythingOfType("context.backgroundCtx"),
		mock.AnythingOfType("goutils.ReqRespMessageHandler"),
	).Return(nil).Once()

	controlName := "ut-controller"
	uut, err := control.NewEdgeRequestClient(utCtxt, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 0: successful request

	testRRTargetID := uuid.NewString()
	testSource := common.VideoSource{ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID}
	testRecordingID := uuid.NewString()
	currentTime := time.Now().UTC()

	testResponse := ipc.NewGeneralResponse(true, "")

	// Prepare mocks for request
	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
		testRRTargetID,
		mock.AnythingOfType("[]uint8"),
		mock.AnythingOfType("map[string]string"),
		mock.AnythingOfType("goutils.RequestCallParam"),
	).Run(func(args mock.Arguments) {
		requestRaw := args.Get(2).([]byte)
		requestParam := args.Get(4).(goutils.RequestCallParam)

		// Parse the request
		p, err := ipc.ParseRawMessage(requestRaw)
		assert.Nil(err)
		assert.IsType(ipc.StopVideoRecordingRequest{}, p)
		request, ok := p.(ipc.StopVideoRecordingRequest)
		assert.True(ok)
		assert.Equal(testRecordingID, request.RecordingID)
		assert.Equal(currentTime, request.EndTime)

		// Send a response back
		t, err := json.Marshal(&testResponse)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	err = uut.StopRecordingSession(utCtxt, testSource, testRecordingID, currentTime)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Case 1: failed request

	testSource = common.VideoSource{ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID}
	testRecordingID = uuid.NewString()

	testResponse = ipc.NewGeneralResponse(false, "dummy error")

	// Prepare mocks for request
	mockRRClient.On(
		"Request",
		mock.AnythingOfType("context.backgroundCtx"),
		testRRTargetID,
		mock.AnythingOfType("[]uint8"),
		mock.AnythingOfType("map[string]string"),
		mock.AnythingOfType("goutils.RequestCallParam"),
	).Run(func(args mock.Arguments) {
		requestRaw := args.Get(2).([]byte)
		requestParam := args.Get(4).(goutils.RequestCallParam)

		// Parse the request
		p, err := ipc.ParseRawMessage(requestRaw)
		assert.Nil(err)
		assert.IsType(ipc.StopVideoRecordingRequest{}, p)
		request, ok := p.(ipc.StopVideoRecordingRequest)
		assert.True(ok)
		assert.Equal(testRecordingID, request.RecordingID)
		assert.Equal(currentTime, request.EndTime)

		// Send a response back
		t, err := json.Marshal(&testResponse)
		assert.Nil(err)
		assert.Nil(requestParam.RespHandler(utCtxt, goutils.ReqRespMessage{Payload: t}))
	}).Return(uuid.NewString(), nil).Once()

	err = uut.StopRecordingSession(utCtxt, testSource, testRecordingID, currentTime)
	assert.NotNil(err)
	assert.Equal("dummy error", err.Error())
}
