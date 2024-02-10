package edge_test

import (
	"context"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/edge"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestVideoSourceOperatorStartRecording(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockCache := mocks.NewVideoSegmentCache(t)
	mockBroadcast := mocks.NewBroadcaster(t)
	mockRecordForwarder := mocks.NewRecordingSegmentForwarder(t)
	mockLiveForwarder := mocks.NewLiveStreamSegmentForwarder(t)
	mockRR := mocks.NewControlRequestClient(t)

	testSource := common.VideoSource{ID: uuid.NewString()}

	uutConfig := edge.VideoSourceOperatorConfig{
		Self:                       testSource,
		SelfReqRespTargetID:        uuid.NewString(),
		DBConns:                    mockSQL,
		VideoCache:                 mockCache,
		BroadcastClient:            mockBroadcast,
		RecordingSegmentForwarder:  mockRecordForwarder,
		LiveStreamSegmentForwarder: mockLiveForwarder,
		StatusReportInterval:       time.Minute * 5,
	}

	// ====================================================================================
	// Prepare mock for initialization

	mockBroadcast.On(
		"Broadcast",
		mock.AnythingOfType("*context.cancelCtx"),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		report, ok := args.Get(1).(*ipc.VideoSourceStatusReport)
		assert.True(ok)

		assert.Equal(testSource.ID, report.SourceID)
		assert.Equal(uutConfig.SelfReqRespTargetID, report.RequestResponseTargetID)
	}).Return(nil)
	mockDB.On(
		"UpdateVideoSourceStats",
		mock.AnythingOfType("*context.cancelCtx"),
		testSource.ID,
		uutConfig.SelfReqRespTargetID,
		mock.AnythingOfType("time.Time"),
	).Return(nil)

	uut, err := edge.NewManager(utCtxt, uutConfig, mockRR, nil)
	assert.Nil(err)

	// ====================================================================================
	// Case 0: new recording, but no existing segments

	timestamp := time.Now().UTC()

	{
		complete := make(chan bool, 1)
		testRecording := common.Recording{
			ID:        uuid.NewString(),
			SourceID:  testSource.ID,
			StartTime: timestamp,
		}

		// Prepare mock
		mockDB.On(
			"RecordKnownRecordingSession",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.AnythingOfType("common.Recording"),
		).Run(func(args mock.Arguments) {
			recording, ok := args.Get(1).(common.Recording)
			assert.True(ok)
			assert.Equal(testRecording.ID, recording.ID)
			assert.Equal(testRecording.SourceID, recording.SourceID)
			assert.Equal(testRecording.StartTime.Unix(), recording.StartTime.Unix())
		}).Return(nil).Once()
		mockDB.On(
			"ListAllLiveStreamSegmentsAfterTime",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
			mock.AnythingOfType("time.Time"),
		).Run(func(args mock.Arguments) {
			complete <- true
		}).Return([]common.VideoSegment{}, nil).Once()

		lclCtxt, lclCtxtCancel := context.WithCancel(utCtxt)
		defer lclCtxtCancel()
		// Make request
		assert.Nil(uut.StartRecording(lclCtxt, testRecording))
		select {
		case <-lclCtxt.Done():
			assert.True(false, "request timed out")
		case <-complete:
			break
		}
	}

	// ====================================================================================
	// Case 1: new recording, have existing segments

	{
		complete := make(chan bool, 1)
		testRecording := common.Recording{
			ID:        uuid.NewString(),
			SourceID:  testSource.ID,
			StartTime: timestamp,
		}
		testSegments := []common.VideoSegment{
			{ID: uuid.NewString()}, {ID: uuid.NewString()}, {ID: uuid.NewString()},
		}
		testSegmentIDs := []string{}
		testSegmentContents := map[string][]byte{}
		for _, segment := range testSegments {
			testSegmentIDs = append(testSegmentIDs, segment.ID)
			testSegmentContents[segment.ID] = []byte(uuid.NewString())
		}

		// Prepare mock
		mockDB.On(
			"RecordKnownRecordingSession",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.AnythingOfType("common.Recording"),
		).Run(func(args mock.Arguments) {
			recording, ok := args.Get(1).(common.Recording)
			assert.True(ok)
			assert.Equal(testRecording.ID, recording.ID)
			assert.Equal(testRecording.SourceID, recording.SourceID)
			assert.Equal(testRecording.StartTime.Unix(), recording.StartTime.Unix())
		}).Return(nil).Once()
		mockDB.On(
			"ListAllLiveStreamSegmentsAfterTime",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
			mock.AnythingOfType("time.Time"),
		).Return(testSegments, nil).Once()
		mockCache.On(
			"GetSegments",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.AnythingOfType("[]common.VideoSegment"),
		).Run(func(args mock.Arguments) {
			querySegments := args.Get(1).([]common.VideoSegment)
			for idx, oneSegment := range querySegments {
				assert.Equal(testSegmentIDs[idx], oneSegment.ID)
			}
		}).Return(testSegmentContents, nil).Once()
		mockRecordForwarder.On(
			"ForwardSegment",
			mock.AnythingOfType("*context.cancelCtx"),
			[]string{testRecording.ID},
			mock.AnythingOfType("[]common.VideoSegmentWithData"),
		).Run(func(args mock.Arguments) {
			segments, ok := args.Get(2).([]common.VideoSegmentWithData)
			assert.True(ok)
			assert.Len(segments, len(testSegments))
			for idx, outSegment := range segments {
				testSegment := testSegments[idx]
				assert.Equal(testSegment.ID, outSegment.ID)
				assert.Equal(testSegmentContents[testSegment.ID], outSegment.Content)
			}
			complete <- true
		}).Return(nil).Once()

		lclCtxt, lclCtxtCancel := context.WithCancel(utCtxt)
		defer lclCtxtCancel()
		// Make request
		assert.Nil(uut.StartRecording(lclCtxt, testRecording))
		select {
		case <-lclCtxt.Done():
			assert.True(false, "request timed out")
		case <-complete:
			break
		}
	}

	// ====================================================================================
	// Cleanup
	mockRecordForwarder.On(
		"Stop",
		mock.AnythingOfType("*context.emptyCtx"),
	).Return(nil).Once()
	mockLiveForwarder.On(
		"Stop",
		mock.AnythingOfType("*context.emptyCtx"),
	).Return(nil).Once()
	assert.Nil(uut.Stop(utCtxt))
}

