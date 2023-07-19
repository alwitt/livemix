package forwarder_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/forwarder"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestS3RecordingSegmentForwarder(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	mockS3 := mocks.NewSegmentSender(t)

	broadcasts := make(chan ipc.RecordingSegmentReport, 4)
	rxBroadcast := func(ctxt context.Context, report ipc.RecordingSegmentReport) error {
		broadcasts <- report
		return nil
	}

	storageCfg := common.RecordingStorageConfig{
		StorageBucket: uuid.NewString(), StorageObjectPrefix: uuid.NewString(),
	}

	uut, err := forwarder.NewS3RecordingSegmentForwarder(utCtxt, storageCfg, mockS3, rxBroadcast, 2)
	assert.Nil(err)

	// ------------------------------------------------------------------------------------
	// Case 0: upload segments

	{
		testSourceID := uuid.NewString()
		testRecordings := []string{
			uuid.NewString(), uuid.NewString(), uuid.NewString(),
		}
		testSegments := []common.VideoSegmentWithData{}
		testSegmentByID := map[string]common.VideoSegmentWithData{}
		for itr := 0; itr < 2; itr++ {
			segment := common.VideoSegmentWithData{
				VideoSegment: common.VideoSegment{
					ID:       uuid.NewString(),
					SourceID: testSourceID,
					Segment: hls.Segment{
						Name: fmt.Sprintf("%s.ts", uuid.NewString()),
					},
				},
				Content: []byte(uuid.NewString()),
			}
			testSegments = append(testSegments, segment)
			testSegmentByID[segment.ID] = segment
		}

		// Prepare mocks
		receivedSegment := map[string]bool{}
		mockS3.On(
			"ForwardSegment",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.AnythingOfType("common.VideoSegmentWithData"),
		).Run(func(args mock.Arguments) {
			segment, ok := args.Get(1).(common.VideoSegmentWithData)
			assert.True(ok)
			assert.Contains(testSegmentByID, segment.ID)
			expected := testSegmentByID[segment.ID]
			assert.Equal(expected.SourceID, segment.SourceID)
			assert.Equal(expected.Name, segment.Name)
			assert.EqualValues(expected.Content, segment.Content)
			receivedSegment[segment.ID] = true
		}).Return(nil).Times(len(testSegments))

		// Call
		assert.Nil(uut.ForwardSegment(utCtxt, testRecordings, testSegments))

		// Wait for response
		lclCtxt, lclCtxtCancel := context.WithCancel(utCtxt)
		defer lclCtxtCancel()

		receivedBroadcast := map[string]bool{}
		for itr := 0; itr < 2; itr++ {
			select {
			case <-lclCtxt.Done():
				assert.True(false, "timed out waiting for response")
			case msg, ok := <-broadcasts:
				assert.True(ok)
				assert.EqualValues(testRecordings, msg.RecordingIDs)
				assert.Len(msg.Segments, 1)
				assert.Contains(testSegmentByID, msg.Segments[0].ID)
				expected := testSegmentByID[msg.Segments[0].ID]
				gotten := msg.Segments[0]
				assert.Equal(expected.SourceID, gotten.SourceID)
				assert.Equal(expected.Name, gotten.Name)
				receivedBroadcast[gotten.ID] = true
			}
		}
		assert.Len(receivedBroadcast, len(testSegments))
		assert.Len(receivedSegment, len(testSegments))
	}

	// ------------------------------------------------------------------------------------
	// Case 0: upload segments, but had already been uploaded

	{
		testSourceID := uuid.NewString()
		testRecordings := []string{
			uuid.NewString(), uuid.NewString(), uuid.NewString(),
		}
		testSegments := []common.VideoSegmentWithData{}
		testSegmentByID := map[string]common.VideoSegmentWithData{}
		for itr := 0; itr < 2; itr++ {
			segment := common.VideoSegmentWithData{
				VideoSegment: common.VideoSegment{
					ID:       uuid.NewString(),
					SourceID: testSourceID,
					Segment: hls.Segment{
						Name: fmt.Sprintf("%s.ts", uuid.NewString()),
					},
				},
				Content: []byte(uuid.NewString()),
			}
			testSegments = append(testSegments, segment)
			testSegmentByID[segment.ID] = segment
		}
		uploaded := 1
		testSegments[0].Uploaded = &uploaded

		// Prepare mocks
		receivedSegment := map[string]bool{}
		mockS3.On(
			"ForwardSegment",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.AnythingOfType("common.VideoSegmentWithData"),
		).Run(func(args mock.Arguments) {
			segment, ok := args.Get(1).(common.VideoSegmentWithData)
			assert.True(ok)
			assert.Contains(testSegmentByID, segment.ID)
			assert.Equal(testSegments[1].ID, segment.ID)
			expected := testSegmentByID[segment.ID]
			assert.Equal(expected.SourceID, segment.SourceID)
			assert.Equal(expected.Name, segment.Name)
			assert.EqualValues(expected.Content, segment.Content)
			receivedSegment[segment.ID] = true
		}).Return(nil).Once()

		// Call
		assert.Nil(uut.ForwardSegment(utCtxt, testRecordings, testSegments))

		// Wait for response
		lclCtxt, lclCtxtCancel := context.WithCancel(utCtxt)
		defer lclCtxtCancel()

		receivedBroadcast := map[string]bool{}
		for itr := 0; itr < 2; itr++ {
			select {
			case <-lclCtxt.Done():
				assert.True(false, "timed out waiting for response")
			case msg, ok := <-broadcasts:
				assert.True(ok)
				assert.EqualValues(testRecordings, msg.RecordingIDs)
				assert.Len(msg.Segments, 1)
				assert.Contains(testSegmentByID, msg.Segments[0].ID)
				expected := testSegmentByID[msg.Segments[0].ID]
				gotten := msg.Segments[0]
				assert.Equal(expected.SourceID, gotten.SourceID)
				assert.Equal(expected.Name, gotten.Name)
				receivedBroadcast[gotten.ID] = true
			}
		}
		assert.Len(receivedBroadcast, len(testSegments))
		assert.Len(receivedSegment, 1)
	}

	assert.Nil(uut.Stop(utCtxt))
}
