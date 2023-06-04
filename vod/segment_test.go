package vod_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/alwitt/livemix/utils"
	"github.com/alwitt/livemix/vod"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSegmentManager(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	mockCache := mocks.NewVideoSegmentCache(t)
	mockReader := mocks.NewSegmentReader(t)

	segmentTTL := time.Second * 4413

	// Case 0: no segment reader provided
	{
		uut, err := vod.NewSegmentManager(mockCache, nil, segmentTTL)
		assert.Nil(err)

		segmentID := uuid.NewString()

		// Prepare mock
		mockCache.On(
			"GetSegment",
			mock.AnythingOfType("*context.emptyCtx"),
			segmentID,
		).Return(nil, fmt.Errorf("dummy error")).Once()

		_, err = uut.GetSegment(utCtxt, common.VideoSegment{
			ID: segmentID,
		})
		assert.NotNil(err)
	}
	{
		uut, err := vod.NewSegmentManager(mockCache, nil, segmentTTL)
		assert.Nil(err)

		segmentID := uuid.NewString()
		content := []byte(uuid.NewString())

		// Prepare mock
		mockCache.On(
			"GetSegment",
			mock.AnythingOfType("*context.emptyCtx"),
			segmentID,
		).Return(content, nil).Once()

		resp, err := uut.GetSegment(utCtxt, common.VideoSegment{
			ID: segmentID,
		})
		assert.Nil(err)
		assert.Equal(content, resp)
	}

	// Case 1: segment reader provided
	{
		uut, err := vod.NewSegmentManager(mockCache, mockReader, segmentTTL)
		assert.Nil(err)

		segmentID := uuid.NewString()
		content := []byte(uuid.NewString())

		testSegment := common.VideoSegment{
			ID: segmentID,
			Segment: hls.Segment{
				URI: fmt.Sprintf("file:///%s.ts", segmentID),
			},
		}

		// Prepare mock
		mockCache.On(
			"GetSegment",
			mock.AnythingOfType("*context.timerCtx"),
			segmentID,
		).Return(nil, fmt.Errorf("dummy error")).Once()
		mockReader.On(
			"ReadSegment",
			mock.AnythingOfType("*context.timerCtx"),
			segmentID,
			mock.AnythingOfType("*url.URL"),
			mock.AnythingOfType("utils.SegmentReturnCallback"),
		).Run(func(args mock.Arguments) {
			// Parse the parameters
			id := args.Get(1).(string)
			url := args.Get(2).(*url.URL)

			assert.Equal(segmentID, id)
			assert.Equal(testSegment.URI, url.String())

			// Trigger the callback to return the read "segment"
			returnCB := args.Get(3).(utils.SegmentReturnCallback)
			go func() {
				assert.Nil(returnCB(utCtxt, segmentID, content))
			}()
		}).Return(nil).Once()
		mockCache.On(
			"CacheSegment",
			mock.AnythingOfType("*context.timerCtx"),
			segmentID,
			content,
			segmentTTL,
		).Return(nil).Once()

		lclCtxt, cancel := context.WithTimeout(utCtxt, time.Millisecond*10)
		defer cancel()
		resp, err := uut.GetSegment(lclCtxt, testSegment)
		assert.Nil(err)
		assert.Equal(content, resp)
	}
}
