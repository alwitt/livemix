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
	"github.com/alwitt/livemix/hls"
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

	mockS3 := mocks.NewS3Client(t)
	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()

	uut, err := control.NewManager(utCtxt, mockSQL, nil, mockS3, time.Minute, time.Hour, nil)
	assert.Nil(err)

	currentTime := time.Now().UTC()

	testMessage := ipc.NewVideoSourceStatusReport(
		uuid.NewString(), uuid.NewString(), currentTime,
	)

	broadcastMsg, err := json.Marshal(&testMessage)
	assert.Nil(err)

	// Setup mock
	mockDB.On(
		"UpdateVideoSourceStats",
		mock.AnythingOfType("context.backgroundCtx"),
		testMessage.SourceID,
		testMessage.RequestResponseTargetID,
		currentTime,
	).Return(nil).Once()

	// Process the broadcast message
	assert.Nil(uut.ProcessBroadcastMsgs(utCtxt, currentTime, broadcastMsg, nil))
}

func TestSystemManagerProcessNewRecordingSegmentsBroadcast(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockS3 := mocks.NewS3Client(t)
	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()

	uut, err := control.NewManager(utCtxt, mockSQL, nil, mockS3, time.Minute, time.Hour, nil)
	assert.Nil(err)

	currentTime := time.Now().UTC()

	testMessage := ipc.NewRecordingSegmentReport(
		[]string{uuid.NewString(), uuid.NewString(), uuid.NewString()},
		[]common.VideoSegment{
			{
				ID:       uuid.NewString(),
				SourceID: uuid.NewString(),
				Segment:  hls.Segment{Name: uuid.NewString()},
			},
		},
	)

	broadcastMsg, err := json.Marshal(&testMessage)
	assert.Nil(err)

	// Setup mock
	mockDB.On(
		"RegisterRecordingSegments",
		mock.AnythingOfType("context.backgroundCtx"),
		testMessage.RecordingIDs,
		testMessage.Segments,
	).Return(nil).Once()

	// Process the broadcast message
	assert.Nil(uut.ProcessBroadcastMsgs(utCtxt, currentTime, broadcastMsg, nil))
}

func TestSystemManagerRequestStreamingStateChange(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockS3 := mocks.NewS3Client(t)
	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockRR := mocks.NewEdgeRequestClient(t)

	currentTime := time.Now().UTC()

	uut, err := control.NewManager(utCtxt, mockSQL, mockRR, mockS3, time.Minute, time.Hour, nil)
	assert.Nil(err)

	// Case 0: video has not specified target RR ID
	{
		testSource := common.VideoSource{ID: uuid.NewString()}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
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
			mock.AnythingOfType("context.backgroundCtx"),
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
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"ChangeVideoSourceStreamState",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
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
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockRR.On(
			"ChangeVideoStreamingState",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource,
			1,
		).Return(nil).Once()
		mockDB.On(
			"ChangeVideoSourceStreamState",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
			1,
		).Return(nil).Once()

		assert.Nil(uut.ChangeVideoSourceStreamState(utCtxt, testSource.ID, 1))
	}
}

