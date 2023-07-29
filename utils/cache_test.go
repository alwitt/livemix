package utils

import (
	"context"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestLocalSegmentCacheBasicSanity(t *testing.T) {
	assert := assert.New(t)

	utCtxt := context.Background()

	uut, err := NewLocalVideoSegmentCache(utCtxt, time.Minute)
	assert.Nil(err)

	// Case 0: no segments cached
	{
		_, err := uut.GetSegment(utCtxt, common.VideoSegment{
			ID: uuid.NewString(), SourceID: uuid.NewString(),
		})
		assert.NotNil(err)
	}

	// Case 1: add segment
	segment0 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	assert.Nil(uut.CacheSegment(utCtxt, segment0, time.Second))
	{
		content, err := uut.GetSegment(utCtxt, segment0.VideoSegment)
		assert.Nil(err)
		assert.Equal(segment0.Content, content)
		_, err = uut.GetSegment(
			utCtxt, common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		)
		assert.NotNil(err)
	}

	// Case 2: update segment content
	segment0Dup := common.VideoSegmentWithData{
		VideoSegment: segment0.VideoSegment,
		Content:      []byte(uuid.NewString()),
	}
	assert.Nil(uut.CacheSegment(utCtxt, segment0Dup, time.Second))
	{
		content, err := uut.GetSegment(utCtxt, segment0Dup.VideoSegment)
		assert.Nil(err)
		assert.Equal(segment0Dup.Content, content)
	}

	// Case 3: add segment
	segment2 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	assert.Nil(uut.CacheSegment(utCtxt, segment2, time.Second))
	{
		content, err := uut.GetSegment(utCtxt, segment2.VideoSegment)
		assert.Nil(err)
		assert.Equal(segment2.Content, content)
	}

	// Case 4: delete from cache
	assert.Nil(uut.PurgeSegments(utCtxt, []common.VideoSegment{segment0.VideoSegment}))
	{
		checkIDs := []common.VideoSegment{segment0.VideoSegment, segment2.VideoSegment}
		cached, err := uut.GetSegments(utCtxt, checkIDs)
		assert.Nil(err)
		assert.Len(cached, 1)
		assert.Contains(cached, segment2.ID)
	}
}

func TestLocalSegmentCacheManualCachePurgeTrigger(t *testing.T) {
	assert := assert.New(t)

	utCtxt := context.Background()

	uut, err := NewLocalVideoSegmentCache(utCtxt, time.Minute)
	assert.Nil(err)

	startTime := time.Now().UTC()

	// Setup test entries
	entry0 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	ttl0 := time.Second
	entry1 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	ttl1 := time.Second * 2
	entry2 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	ttl2 := time.Second * 4
	assert.Nil(uut.CacheSegment(utCtxt, entry0, ttl0))
	assert.Nil(uut.CacheSegment(utCtxt, entry1, ttl1))
	assert.Nil(uut.CacheSegment(utCtxt, entry2, ttl2))

	uutCast, ok := uut.(*inProcessSegmentCacheImpl)
	assert.True(ok)

	purgeTime := startTime
	assert.Nil(uutCast.purgeExpiredEntry(utCtxt, purgeTime))
	{
		entries, err := uut.GetSegments(
			utCtxt, []common.VideoSegment{entry0.VideoSegment, entry1.VideoSegment, entry2.VideoSegment},
		)
		assert.Nil(err)
		assert.Len(entries, 3)
	}

	purgeTime = purgeTime.Add(time.Millisecond * 1250)
	assert.Nil(uutCast.purgeExpiredEntry(utCtxt, purgeTime))
	{
		entries, err := uut.GetSegments(
			utCtxt, []common.VideoSegment{entry0.VideoSegment, entry1.VideoSegment, entry2.VideoSegment},
		)
		assert.Nil(err)
		assert.Len(entries, 2)
		_, err = uut.GetSegment(utCtxt, entry0.VideoSegment)
		assert.NotNil(err)
	}

	purgeTime = purgeTime.Add(time.Millisecond * 2500)
	assert.Nil(uutCast.purgeExpiredEntry(utCtxt, purgeTime))
	{
		entries, err := uut.GetSegments(
			utCtxt, []common.VideoSegment{entry0.VideoSegment, entry1.VideoSegment, entry2.VideoSegment},
		)
		assert.Nil(err)
		assert.Len(entries, 1)
		_, err = uut.GetSegment(utCtxt, entry0.VideoSegment)
		assert.NotNil(err)
		_, err = uut.GetSegment(utCtxt, entry1.VideoSegment)
		assert.NotNil(err)
		_, err = uut.GetSegment(utCtxt, entry2.VideoSegment)
		assert.Nil(err)
	}
}

func TestLocalSegmentCacheAutoCachePurge(t *testing.T) {
	assert := assert.New(t)

	utCtxt := context.Background()

	uut, err := NewLocalVideoSegmentCache(utCtxt, time.Millisecond*100)
	assert.Nil(err)

	// Setup test entries
	entry0 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	ttl0 := time.Millisecond * 50
	entry1 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	ttl1 := time.Millisecond * 140
	entry2 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	ttl2 := time.Millisecond * 240
	assert.Nil(uut.CacheSegment(utCtxt, entry0, ttl0))
	assert.Nil(uut.CacheSegment(utCtxt, entry1, ttl1))
	assert.Nil(uut.CacheSegment(utCtxt, entry2, ttl2))

	// Verify all entry are in place
	{
		entries, err := uut.GetSegments(
			utCtxt, []common.VideoSegment{entry0.VideoSegment, entry1.VideoSegment, entry2.VideoSegment},
		)
		assert.Nil(err)
		assert.Len(entries, 3)
	}

	// Verify first entry is gone
	time.Sleep(time.Millisecond * 120)
	{
		entries, err := uut.GetSegments(
			utCtxt, []common.VideoSegment{entry0.VideoSegment, entry1.VideoSegment, entry2.VideoSegment},
		)
		assert.Nil(err)
		assert.Len(entries, 2)
		_, err = uut.GetSegment(utCtxt, entry0.VideoSegment)
		assert.NotNil(err)
	}

	// Verify second entry is gone
	time.Sleep(time.Millisecond * 90)
	{
		entries, err := uut.GetSegments(
			utCtxt, []common.VideoSegment{entry0.VideoSegment, entry1.VideoSegment, entry2.VideoSegment},
		)
		assert.Nil(err)
		assert.Len(entries, 1)
		_, err = uut.GetSegment(utCtxt, entry0.VideoSegment)
		assert.NotNil(err)
		_, err = uut.GetSegment(utCtxt, entry1.VideoSegment)
		assert.NotNil(err)
		_, err = uut.GetSegment(utCtxt, entry2.VideoSegment)
		assert.Nil(err)
	}
}

func TestMemcachedSegmentCacheBasicSanity(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	uut, err := NewMemcachedVideoSegmentCache([]string{"localhost:18080"})
	assert.Nil(err)

	// Case 0: no segments cached
	{
		_, err := uut.GetSegment(
			utCtxt, common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		)
		assert.NotNil(err)
	}

	// Case 1: add segment
	segment0 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	assert.Nil(uut.CacheSegment(utCtxt, segment0, time.Second))
	{
		content, err := uut.GetSegment(utCtxt, segment0.VideoSegment)
		assert.Nil(err)
		assert.Equal(segment0.Content, content)
		_, err = uut.GetSegment(
			utCtxt, common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		)
		assert.NotNil(err)
	}

	// Case 2: update segment content
	segment0Dup := common.VideoSegmentWithData{
		VideoSegment: segment0.VideoSegment,
		Content:      []byte(uuid.NewString()),
	}
	assert.Nil(uut.CacheSegment(utCtxt, segment0Dup, time.Second))
	{
		content, err := uut.GetSegment(utCtxt, segment0Dup.VideoSegment)
		assert.Nil(err)
		assert.Equal(segment0Dup.Content, content)
	}

	// Case 3: add segment
	segment2 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	assert.Nil(uut.CacheSegment(utCtxt, segment2, time.Second))
	{
		content, err := uut.GetSegment(utCtxt, segment2.VideoSegment)
		assert.Nil(err)
		assert.Equal(segment2.Content, content)
	}

	// Case 4: delete from cache
	assert.Nil(uut.PurgeSegments(utCtxt, []common.VideoSegment{segment0.VideoSegment}))
	{
		checkIDs := []common.VideoSegment{segment0.VideoSegment, segment2.VideoSegment}
		cached, err := uut.GetSegments(utCtxt, checkIDs)
		assert.Nil(err)
		assert.Len(cached, 1)
		assert.Contains(cached, segment2.ID)
	}
}

func TestMemcachedSegmentCacheAutoCachePurge(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	uut, err := NewMemcachedVideoSegmentCache([]string{"localhost:18080"})
	assert.Nil(err)

	// Case 0: add segment
	segment0 := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		Content:      []byte(uuid.NewString()),
	}
	assert.Nil(uut.CacheSegment(utCtxt, segment0, time.Second))
	{
		content, err := uut.GetSegment(utCtxt, segment0.VideoSegment)
		assert.Nil(err)
		assert.Equal(segment0.Content, content)
		_, err = uut.GetSegment(
			utCtxt, common.VideoSegment{ID: uuid.NewString(), SourceID: uuid.NewString()},
		)
		assert.NotNil(err)
	}

	time.Sleep(time.Millisecond * 1500)

	// Case 1: segment should be gone
	{
		_, err = uut.GetSegment(utCtxt, segment0.VideoSegment)
		assert.NotNil(err)
	}
}
