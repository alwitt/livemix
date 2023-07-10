package forwarder

import (
	"context"
	"testing"

	"github.com/alwitt/livemix/common"
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

	mockDB := mocks.NewPersistenceManager(t)
	mockSender := mocks.NewSegmentSender(t)

	uut, err := NewHTTPLiveStreamSegmentForwarder(utCtxt, mockDB, mockSender, 2)
	assert.Nil(err)

	uutCast, ok := uut.(*httpLiveStreamSegmentForwarder)
	assert.True(ok)

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

		assert.Nil(uutCast.handleForwardSegment(newSegment))
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
			testSource.ID,
			mock.AnythingOfType("hls.Segment"),
			newSegment.Content,
		).Run(func(args mock.Arguments) {
			segment, ok := args.Get(2).(hls.Segment)
			assert.True(ok)
			assert.Equal(newSegment.Name, segment.Name)
		}).Return(nil).Once()

		assert.Nil(uutCast.handleForwardSegment(newSegment))
	}

	// Clean up
	assert.Nil(uut.Stop(utCtxt))
}
