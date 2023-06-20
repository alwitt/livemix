package api_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
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

	edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := api.NewEdgeToControlRRClient(edgeName, controlName, mockRRClient, time.Second)
	assert.Nil(err)

	// --------------------------------------------------------------------------
	// Prepare mocks for the request

	targetSource := "video-00"

	testResponse := ipc.NewGetVideoSourceByNameResponse(common.VideoSource{
		ID:   uuid.NewString(),
		Name: uuid.NewString(),
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

func TestEdgeToControlGetVideoSourceInfoRequestTimeout(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockRRClient := mocks.NewRequestResponseClient(t)

	edgeName := "unit-tester"
	controlName := "ut-controller"
	uut, err := api.NewEdgeToControlRRClient(edgeName, controlName, mockRRClient, time.Second)
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