func TestSystemManagerDefineRecordingSession(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockS3 := mocks.NewS3Client(t)
	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockRR := mocks.NewEdgeRequestClient(t)

	currentTime := time.Now().UTC()

	uut, err := control.NewManager(utCtxt, mockSQL, mockRR, mockS3, time.Minute, time.Hour, nil)
	assert.Nil(err)

	// Case 0: unknown video source
	{
		testSourceID := uuid.NewString()

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSourceID,
		).Return(common.VideoSource{}, fmt.Errorf("dummy error")).Once()

		_, err := uut.DefineRecordingSession(utCtxt, testSourceID, nil, nil, currentTime)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 1: known video source, but failed to define recording session
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"DefineRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*string"),
			currentTime,
		).Return(uuid.NewString(), fmt.Errorf("dummy error")).Once()

		_, err := uut.DefineRecordingSession(utCtxt, testSource.ID, nil, nil, currentTime)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 2: defined recording session, but failed to make the request
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSource.ID}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"DefineRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*string"),
			currentTime,
		).Return(testRecording.ID, nil).Once()
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockRR.On(
			"StartRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource,
			testRecording,
		).Return(fmt.Errorf("dummy error")).Once()
		mockDB.On(
			"MarkExternalError",
			fmt.Errorf("dummy error"),
		).Return().Once()

		_, err := uut.DefineRecordingSession(utCtxt, testSource.ID, nil, nil, currentTime)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 3: complete call
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSource.ID}

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"DefineRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*string"),
			currentTime,
		).Return(testRecording.ID, nil).Once()
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockRR.On(
			"StartRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource,
			testRecording,
		).Return(nil).Once()

		_, err := uut.DefineRecordingSession(utCtxt, testSource.ID, nil, nil, currentTime)
		assert.Nil(err)
	}
}

func TestSystemManagerMarkEndOfRecordingSession(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockS3 := mocks.NewS3Client(t)
	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockRR := mocks.NewEdgeRequestClient(t)

	currentTime := time.Now().UTC()

	uut, err := control.NewManager(utCtxt, mockSQL, mockRR, mockS3, time.Minute, time.Hour, nil)
	assert.Nil(err)

	// Case 0: unknown recording
	{
		testRecordingID := uuid.NewString()

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecordingID,
		).Return(common.Recording{}, fmt.Errorf("dummy error")).Once()

		err := uut.MarkEndOfRecordingSession(utCtxt, testRecordingID, currentTime, false)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 1: known recording, but failed to find video source
	{
		testSourceID := uuid.NewString()
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSourceID}

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSourceID,
		).Return(common.VideoSource{}, fmt.Errorf("dummy error")).Once()

		err := uut.MarkEndOfRecordingSession(utCtxt, testRecording.ID, currentTime, false)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 2: good recording and source, but failed to end recording session
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSource.ID, Active: 1}

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
			currentTime,
		).Return(fmt.Errorf("dummy error")).Once()

		err := uut.MarkEndOfRecordingSession(utCtxt, testRecording.ID, currentTime, false)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 3: DB operations passed, but RR failed
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSource.ID, Active: 1}

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
			currentTime,
		).Return(nil).Once()
		mockRR.On(
			"StopRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource,
			testRecording.ID,
			currentTime,
		).Return(fmt.Errorf("dummy error")).Once()
		mockDB.On(
			"MarkExternalError",
			fmt.Errorf("dummy error"),
		).Return().Once()

		err := uut.MarkEndOfRecordingSession(utCtxt, testRecording.ID, currentTime, false)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 4: complete call
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSource.ID, Active: 1}

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
			currentTime,
		).Return(nil).Once()
		mockRR.On(
			"StopRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource,
			testRecording.ID,
			currentTime,
		).Return(nil).Once()

		err := uut.MarkEndOfRecordingSession(utCtxt, testRecording.ID, currentTime, false)
		assert.Nil(err)
	}

	// Case 5: DB operation passed, RR failed, but the request forced through
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSource.ID, Active: 1}

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
			currentTime,
		).Return(nil).Once()
		mockRR.On(
			"StopRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource,
			testRecording.ID,
			currentTime,
		).Return(fmt.Errorf("dummy error")).Once()

		err := uut.MarkEndOfRecordingSession(utCtxt, testRecording.ID, currentTime, true)
		assert.Nil(err)
	}

	// Case 6: DB operation passed, recording already complete, so no changes
	{
		testRRTargetID := uuid.NewString()
		testSource := common.VideoSource{
			ID: uuid.NewString(), ReqRespTargetID: &testRRTargetID, SourceLocalTime: currentTime,
		}
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSource.ID, Active: -1}

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()

		err := uut.MarkEndOfRecordingSession(utCtxt, testRecording.ID, currentTime, true)
		assert.Nil(err)
	}
}

