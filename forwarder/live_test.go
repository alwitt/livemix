package forwarder_test

import (
	"context"
	"testing"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/forwarder"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHTTPLiveStreamForwarder(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockSender := mocks.NewSegmentSender(t)

	uut, err := forwarder.NewHTTPLiveStreamSegmentForwarder(utCtxt, mockSQL, mockSender, 2)
	assert.Nil(err)

	// ------------------------------------------------------------------------------------
	// Case 0: source not streaming

	{
		testSource := common.VideoSource{ID: uuid.NewString(), Streaming: -1}
		newSegment := common.VideoSegmentWithData{
			VideoSegment: common.VideoSegment{
				SourceID: testSource.ID,
				Segment:  hls.Segment{Name: uuid.NewString()},
			},
			Content: []byte(uuid.NewString()),
		}

		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()

		assert.Nil(uut.ForwardSegment(utCtxt, newSegment, true))
	}

	// ------------------------------------------------------------------------------------
	// Case 1: source is streaming

	{
		testSource := common.VideoSource{ID: uuid.NewString(), Streaming: 1}
		newSegment := common.VideoSegmentWithData{
			VideoSegment: common.VideoSegment{
				SourceID: testSource.ID,
				Segment:  hls.Segment{Name: uuid.NewString()},
			},
			Content: []byte(uuid.NewString()),
		}

		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.cancelCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockSender.On(
			"ForwardSegment",
			mock.AnythingOfType("*context.cancelCtx"),
			mock.AnythingOfType("common.VideoSegmentWithData"),
		).Run(func(args mock.Arguments) {
			segment, ok := args.Get(1).(common.VideoSegmentWithData)
			assert.True(ok)
			assert.Equal(testSource.ID, segment.SourceID)
			assert.Equal(newSegment.Name, segment.Name)
			assert.Equal(newSegment.Content, segment.Content)
		}).Return(nil).Once()

		assert.Nil(uut.ForwardSegment(utCtxt, newSegment, true))
	}

	// Clean up
	assert.Nil(uut.Stop(utCtxt))
}
