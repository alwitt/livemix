package ipc_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestIPCMessageParsing(t *testing.T) {
	assert := assert.New(t)

	type testCase struct {
		input     interface{}
		inputType reflect.Type
	}

	currentTime := time.Now().UTC()

	testCases := []testCase{
		{
			input:     ipc.NewGetVideoSourceByNameRequest(uuid.NewString()),
			inputType: reflect.TypeOf(ipc.GetVideoSourceByNameRequest{}),
		},
		{
			input: ipc.NewGetVideoSourceByNameResponse(common.VideoSource{
				ID:   uuid.NewString(),
				Name: uuid.NewString(),
			}),
			inputType: reflect.TypeOf(ipc.GetVideoSourceByNameResponse{}),
		},
		{
			input:     ipc.NewChangeSourceStreamingStateRequest(uuid.NewString(), 1),
			inputType: reflect.TypeOf(ipc.ChangeSourceStreamingStateRequest{}),
		},
		{
			input:     ipc.NewGeneralResponse(false, uuid.NewString()),
			inputType: reflect.TypeOf(ipc.GeneralResponse{}),
		},
		{
			input:     ipc.NewVideoSourceStatusReport(uuid.NewString(), uuid.NewString(), currentTime),
			inputType: reflect.TypeOf(ipc.VideoSourceStatusReport{}),
		},
		{
			input:     ipc.NewStartVideoRecordingSessionRequest(common.Recording{ID: uuid.NewString()}),
			inputType: reflect.TypeOf(ipc.StartVideoRecordingRequest{}),
		},
		{
			input:     ipc.NewStopVideoRecordingSessionRequest(uuid.NewString(), time.Now().UTC()),
			inputType: reflect.TypeOf(ipc.StopVideoRecordingRequest{}),
		},
		{
			input:     ipc.NewCloseAllActiveRecordingRequest(uuid.NewString()),
			inputType: reflect.TypeOf(ipc.CloseAllActiveRecordingRequest{}),
		},
		{
			input:     ipc.NewRecordingSegmentReport([]string{uuid.NewString(), uuid.NewString()}, nil),
			inputType: reflect.TypeOf(ipc.RecordingSegmentReport{}),
		},
		{
			input:     ipc.NewListActiveRecordingsRequest(uuid.NewString()),
			inputType: reflect.TypeOf(ipc.ListActiveRecordingsRequest{}),
		},
		{
			input:     ipc.NewListActiveRecordingsResponse([]common.Recording{}),
			inputType: reflect.TypeOf(ipc.ListActiveRecordingsResponse{}),
		},
	}

	for idx, oneTest := range testCases {
		// Serialize
		asString, err := json.Marshal(oneTest.input)
		assert.Nil(err, "Failed in %d", idx)
		parsed, err := ipc.ParseRawMessage(asString)
		assert.Nil(err, "Failed in %d", idx)
		assert.NotNil(parsed)
		assert.Equalf(
			oneTest.inputType,
			reflect.TypeOf(parsed),
			"Expected type %s, Received type %s",
			oneTest.inputType,
			reflect.TypeOf(parsed),
		)
		assert.EqualValues(parsed, oneTest.input)
	}
}