func TestSystemManagerStopAllActiveRecordingsOfSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockS3 := mocks.NewS3Client(t)
	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockRR := mocks.NewEdgeRequestClient(t)

	currentTime := time.Now().UTC()

	uut, err := control.NewManager(utCtxt, mockSQL, mockRR, mockS3, time.Minute, time.Hour, nil)
	assert.Nil(err)

	// Case 0: unknown video source
	{
		testSourceID := uuid.NewString()

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("context.backgroundCtx"),
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
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("context.backgroundCtx"),
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
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("context.backgroundCtx"),
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
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource.ID,
			true,
		).Return(testSessions, nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSessions[0].ID,
			currentTime,
		).Return(nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSessions[1].ID,
			currentTime,
		).Return(nil).Once()
		mockRR.On(
			"StopRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource,
			testSessions[0].ID,
			currentTime,
		).Return(nil).Once()
		mockRR.On(
			"StopRecordingSession",
			mock.AnythingOfType("context.backgroundCtx"),
			testSource,
			testSessions[1].ID,
			currentTime,
		).Return(fmt.Errorf("dummy error")).Once()

		err := uut.StopAllActiveRecordingOfSource(utCtxt, testSource.ID, currentTime)
		assert.Nil(err)
	}
}

func TestSystemManagerPurgeUnassociatedRecordingSegments(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockS3 := mocks.NewS3Client(t)
	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()

	uut, err := control.NewManager(utCtxt, mockSQL, nil, mockS3, time.Minute, time.Hour, nil)
	assert.Nil(err)

	// ------------------------------------------------------------------------------------
	// Case 0: there were no segments to purge

	{
		// Prepare mock
		mockDB.On(
			"DeleteUnassociatedRecordingSegments",
			mock.AnythingOfType("*context.cancelCtx"),
		).Return([]common.VideoSegment{}, nil).Once()

		assert.Nil(uut.DeleteUnassociatedRecordingSegments())
	}

	// ------------------------------------------------------------------------------------
	// Case 1: return segments

	{
		testSegments := []common.VideoSegment{}
		// Test segments from two different buckets
		testBucket0 := uuid.NewString()
		testBucket1 := uuid.NewString()
		objectsByBucket := map[string][]string{
			testBucket0: {},
			testBucket1: {},
		}
		for itr := 0; itr < 3; itr++ {
			segmentName := fmt.Sprintf("%s.ts", uuid.NewString())
			testSegments = append(testSegments, common.VideoSegment{
				ID: uuid.NewString(),
				Segment: hls.Segment{
					Name: segmentName,
					URI:  fmt.Sprintf("s3://%s/%s", testBucket0, segmentName),
				},
			})
			objectsByBucket[testBucket0] = append(objectsByBucket[testBucket0], segmentName)
		}
		for itr := 0; itr < 3; itr++ {
			segmentName := fmt.Sprintf("%s.ts", uuid.NewString())
			testSegments = append(testSegments, common.VideoSegment{
				ID: uuid.NewString(),
				Segment: hls.Segment{
					Name: segmentName,
					URI:  fmt.Sprintf("s3://%s/%s", testBucket1, segmentName),
				},
			})
			objectsByBucket[testBucket1] = append(objectsByBucket[testBucket1], segmentName)
		}

		// Prepare mock
		mockDB.On(
			"DeleteUnassociatedRecordingSegments",
			mock.AnythingOfType("*context.cancelCtx"),
		).Return(testSegments, nil).Once()
		mockS3.On(
			"DeleteObjects",
			mock.AnythingOfType("*context.cancelCtx"),
			testBucket0,
			objectsByBucket[testBucket0],
		).Return(nil).Once()
		mockS3.On(
			"DeleteObjects",
			mock.AnythingOfType("*context.cancelCtx"),
			testBucket1,
			objectsByBucket[testBucket1],
		).Return(nil).Once()

		assert.Nil(uut.DeleteUnassociatedRecordingSegments())
	}
}
