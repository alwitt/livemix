package control_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/control"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSystemManagerProcessSourceStatusBroadcast(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockDB := mocks.NewPersistenceManager(t)

	uut, err := control.NewManager(mockDB, nil, time.Minute)
	assert.Nil(err)

	currentTime := time.Now().UTC()

	testMessage := ipc.NewVideoSourceStatusReport(
		uuid.NewString(), uuid.NewString(), currentTime,
	)

	broadcastMsg, err := json.Marshal(&testMessage)
	assert.Nil(err)

	// Setup mock
	mockDB.On(
		"RefreshVideoSourceStats",
		mock.AnythingOfType("*context.emptyCtx"),
		testMessage.SourceID,
		testMessage.RequestResponseTargetID,
		currentTime,
	).Return(nil).Once()

	// Process the broadcast message
	assert.Nil(uut.ProcessBroadcastMsgs(utCtxt, currentTime, broadcastMsg, nil))
}

func TestSystemManagerRequestStreamingStateChange(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockDB := mocks.NewPersistenceManager(t)
	mockRR := mocks.NewEdgeRequestClient(t)

	currentTime := time.Now().UTC()

	uut, err := control.NewManager(mockDB, mockRR, time.Minute)
	assert.Nil(err)

	// Case 0: video has not specified target RR ID
	{
		testSource := common.VideoSource{ID: uuid.NewString()}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()

		assert.NotNil(uut.ChangeVideoSourceStreamState(utCtxt, testSource.ID, 1))
	}

	// Case 1: video has not sent a status report recently
	{
		rrTarget := uuid.NewString()
		testSource := common.VideoSource{
			ID:              uuid.NewString(),
			ReqRespTargetID: &rrTarget,
			SourceLocalTime: currentTime.Add(time.Minute * -2),
		}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()

		assert.NotNil(uut.ChangeVideoSourceStreamState(utCtxt, testSource.ID, 1))
	}

	// Case 2: state change request change failed
	{
		rrTarget := uuid.NewString()
		testSource := common.VideoSource{
			ID:              uuid.NewString(),
			ReqRespTargetID: &rrTarget,
			SourceLocalTime: currentTime,
		}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockRR.On(
			"ChangeVideoStreamingState",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource,
			1,
		).Return(fmt.Errorf("dummy error")).Once()

		assert.NotNil(uut.ChangeVideoSourceStreamState(utCtxt, testSource.ID, 1))
	}

	// Case 3: successful state change
	{
		rrTarget := uuid.NewString()
		testSource := common.VideoSource{
			ID:              uuid.NewString(),
			ReqRespTargetID: &rrTarget,
			SourceLocalTime: currentTime,
		}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockRR.On(
			"ChangeVideoStreamingState",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource,
			1,
		).Return(nil).Once()
		mockDB.On(
			"ChangeVideoSourceStreamState",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
			1,
		).Return(nil).Once()

		assert.Nil(uut.ChangeVideoSourceStreamState(utCtxt, testSource.ID, 1))
	}
}

func TestSystemManagerStopAllActiveRecordingsOfSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockDB := mocks.NewPersistenceManager(t)
	mockRR := mocks.NewEdgeRequestClient(t)

	currentTime := time.Now().UTC()

	uut, err := control.NewManager(mockDB, mockRR, time.Minute)
	assert.Nil(err)

	// Case 0: unknown video source
	{
		testSourceID := uuid.NewString()

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSourceID,
		).Return(common.VideoSource{}, fmt.Errorf("dummy error")).Once()

		err := uut.StopAllActiveRecordingOfSource(utCtxt, testSourceID, currentTime)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 1: known video source, but failed to read sessions
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
			true,
		).Return(nil, fmt.Errorf("dummy error")).Once()

		err := uut.StopAllActiveRecordingOfSource(utCtxt, testSource.ID, currentTime)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 2: known video source, but no active sessions
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
			true,
		).Return([]common.Recording{}, nil).Once()

		err := uut.StopAllActiveRecordingOfSource(utCtxt, testSource.ID, currentTime)
		assert.Nil(err)
	}

	// Case 3: normal operations, two active session, one failed on RPC call
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}
		testSessions := []common.Recording{{ID: uuid.NewString()}, {ID: uuid.NewString()}}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
			true,
		).Return(testSessions, nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("*context.emptyCtx"),
			testSessions[0].ID,
			currentTime,
		).Return(nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("*context.emptyCtx"),
			testSessions[1].ID,
			currentTime,
		).Return(nil).Once()
		mockRR.On(
			"StopRecordingSession",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource,
			testSessions[0].ID,
			currentTime,
		).Return(nil).Once()
		mockRR.On(
			"StopRecordingSession",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource,
			testSessions[1].ID,
			currentTime,
		).Return(fmt.Errorf("dummy error")).Once()

		err := uut.StopAllActiveRecordingOfSource(utCtxt, testSource.ID, currentTime)
		assert.Nil(err)
	}
}
