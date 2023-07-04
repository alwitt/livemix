package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/apex/log"
	"github.com/bradfitz/gomemcache/memcache"
)

// VideoSegmentCache video segment cache
type VideoSegmentCache interface {
	/*
		CacheSegment add video segment to cache

			@param ctxt context.Context - execution context
			@param segmentID string - segment reference ID
			@param content []byte - byte array containing content of the MPEG-TS file
			@param ttl time.Duration - data retention in seconds before the entry expires
	*/
	CacheSegment(ctxt context.Context, segmentID string, content []byte, ttl time.Duration) error

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

// =====================================================================================
// In-Process (Local Ram) Video Segment Cache

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
	ctxt context.Context, segmentID string, content []byte, ttl time.Duration,
) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[segmentID] = inProcessCacheEntry{expireAt: time.Now().UTC().Add(ttl), content: content}
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
	return content.content, nil
}

func (c *inProcessSegmentCacheImpl) GetSegments(
	ctxt context.Context, segmentIDs []string,
) (map[string][]byte, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	result := map[string][]byte{}
	for _, segmentID := range segmentIDs {
		if segment, ok := c.cache[segmentID]; ok {
			result[segmentID] = segment.content
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
	ctxt context.Context, segmentID string, content []byte, ttl time.Duration,
) error {
	logTags := c.GetLogTagsForContext(ctxt)
	ttlSec := int32(ttl.Seconds())
	log.
		WithFields(logTags).
		WithField("segment-id", segmentID).
		WithField("ttl", ttlSec).
		Debug("Caching segment")
	cacheEntry := &memcache.Item{Key: segmentID, Value: content, Expiration: int32(ttl.Seconds())}
	if err := c.client.Set(cacheEntry); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment-id", segmentID).
			Error("Segment failed to cache")
		return err
	}
	log.
		WithFields(logTags).
		WithField("segment-id", segmentID).
		WithField("ttl", ttlSec).
		Debug("Cached segment")
	return nil
}

func (c *memcachedSegmentCacheImpl) PurgeSegments(ctxt context.Context, segmentIDs []string) error {
	logTags := c.GetLogTagsForContext(ctxt)
	var err error
	err = nil
	failedSegs := []string{}
	purgedSegs := []string{}
	// Go through each segment
	for _, segmentID := range segmentIDs {
		log.
			WithFields(logTags).
			WithField("segment-id", segmentID).
			Debug("Purging segment")
		if lclErr := c.client.Delete(segmentID); lclErr != nil {
			failedSegs = append(failedSegs, segmentID)
			log.
				WithError(lclErr).
				WithFields(logTags).
				WithField("segment-id", segmentID).
				Error("Segment purge failed")
		} else {
			purgedSegs = append(purgedSegs, segmentID)
		}
		log.
			WithFields(logTags).
			WithField("segment-id", segmentID).
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
	ctxt context.Context, segmentID string,
) ([]byte, error) {
	logTags := c.GetLogTagsForContext(ctxt)
	log.
		WithFields(logTags).
		WithField("segment-id", segmentID).
		Debug("Reading segment")
	entry, err := c.client.Get(segmentID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment-id", segmentID).
			Error("Failed to fetch segment")
		return nil, err
	}
	log.
		WithFields(logTags).
		WithField("segment-id", segmentID).
		Debug("Read segment")
	return entry.Value, nil
}

func (c *memcachedSegmentCacheImpl) GetSegments(
	ctxt context.Context, segmentIDs []string,
) (map[string][]byte, error) {
	logTags := c.GetLogTagsForContext(ctxt)
	log.
		WithFields(logTags).
		WithField("segment-ids", segmentIDs).
		Debug("Reading segments")
	entries, err := c.client.GetMulti(segmentIDs)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment-ids", segmentIDs).
			Debug("Multi-segment read failed")
		return nil, err
	}
	log.
		WithFields(logTags).
		WithField("segment-ids", segmentIDs).
		Debug("Read segments")
	result := map[string][]byte{}
	for segmentID, segment := range entries {
		result[segmentID] = segment.Value
	}
	return result, nil
}
