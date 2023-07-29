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

	/* Metrics Collection Agents */
	segmentIOMetrics utils.SegmentMetricsAgent
}

/*
NewSegmentManager define new segment manager

	@param ctxt context.Context - execution context
	@param cache utils.VideoSegmentCache - segment cache
	@param reader utils.SegmentReader - segment reader
	@param cacheTTL time.Duration - when recording new entries in cache, use this TTL
	@param metrics goutils.MetricsCollector - metrics framework client
	@returns SegmentManager
*/
func NewSegmentManager(
	ctxt context.Context,
	cache utils.VideoSegmentCache,
	reader utils.SegmentReader,
	cacheTTL time.Duration,
	metrics goutils.MetricsCollector,
) (SegmentManager, error) {
	logTags := log.Fields{"module": "vod", "component": "segment-manager"}

	instance := &segmentManagerImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		}, cache: cache, reader: reader, cacheTTL: cacheTTL, segmentIOMetrics: nil,
	}

	// Install metrics
	if metrics != nil {
		var err error
		instance.segmentIOMetrics, err = utils.NewSegmentMetricsAgent(
			ctxt,
			metrics,
			utils.MetricsNameVODSegmentMgmtSegmentLen,
			"Tracking total bytes read VOD segment manager",
			utils.MetricsNameVODSegmentMgmtIOCount,
			"Tracking total segments read by VOD segment manager",
			[]string{"cache_hit", "source"},
		)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				Error("Unable to define segment IO tracking metrics helper agent")
			return nil, err
		}
	}
	return instance, nil
}

func (m *segmentManagerImpl) GetSegment(
	ctxt context.Context, target common.VideoSegment,
) ([]byte, error) {
	logTags := m.GetLogTagsForContext(ctxt)

	// Check cache first
	content, err := m.cache.GetSegment(ctxt, target)
	if err == nil {
		// Update metrics
		if m.segmentIOMetrics != nil {
			m.segmentIOMetrics.RecordSegment(len(content), map[string]string{
				"cache_hit": "true", "source": target.SourceID,
			})
		}
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

	// Update metrics
	if m.segmentIOMetrics != nil {
		m.segmentIOMetrics.RecordSegment(len(content), map[string]string{
			"cache_hit": "false", "source": target.SourceID,
		})
	}
	return content, nil
}
