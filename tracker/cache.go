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
		ListCachedSegments fetch currently cached segments

			@param ctxt context.Context - execution context
			@returns list of cached segment IDs
	*/
	ListCachedSegments(ctxt context.Context) ([]string, error)

	/*
		ListMissingSegments given a list of segment IDs, return which IDs are unknown

			@param ctxt context.Context - execution context
			@param expectedSegmentIDs []string - list of segment IDs to check
			@returns list of unknown IDs
	*/
	ListMissingSegments(ctxt context.Context, expectedSegmentIDs []string) ([]string, error)

	/*
		GetSegment fetch video segment from cache

			@param ctxt context.Context - execution context
			@param segmentID string - segment reference ID
			@returns MPEG-TS file content
	*/
	GetSegment(ctxt context.Context, segmentID string) ([]byte, error)
}

// sourceHLSSegmentCacheImpl implements SourceHLSSegmentCache
type sourceHLSSegmentCacheImpl struct {
	goutils.Component
	cache map[string][]byte
	lock  sync.RWMutex
}

/*
NewSourceHLSSegmentCache define new single HLS source video segment cache

	@param source common.VideoSource - the HLS source to tracker
	@returns new SourceHLSSegmentCache
*/
func NewSourceHLSSegmentCache(source common.VideoSource) (SourceHLSSegmentCache, error) {
	return &sourceHLSSegmentCacheImpl{
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

func (c *sourceHLSSegmentCacheImpl) CacheSegment(
	ctxt context.Context, segmentID string, content []byte,
) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[segmentID] = content
	return nil
}

func (c *sourceHLSSegmentCacheImpl) PurgeSegments(
	ctxt context.Context, segmentIDs []string,
) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	for _, segmentID := range segmentIDs {
		delete(c.cache, segmentID)
	}

	return nil
}

func (c *sourceHLSSegmentCacheImpl) ListCachedSegments(ctxt context.Context) ([]string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	result := []string{}
	for segmentID := range c.cache {
		result = append(result, segmentID)
	}

	return result, nil
}

func (c *sourceHLSSegmentCacheImpl) ListMissingSegments(
	ctxt context.Context, expectedSegmentIDs []string,
) ([]string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	missing := []string{}
	for _, segmentID := range expectedSegmentIDs {
		if _, ok := c.cache[segmentID]; !ok {
			missing = append(missing, segmentID)
		}
	}

	return missing, nil
}

func (c *sourceHLSSegmentCacheImpl) GetSegment(
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