func TestVideoSourceOperatorStopRecording(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockCache := mocks.NewVideoSegmentCache(t)
	mockBroadcast := mocks.NewBroadcaster(t)
	mockRecordForwarder := mocks.NewRecordingSegmentForwarder(t)
	mockLiveForwarder := mocks.NewLiveStreamSegmentForwarder(t)
	mockRR := mocks.NewControlRequestClient(t)

	testSource := common.VideoSource{ID: uuid.NewString()}

	uutConfig := edge.VideoSourceOperatorConfig{
		Self:                       testSource,
		SelfReqRespTargetID:        uuid.NewString(),
		DBConns:                    mockSQL,
		VideoCache:                 mockCache,
		BroadcastClient:            mockBroadcast,
		RecordingSegmentForwarder:  mockRecordForwarder,
		LiveStreamSegmentForwarder: mockLiveForwarder,
		StatusReportInterval:       time.Minute * 5,
	}

	// ====================================================================================
	// Prepare mock for initialization

	mockBroadcast.On(
		"Broadcast",
		mock.AnythingOfType("*context.cancelCtx"),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		report, ok := args.Get(1).(*ipc.VideoSourceStatusReport)
		assert.True(ok)

		assert.Equal(testSource.ID, report.SourceID)
		assert.Equal(uutConfig.SelfReqRespTargetID, report.RequestResponseTargetID)
	}).Return(nil)
	mockDB.On(
		"UpdateVideoSourceStats",
		mock.AnythingOfType("*context.cancelCtx"),
		testSource.ID,
		uutConfig.SelfReqRespTargetID,
		mock.AnythingOfType("time.Time"),
	).Return(nil)

	uut, err := edge.NewManager(utCtxt, uutConfig, mockRR, nil)
	assert.Nil(err)

	// ====================================================================================
	// Stop a recording

	{
		complete := make(chan bool, 1)
		testRecordingID := uuid.NewString()
		endTime := time.Now().UTC()

		// Prepare mock
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("*context.cancelCtx"),
			testRecordingID,
			endTime,
		).Run(func(args mock.Arguments) {
			complete <- true
		}).Return(nil).Once()

		lclCtxt, lclCtxtCancel := context.WithCancel(utCtxt)
		defer lclCtxtCancel()
		// Make request
		assert.Nil(uut.StopRecording(lclCtxt, testRecordingID, endTime))
		select {
		case <-lclCtxt.Done():
			assert.True(false, "request timed out")
		case <-complete:
			break
		}
	}

	// ====================================================================================
	// Cleanup
	mockRecordForwarder.On(
		"Stop",
		mock.AnythingOfType("*context.emptyCtx"),
	).Return(nil).Once()
	mockLiveForwarder.On(
		"Stop",
		mock.AnythingOfType("*context.emptyCtx"),
	).Return(nil).Once()
	assert.Nil(uut.Stop(utCtxt))
}

