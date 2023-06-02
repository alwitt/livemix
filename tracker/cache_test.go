package tracker_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/tracker"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSegmentCacheBasicSanity(t *testing.T) {
	assert := assert.New(t)

	testSource := common.VideoSource{
		ID:   uuid.NewString(),
		Name: fmt.Sprintf("vid-%s.m3u8", uuid.NewString()),
	}
	testSource.PlaylistURI = fmt.Sprintf("file:///%s", testSource.Name)

	uut, err := tracker.NewLocalSourceHLSSegmentCache(testSource)
	assert.Nil(err)

	utCtxt := context.Background()

	// Case 0: no segments cached
	{
		_, err := uut.GetSegment(utCtxt, uuid.NewString())
		assert.NotNil(err)
	}

	// Case 1: add segment
	segment0 := uuid.NewString()
	content0 := []byte(uuid.NewString())
	assert.Nil(uut.CacheSegment(utCtxt, segment0, content0))
	{
		content, err := uut.GetSegment(utCtxt, segment0)
		assert.Nil(err)
		assert.Equal(content0, content)
		_, err = uut.GetSegment(utCtxt, uuid.NewString())
		assert.NotNil(err)
	}

	// Case 2: update segment content
	content1 := []byte(uuid.NewString())
	assert.Nil(uut.CacheSegment(utCtxt, segment0, content1))
	{
		content, err := uut.GetSegment(utCtxt, segment0)
		assert.Nil(err)
		assert.Equal(content1, content)
	}

	// Case 3: add segment
	segment2 := uuid.NewString()
	content2 := []byte(uuid.NewString())
	assert.Nil(uut.CacheSegment(utCtxt, segment2, content2))
	{
		content, err := uut.GetSegment(utCtxt, segment2)
		assert.Nil(err)
		assert.Equal(content2, content)
	}

	// Case 4: delete from cache
	assert.Nil(uut.PurgeSegments(utCtxt, []string{segment0}))
	{
		checkIDs := []string{segment0, segment2}
		cached, err := uut.GetSegments(utCtxt, checkIDs)
		assert.Nil(err)
		assert.Len(cached, 1)
		assert.Contains(cached, segment2)
	}
}
