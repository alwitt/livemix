package vod

import (
	"context"
	"fmt"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
)

// SegmentManager segment cache management which integrates segment cache with segment reader
type SegmentManager interface {
	/*
		GetSegment fetch video segment from cache. If cache miss, get the segment from source.

			@param ctxt context.Context - execution context
			@param target common.VideoSegment - video segment to fetch
			@returns segment data
	*/
	GetSegment(ctxt context.Context, target common.VideoSegment) ([]byte, error)
}

// segmentManagerImpl implements SegmentManager
type segmentManagerImpl struct {
	goutils.Component
	cache    utils.VideoSegmentCache
	reader   utils.SegmentReader
	cacheTTL time.Duration
}

/*
NewSegmentManager define new segment manager

	@param cache utils.VideoSegmentCache - segment cache
	@param reader utils.SegmentReader - segment reader
	@param cacheTTL time.Duration - when recording new entries in cache, use this TTL
	@returns SegmentManager
*/
func NewSegmentManager(
	cache utils.VideoSegmentCache, reader utils.SegmentReader, cacheTTL time.Duration,
) (SegmentManager, error) {
	return &segmentManagerImpl{
		Component: goutils.Component{
			LogTags: log.Fields{"module": "vod", "component": "segment-manager"},
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		}, cache: cache, reader: reader, cacheTTL: cacheTTL,
	}, nil
}

/*
TODO FIXME:

Add metrics
* segment get action
	* total bytes - count
	* total segments - count

Labels:
* "cache-hit": "true" or "false"
* "source": video source ID

*/

func (m *segmentManagerImpl) GetSegment(
	ctxt context.Context, target common.VideoSegment,
) ([]byte, error) {
	logTags := m.GetLogTagsForContext(ctxt)

	// Check cache first
	content, err := m.cache.GetSegment(ctxt, target)
	if err == nil {
		return content, nil
	}

	// Cache miss
	//
	// If reader was provided, read the segment from the source
	if m.reader == nil {
		err := fmt.Errorf("no segment reader available")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", target.SourceID).
			WithField("segment", target.Name).
			Error("Segment read failure")
		return nil, err
	}

	// Prepare data return path for segment reader
	resultChan := make(chan []byte)
	readSegmentCB := func(ctxt context.Context, segmentID string, content []byte) error {
		resultChan <- content
		return nil
	}

	// Request reading segment from source
	if err := m.reader.ReadSegment(ctxt, target, readSegmentCB); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", target.SourceID).
			WithField("segment", target.Name).
			WithField("segment-uri", target.URI).
			Error("Segment read request could not be submitted")
		return nil, err
	}

	// Wait for response from source
	select {
	case <-ctxt.Done():
		err := fmt.Errorf("request context ended before segment completed")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", target.SourceID).
			WithField("segment", target.Name).
			WithField("segment-uri", target.URI).
			Error("Segment read request failed")
		return nil, err
	case msg, ok := <-resultChan:
		if !ok {
			err := fmt.Errorf("internal message passing failure")
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source-id", target.SourceID).
				WithField("segment", target.Name).
				WithField("segment-uri", target.URI).
				Error("Segment read request failed")
			return nil, err
		}
		content = msg
	}

	// Record in cache for future reads
	err = m.cache.CacheSegment(
		ctxt, common.VideoSegmentWithData{VideoSegment: target, Content: content}, m.cacheTTL,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", target.SourceID).
			WithField("segment", target.Name).
			Error("Unable to cache newly read segment content")
	}

	return content, nil
}