func TestVideoSourceOperatorNewSegmentFromSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockCache := mocks.NewVideoSegmentCache(t)
	mockBroadcast := mocks.NewBroadcaster(t)
	mockRecordForwarder := mocks.NewRecordingSegmentForwarder(t)
	mockLiveForwarder := mocks.NewLiveStreamSegmentForwarder(t)
	mockRR := mocks.NewControlRequestClient(t)

	testSource := common.VideoSource{ID: uuid.NewString()}

	uutConfig := edge.VideoSourceOperatorConfig{
		Self:                       testSource,
		SelfReqRespTargetID:        uuid.NewString(),
		DBConns:                    mockSQL,
		VideoCache:                 mockCache,
		BroadcastClient:            mockBroadcast,
		RecordingSegmentForwarder:  mockRecordForwarder,
		LiveStreamSegmentForwarder: mockLiveForwarder,
		StatusReportInterval:       time.Minute * 5,
	}

	// ====================================================================================
	// Prepare mock for initialization

	mockBroadcast.On(
		"Broadcast",
		mock.AnythingOfType("*context.cancelCtx"),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		report, ok := args.Get(1).(*ipc.VideoSourceStatusReport)
		assert.True(ok)

		assert.Equal(testSource.ID, report.SourceID)
		assert.Equal(uutConfig.SelfReqRespTargetID, report.RequestResponseTargetID)
	}).Return(nil)
	mockDB.On(
		"UpdateVideoSourceStats",
		mock.AnythingOfType("*context.cancelCtx"),
		testSource.ID,
		uutConfig.SelfReqRespTargetID,
		mock.AnythingOfType("time.Time"),
	).Return(nil)

	uut, err := edge.NewManager(utCtxt, uutConfig, mockRR, nil)
	assert.Nil(err)

	// ====================================================================================
	// Case 0: forwarded to live recording, but there no recordings

	{
		complete := make(chan bool, 1)
		testSegment := common.VideoSegmentWithData{
			VideoSegment: common.VideoSegment{
				ID:       uuid.NewString(),
				SourceID: testSource.ID,
				Segment:  hls.Segment{Name: uuid.NewString()},
			},
			Content: []byte(uuid.NewString()),
		}

		// Prepare mock
		mockLiveForwarder.On(
			"ForwardSegment",
			mock.AnythingOfType("*context.cancelCtx"),
			testSegment,
			false,
		).Return(nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
			true,
		).Run(func(args mock.Arguments) {
			complete <- true
		}).Return([]common.Recording{}, nil).Once()

		lclCtxt, lclCtxtCancel := context.WithCancel(utCtxt)
		defer lclCtxtCancel()
		// Make request
		assert.Nil(uut.NewSegmentFromSource(lclCtxt, testSegment))
		select {
		case <-lclCtxt.Done():
			assert.True(false, "request timed out")
		case <-complete:
			break
		}
	}

	// ====================================================================================
	// Case 1: forwarded to live recording, have ongoing recordings

	{
		complete := make(chan bool, 1)
		testSegment := common.VideoSegmentWithData{
			VideoSegment: common.VideoSegment{
				ID:       uuid.NewString(),
				SourceID: testSource.ID,
				Segment:  hls.Segment{Name: uuid.NewString()},
			},
			Content: []byte(uuid.NewString()),
		}
		testRecording := []common.Recording{{ID: uuid.NewString()}}
		testRecordIDs := []string{}
		for _, recording := range testRecording {
			testRecordIDs = append(testRecordIDs, recording.ID)
		}

		// Prepare mock
		mockLiveForwarder.On(
			"ForwardSegment",
			mock.AnythingOfType("*context.cancelCtx"),
			testSegment,
			false,
		).Return(nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
			true,
		).Return(testRecording, nil).Once()
		mockRecordForwarder.On(
			"ForwardSegment",
			mock.AnythingOfType("*context.cancelCtx"),
			testRecordIDs,
			[]common.VideoSegmentWithData{testSegment},
		).Run(func(args mock.Arguments) {
			complete <- true
		}).Return(nil).Once()

		lclCtxt, lclCtxtCancel := context.WithCancel(utCtxt)
		defer lclCtxtCancel()
		// Make request
		assert.Nil(uut.NewSegmentFromSource(lclCtxt, testSegment))
		select {
		case <-lclCtxt.Done():
			assert.True(false, "request timed out")
		case <-complete:
			break
		}
	}

	// ====================================================================================
	// Cleanup
	mockRecordForwarder.On(
		"Stop",
		mock.AnythingOfType("*context.emptyCtx"),
	).Return(nil).Once()
	mockLiveForwarder.On(
		"Stop",
		mock.AnythingOfType("*context.emptyCtx"),
	).Return(nil).Once()
	assert.Nil(uut.Stop(utCtxt))
}

