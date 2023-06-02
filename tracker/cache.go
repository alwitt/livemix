package tracker

import (
	"context"
	"fmt"
	"sync"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
)

// SourceHLSSegmentCache video segment cache for a single HLS video source updated
// in sync with SourceHLSTracker
type SourceHLSSegmentCache interface {
	/*
		CacheSegment add video segment to cache

			@param ctxt context.Context - execution context
			@param segmentID string - segment reference ID
			@param content []byte - byte array containing content of the MPEG-TS file
	*/
	CacheSegment(ctxt context.Context, segmentID string, content []byte) error

	/*
		PurgeSegment delete video segment from cache

			@param ctxt context.Context - execution context
			@param segmentID []string - list of segments by ID to purge
	*/
	PurgeSegments(ctxt context.Context, segmentIDs []string) error

	/*
		GetSegment fetch video segment from cache

			@param ctxt context.Context - execution context
			@param segmentID string - segment reference ID
			@returns MPEG-TS file content
	*/
	GetSegment(ctxt context.Context, segmentID string) ([]byte, error)

	/*
		GetSegments fetch group of video segments from cache. The returned entries are what is
		currently available within the cache.

			@param ctxt context.Context - execution context
			@param segmentIDs []string - segment reference IDs
			@returns set of MPEG-TS file content
	*/
	GetSegments(ctxt context.Context, segmentIDs []string) (map[string][]byte, error)
}

// inProcessSegmentCacheImpl implements SourceHLSSegmentCache
type inProcessSegmentCacheImpl struct {
	goutils.Component
	cache map[string][]byte
	lock  sync.RWMutex
}

/*
NewLocalSourceHLSSegmentCache define new local in process single HLS source video segment cache

	@param source common.VideoSource - the HLS source to tracker
	@returns new SourceHLSSegmentCache
*/
func NewLocalSourceHLSSegmentCache(source common.VideoSource) (SourceHLSSegmentCache, error) {
	return &inProcessSegmentCacheImpl{
		Component: goutils.Component{
			LogTags: log.Fields{
				"module":       "tracker",
				"component":    "hls-source-segment-cache",
				"instance":     source.Name,
				"playlist-uri": source.PlaylistURI,
			},
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		cache: make(map[string][]byte),
		lock:  sync.RWMutex{},
	}, nil
}

func (c *inProcessSegmentCacheImpl) CacheSegment(
	ctxt context.Context, segmentID string, content []byte,
) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[segmentID] = content
	return nil
}

func (c *inProcessSegmentCacheImpl) PurgeSegments(
	ctxt context.Context, segmentIDs []string,
) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	for _, segmentID := range segmentIDs {
		delete(c.cache, segmentID)
	}

	return nil
}

func (c *inProcessSegmentCacheImpl) GetSegment(
	ctxt context.Context, segmentID string,
) ([]byte, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	content, ok := c.cache[segmentID]
	if !ok {
		return nil, fmt.Errorf("segment ID '%s' is unknown", segmentID)
	}
	return content, nil
}

func (c *inProcessSegmentCacheImpl) GetSegments(
	ctxt context.Context, segmentIDs []string,
) (map[string][]byte, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	result := map[string][]byte{}
	for _, segmentID := range segmentIDs {
		if segment, ok := c.cache[segmentID]; ok {
			result[segmentID] = segment
		}
	}

	return result, nil
}
