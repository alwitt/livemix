package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	"github.com/bradfitz/gomemcache/memcache"
)

// VideoSegmentCache video segment cache
type VideoSegmentCache interface {
	/*
		CacheSegment add video segment to cache

			@param ctxt context.Context - execution context
			@param segment common.VideoSegmentWithData - video segment to cache
			@param ttl time.Duration - data retention in seconds before the entry expires
	*/
	CacheSegment(ctxt context.Context, segment common.VideoSegmentWithData, ttl time.Duration) error

	/*
		PurgeSegment delete video segment from cache

			@param ctxt context.Context - execution context
			@param segments []common.VideoSegment - list of segments to purge
	*/
	PurgeSegments(ctxt context.Context, segments []common.VideoSegment) error

	/*
		GetSegment fetch video segment from cache

			@param ctxt context.Context - execution context
			@param segment common.VideoSegment - segment to read
			@returns MPEG-TS file content
	*/
	GetSegment(ctxt context.Context, segment common.VideoSegment) ([]byte, error)

	/*
		GetSegments fetch group of video segments from cache. The returned entries are what is
		currently available within the cache.

			@param ctxt context.Context - execution context
			@param segments []common.VideoSegment - segments to read
			@returns set of MPEG-TS file content
	*/
	GetSegments(ctxt context.Context, segments []common.VideoSegment) (map[string][]byte, error)
}

// =====================================================================================
// In-Process (Local Ram) Video Segment Cache

/*
TODO FIXME:

Add metrics
* current segments in cache - Gauge
* segment put action
	* total bytes - count
	* total segments - count
* segment get action
	* total bytes - count
	* total segment - count

Labels:
* "type": Fixed at "ram"
* "source": video source ID

*/

// inProcessCacheEntry wrapper structure holding content with retention support
type inProcessCacheEntry struct {
	expireAt time.Time
	content  []byte
}

// inProcessSegmentCacheImpl implements SourceHLSSegmentCache
type inProcessSegmentCacheImpl struct {
	goutils.Component
	cache                      map[string]inProcessCacheEntry
	lock                       sync.RWMutex
	retentionCheckTimer        goutils.IntervalTimer
	retentionExecContext       context.Context
	retentionExecContextCancel context.CancelFunc
	wg                         sync.WaitGroup
}

/*
NewLocalVideoSegmentCache define new local in process single HLS source video segment cache

	@param parentContext context.Context - parent context from which a worker context is defined
		for the data retention enforcement process
	@param retentionCheckInterval time.Duration - cache entry retention enforce interval
	@returns new SourceHLSSegmentCache
*/
func NewLocalVideoSegmentCache(
	parentContext context.Context, retentionCheckInterval time.Duration,
) (VideoSegmentCache, error) {
	logTags := log.Fields{
		"module":    "utils",
		"component": "video-segment-cache",
		"instance":  "in-process",
	}

	workerCtxt, cancel := context.WithCancel(parentContext)

	instance := &inProcessSegmentCacheImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		cache:                      make(map[string]inProcessCacheEntry),
		lock:                       sync.RWMutex{},
		retentionExecContext:       workerCtxt,
		retentionExecContextCancel: cancel,
		wg:                         sync.WaitGroup{},
	}

	timer, err := goutils.GetIntervalTimerInstance(parentContext, &instance.wg, logTags)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define support timer")
		return nil, err
	}
	instance.retentionCheckTimer = timer

	// Start interval timer to trigger the cache retention enforcement logic
	if err := timer.Start(retentionCheckInterval, func() error {
		currentTime := time.Now().UTC()
		return instance.purgeExpiredEntry(workerCtxt, currentTime)
	}, false); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to start support timer")
		return nil, err
	}

	return instance, nil
}

func (c *inProcessSegmentCacheImpl) CacheSegment(
	ctxt context.Context, segment common.VideoSegmentWithData, ttl time.Duration,
) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[segment.ID] = inProcessCacheEntry{
		expireAt: time.Now().UTC().Add(ttl), content: segment.Content,
	}
	return nil
}

func (c *inProcessSegmentCacheImpl) PurgeSegments(
	ctxt context.Context, segments []common.VideoSegment,
) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	for _, segment := range segments {
		delete(c.cache, segment.ID)
	}

	return nil
}

func (c *inProcessSegmentCacheImpl) GetSegment(
	ctxt context.Context, segment common.VideoSegment,
) ([]byte, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	content, ok := c.cache[segment.ID]
	if !ok {
		return nil, fmt.Errorf("segment ID '%s' is unknown", segment.ID)
	}
	return content.content, nil
}

func (c *inProcessSegmentCacheImpl) GetSegments(
	ctxt context.Context, segments []common.VideoSegment,
) (map[string][]byte, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	result := map[string][]byte{}
	for _, segment := range segments {
		if cachedSegment, ok := c.cache[segment.ID]; ok {
			result[segment.ID] = cachedSegment.content
		}
	}

	return result, nil
}