func TestVideoSourceOperatorSyncActiveRecordingState(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockCache := mocks.NewVideoSegmentCache(t)
	mockBroadcast := mocks.NewBroadcaster(t)
	mockRecordForwarder := mocks.NewRecordingSegmentForwarder(t)
	mockLiveForwarder := mocks.NewLiveStreamSegmentForwarder(t)
	mockRR := mocks.NewControlRequestClient(t)

	mockDB.On(
		"ListAllLiveStreamSegmentsAfterTime",
		mock.AnythingOfType("*context.cancelCtx"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Time"),
	).Return([]common.VideoSegment{}, nil).Maybe()

	testSource := common.VideoSource{ID: uuid.NewString()}

	uutConfig := edge.VideoSourceOperatorConfig{
		Self:                       testSource,
		SelfReqRespTargetID:        uuid.NewString(),
		DBConns:                    mockSQL,
		VideoCache:                 mockCache,
		BroadcastClient:            mockBroadcast,
		RecordingSegmentForwarder:  mockRecordForwarder,
		LiveStreamSegmentForwarder: mockLiveForwarder,
		StatusReportInterval:       time.Minute * 5,
	}

	// ====================================================================================
	// Prepare mock for initialization

	mockBroadcast.On(
		"Broadcast",
		mock.AnythingOfType("*context.cancelCtx"),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		report, ok := args.Get(1).(*ipc.VideoSourceStatusReport)
		assert.True(ok)

		assert.Equal(testSource.ID, report.SourceID)
		assert.Equal(uutConfig.SelfReqRespTargetID, report.RequestResponseTargetID)
	}).Return(nil)
	mockDB.On(
		"UpdateVideoSourceStats",
		mock.AnythingOfType("*context.cancelCtx"),
		testSource.ID,
		uutConfig.SelfReqRespTargetID,
		mock.AnythingOfType("time.Time"),
	).Return(nil)

	uut, err := edge.NewManager(utCtxt, uutConfig, mockRR, nil)
	assert.Nil(err)

	timestamp := time.Now().UTC()

	// ====================================================================================
	// Case 0: no recording at local or control node

	{
		mockRR.On(
			"ListActiveRecordingsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
		).Return([]common.Recording{}, nil).Once()

		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
			true,
		).Return([]common.Recording{}, nil).Once()

		assert.Nil(uut.SyncActiveRecordingState(timestamp))
	}

	// ====================================================================================
	// Case 1: no local recordings, control node has recordings

	{
		complete := make(chan bool, 1)

		remoteRecords := []common.Recording{{ID: ulid.Make().String()}}
		mockRR.On(
			"ListActiveRecordingsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
		).Return(remoteRecords, nil).Once()

		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
			true,
		).Return([]common.Recording{}, nil).Once()
		mockDB.On(
			"RecordKnownRecordingSession",
			mock.AnythingOfType("*context.cancelCtx"),
			remoteRecords[0],
		).Run(func(args mock.Arguments) {
			complete <- true
		}).Return(nil).Once()

		assert.Nil(uut.SyncActiveRecordingState(timestamp))

		{
			lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*100)
			select {
			case <-lclCtxt.Done():
				assert.False(true, "request timed out")
			case <-complete:
				break
			}
			lclCancel()
		}
	}

	// ====================================================================================
	// Case 2: local recordings, control node no recordings

	{
		mockRR.On(
			"ListActiveRecordingsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
		).Return([]common.Recording{}, nil).Once()

		localRecords := []common.Recording{{ID: ulid.Make().String()}}
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
			true,
		).Return(localRecords, nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("*context.cancelCtx"),
			localRecords[0].ID,
			timestamp,
		).Return(nil).Once()

		assert.Nil(uut.SyncActiveRecordingState(timestamp))
	}

	// ====================================================================================
	// Case 3: local recording matches control node recordings

	{
		remoteRecords := []common.Recording{{ID: ulid.Make().String()}}

		mockRR.On(
			"ListActiveRecordingsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
		).Return(remoteRecords, nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
			true,
		).Return(remoteRecords, nil).Once()

		assert.Nil(uut.SyncActiveRecordingState(timestamp))
	}

	// ====================================================================================
	// Case 4: local recording does not match control node recordings

	{
		complete := make(chan bool, 1)

		remoteRecords := []common.Recording{{ID: ulid.Make().String()}}
		localRecords := []common.Recording{{ID: ulid.Make().String()}}

		mockRR.On(
			"ListActiveRecordingsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
		).Return(remoteRecords, nil).Once()
		mockDB.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
			true,
		).Return(localRecords, nil).Once()
		mockDB.On(
			"MarkEndOfRecordingSession",
			mock.AnythingOfType("*context.cancelCtx"),
			localRecords[0].ID,
			timestamp,
		).Return(nil).Once()
		mockDB.On(
			"RecordKnownRecordingSession",
			mock.AnythingOfType("*context.cancelCtx"),
			remoteRecords[0],
		).Run(func(args mock.Arguments) {
			complete <- true
		}).Return(nil).Once()

		assert.Nil(uut.SyncActiveRecordingState(timestamp))

		{
			lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*100)
			select {
			case <-lclCtxt.Done():
				assert.False(true, "request timed out")
			case <-complete:
				break
			}
			lclCancel()
		}
	}
}