// purgeExpiredEntry purge expired cache entries
func (c *inProcessSegmentCacheImpl) purgeExpiredEntry(
	ctxt context.Context, currentTime time.Time,
) error {
	logTags := c.GetLogTagsForContext(ctxt)

	c.lock.Lock()
	defer c.lock.Unlock()

	log.WithFields(logTags).Info("Checking for expired video segments")
	// Check for expired entries
	purgeIDs := []string{}
	for segmentID, entry := range c.cache {
		if entry.expireAt.Before(currentTime) {
			purgeIDs = append(purgeIDs, segmentID)
			log.
				WithFields(logTags).
				WithField("segment-id", segmentID).
				Debug("Video segment expired")
		}
	}

	// Purge expired entries
	for _, purgeID := range purgeIDs {
		delete(c.cache, purgeID)
	}

	log.
		WithFields(logTags).
		Infof("Purged [%d] expired video segments. [%d] remain in cache", len(purgeIDs), len(c.cache))

	return nil
}

// =====================================================================================
// Memcached Video Segment Cache

/*
TODO FIXME:

Add metrics
* segment put action
	* total bytes - count
	* total segments - count
* segment get action
	* total bytes - count
	* total segment - count

Labels:
* "type": Fixed at "memcached"
* "source": video source ID

*/

// memcachedSegmentCacheImpl implements SourceHLSSegmentCache
type memcachedSegmentCacheImpl struct {
	goutils.Component
	client *memcache.Client
}

/*
NewMemcachedVideoSegmentCache define new memcached video segment cache

	@param servers []string - list of memcached servers to connect to
	@returns new VideoSegmentCache
*/
func NewMemcachedVideoSegmentCache(servers []string) (VideoSegmentCache, error) {
	logTags := log.Fields{
		"module":    "utils",
		"component": "video-segment-cache",
		"instance":  "memcached",
		"servers":   servers,
	}

	// Define memcached client
	mc := memcache.New(servers...)
	if err := mc.Ping(); err != nil {
		log.WithError(err).WithFields(logTags).Error("Server Up check failed")
		return nil, err
	}

	return &memcachedSegmentCacheImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		}, client: mc,
	}, nil
}

func (c *memcachedSegmentCacheImpl) CacheSegment(
	ctxt context.Context, segment common.VideoSegmentWithData, ttl time.Duration,
) error {
	logTags := c.GetLogTagsForContext(ctxt)
	ttlSec := int32(ttl.Seconds())
	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment", segment.Name).
		WithField("ttl", ttlSec).
		Debug("Caching segment")
	cacheEntry := &memcache.Item{
		Key: segment.ID, Value: segment.Content, Expiration: int32(ttl.Seconds()),
	}
	if err := c.client.Set(cacheEntry); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment", segment.Name).
			Error("Segment failed to cache")
		return err
	}
	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment", segment.ID).
		WithField("ttl", ttlSec).
		Debug("Cached segment")
	return nil
}

func (c *memcachedSegmentCacheImpl) PurgeSegments(
	ctxt context.Context, segments []common.VideoSegment,
) error {
	logTags := c.GetLogTagsForContext(ctxt)
	var err error
	err = nil
	failedSegs := []string{}
	purgedSegs := []string{}
	// Go through each segment
	for _, segment := range segments {
		log.
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment", segment.ID).
			Debug("Purging segment")
		if lclErr := c.client.Delete(segment.ID); lclErr != nil {
			failedSegs = append(failedSegs, segment.ID)
			log.
				WithError(lclErr).
				WithFields(logTags).
				WithField("source-id", segment.SourceID).
				WithField("segment", segment.ID).
				Error("Segment purge failed")
		} else {
			purgedSegs = append(purgedSegs, segment.ID)
		}
		log.
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment", segment.ID).
			Debug("Purged segment")
	}
	if len(failedSegs) > 0 {
		err = fmt.Errorf("failed to purge one or more segments")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment-ids", failedSegs).
			Error("Failed to purge segments")
	}
	log.
		WithFields(logTags).
		WithField("segment-ids", purgedSegs).
		Info("Purged segments")
	return err
}

func (c *memcachedSegmentCacheImpl) GetSegment(
	ctxt context.Context, segment common.VideoSegment,
) ([]byte, error) {
	logTags := c.GetLogTagsForContext(ctxt)
	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment", segment.ID).
		Debug("Reading segment")
	entry, err := c.client.Get(segment.ID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment", segment.ID).
			Error("Failed to fetch segment")
		return nil, err
	}
	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment", segment.ID).
		Debug("Read segment")
	return entry.Value, nil
}

func (c *memcachedSegmentCacheImpl) GetSegments(
	ctxt context.Context, segments []common.VideoSegment,
) (map[string][]byte, error) {
	logTags := c.GetLogTagsForContext(ctxt)
	sourceIDsCollect := map[string]bool{}
	segmentNames := []string{}
	segmentIDs := []string{}
	for _, segment := range segments {
		segmentNames = append(segmentNames, segment.Name)
		segmentIDs = append(segmentIDs, segment.ID)
		sourceIDsCollect[segment.SourceID] = true
	}
	sourceIDs := []string{}
	for sourceID := range sourceIDsCollect {
		sourceIDs = append(sourceIDs, sourceID)
	}
	log.
		WithFields(logTags).
		WithField("source-ids", sourceIDs).
		WithField("segments", segmentNames).
		Debug("Reading segments")
	entries, err := c.client.GetMulti(segmentIDs)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-ids", sourceIDs).
			WithField("segments", segmentNames).
			Debug("Multi-segment read failed")
		return nil, err
	}
	log.
		WithFields(logTags).
		WithField("source-ids", sourceIDs).
		WithField("segments", segmentNames).
		Debug("Read segments")
	result := map[string][]byte{}
	for segmentID, segment := range entries {
		result[segmentID] = segment.Value
	}
	return result, nil
}
