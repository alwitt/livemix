package utils

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	"github.com/prometheus/client_golang/prometheus"
)

// SegmentReturnCallback function signature of callback to receive a read segment
type SegmentReturnCallback func(ctxt context.Context, segmentID string, content []byte) error

// SegmentReader support daemon process which reads HLS MPEG-TS segment asynchronously
type SegmentReader interface {
	/*
		ReadSegment read one segment from specified location

			@param ctxt context.Context - execution context
			@param segment common.VideoSegment - video segment
			@param returnCB SegmentReturnCallback - callback used to return the read segment back
	*/
	ReadSegment(
		ctxt context.Context, segment common.VideoSegment, returnCB SegmentReturnCallback,
	) error

	/*
		Stop stops the daemon process

			@param ctxt context.Context - execution context
	*/
	Stop(ctxt context.Context) error
}

// segmentReader implements SegmentReader
type segmentReader struct {
	goutils.Component
	workers          goutils.TaskProcessor
	wg               sync.WaitGroup
	workerContext    context.Context
	workerCtxtCancel context.CancelFunc
	s3               S3Client
	maxSegReadTime   time.Duration

	/* Metrics Collection Agents */
	segmentIOMetrics   SegmentMetricsAgent
	readLatencyMetrics *prometheus.CounterVec
}

/*
NewSegmentReader define new SegmentReader

	@param parentContext context.Context - context from which to define the worker context
	@param workerCount int - number of parallel read worker to define
	@param maxSegReadTime time.Duration - max time allowed to completed a segment read
	@param s3 S3Client - S3 client for operating against the S3 server
	@param metrics goutils.MetricsCollector - metrics framework client
	@return new SegmentReader
*/
func NewSegmentReader(
	parentContext context.Context,
	workerCount int,
	maxSegReadTime time.Duration,
	s3 S3Client,
	metrics goutils.MetricsCollector,
) (SegmentReader, error) {
	logTags := log.Fields{
		"module":    "utils",
		"component": "hls-video-segment-reader",
	}
	workers, err := goutils.GetNewTaskDemuxProcessorInstance(
		parentContext, "segment-readers", workerCount, workerCount, logTags,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define worker thread pool")
		return nil, err
	}

	workerCtxt, cancel := context.WithCancel(parentContext)

	reader := &segmentReader{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		workers:          workers,
		wg:               sync.WaitGroup{},
		workerContext:    workerCtxt,
		workerCtxtCancel: cancel,
		s3:               s3,
		maxSegReadTime:   maxSegReadTime,
		segmentIOMetrics: nil,
	}

	// Define supported tasks
	if err := workers.AddToTaskExecutionMap(
		reflect.TypeOf(readSegmentRequest{}), reader.readSegment,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to install task definition")
		return nil, err
	}

	// Start the workers
	if err := workers.StartEventLoop(&reader.wg); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to start the worker thread pool")
		return nil, err
	}

	// Install metrics
	if metrics != nil {
		reader.segmentIOMetrics, err = NewSegmentMetricsAgent(
			parentContext,
			metrics,
			MetricsNameUtilFetcherSegmentLen,
			"Tracking total bytes read by video segment fetcher",
			MetricsNameUtilFetcherIOCount,
			"Tracking total segments read by video segment fetcher",
			[]string{"type", "source"},
		)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				Error("Unable to define segment IO tracking metrics helper agent")
			return nil, err
		}
		reader.readLatencyMetrics, err = metrics.InstallCustomCounterVecMetrics(
			parentContext,
			MetricsNameUtilFetcherReadLatency,
			"Tracking segment read latency of video segment fetcher",
			[]string{"type"},
		)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				Error("Unable to define segment read latency tracking metrics")
			return nil, err
		}
	}

	return reader, nil
}

type readSegmentRequest struct {
	segment  common.VideoSegment
	fetchURI *url.URL
	returnCB SegmentReturnCallback
}

func (r *segmentReader) ReadSegment(
	ctxt context.Context, segment common.VideoSegment, returnCB SegmentReturnCallback,
) error {
	logTags := r.GetLogTagsForContext(ctxt)

	parsedURI, err := url.Parse(segment.URI)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segemnt", segment.Name).
			WithField("segment-uri", segment.URI).
			Error("Unable to parse segment URI")
		return err
	}

	request := readSegmentRequest{segment: segment, fetchURI: parsedURI, returnCB: returnCB}

	if err := r.workers.Submit(ctxt, request); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to submit 'ReadSegment' job")
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segemnt", segment.Name).
		WithField("segment-uri", parsedURI.String()).
		Debug("Submitted 'ReadSegment' job request")

	return nil
}

func (r *segmentReader) readSegment(params interface{}) error {
	// Convert params into expected data type
	if readParams, ok := params.(readSegmentRequest); ok {
		return r.coreReadSegment(readParams.segment, readParams.fetchURI, readParams.returnCB)
	}
	err := fmt.Errorf("received unexpected call parameters: %s", reflect.TypeOf(params))
	logTags := r.GetLogTagsForContext(r.workerContext)
	log.WithError(err).WithFields(logTags).Error("'readSegment' processing failure")
	return err
}

// coreReadSegment contains the actual logic for the ReadSegment function
func (r *segmentReader) coreReadSegment(
	segment common.VideoSegment, fetchURI *url.URL, returnCB SegmentReturnCallback,
) error {
	logTags := r.GetLogTagsForContext(r.workerContext)

	// Choose the read method based on the schema of the segment path URL
	switch fetchURI.Scheme {
	case "file":
		return r.readSegmentFromFile(segment, fetchURI, returnCB)
	case "s3":
		return r.readSegmentFromS3(segment, fetchURI, returnCB)
	default:
		err := fmt.Errorf("invalid segment path URL")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segemnt", segment.Name).
			WithField("segment-uri", segment.String()).
			Error("Unable to read segment file")
		return err
	}
}

// readSegmentFromFile support reading video segment from file
func (r *segmentReader) readSegmentFromFile(
	segment common.VideoSegment, fetchURI *url.URL, returnCB SegmentReturnCallback,
) error {
	logTags := r.GetLogTagsForContext(r.workerContext)

	startTime := time.Now().UTC()
	segmentFile, err := os.Open(fetchURI.Path)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segemnt", segment.Name).
			WithField("segment-uri", fetchURI.String()).
			Error("Unable to open segment file")
		return err
	}
	defer func() {
		_ = segmentFile.Close()
	}()

	content, err := io.ReadAll(segmentFile)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segemnt", segment.Name).
			WithField("segment-uri", fetchURI.String()).
			Error("Reading segment file failed")
		return err
	}
	endTime := time.Now().UTC()
	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segemnt", segment.Name).
		WithField("segment-uri", fetchURI.String()).
		WithField("length", len(content)).
		Debug("Read segment file")

	if err := returnCB(r.workerContext, segment.ID, content); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segemnt", segment.Name).
			WithField("segment-uri", fetchURI.String()).
			Error("Unable to pass on read segment content")
		return err
	}

	// Update metrics
	if r.segmentIOMetrics != nil {
		r.segmentIOMetrics.RecordSegment(len(content), map[string]string{
			"type": "file", "source": segment.SourceID,
		})
	}
	if r.readLatencyMetrics != nil {
		r.readLatencyMetrics.
			With(prometheus.Labels{"type": "file"}).
			Add(float64(endTime.Sub(startTime).Seconds()))
	}
	return nil
}

// CleanupObjectKey cleanup object key string
//
// Remove any leading `/` from object key
func CleanupObjectKey(orig string) string {
	parts := strings.Split(orig, "/")
	keep := []string{}
	for _, onePart := range parts {
		if onePart != "" {
			keep = append(keep, onePart)
		}
	}
	return strings.Join(keep, "/")
}

func (r *segmentReader) readSegmentFromS3(
	segment common.VideoSegment, fetchURI *url.URL, returnCB SegmentReturnCallback,
) error {
	readCtxt, readCtxtCancel := context.WithTimeout(r.workerContext, r.maxSegReadTime)
	defer readCtxtCancel()
	logTags := r.GetLogTagsForContext(r.workerContext)

	if r.s3 == nil {
		err := fmt.Errorf("no S3 client specified")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segemnt", segment.Name).
			WithField("segment-uri", fetchURI.String()).
			Error("Unable to to read segment from S3")
		return err
	}

	sourceBucket := fetchURI.Host
	segmentObjectKey := CleanupObjectKey(fetchURI.Path)

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segemnt", segment.Name).
		WithField("segment-uri", fetchURI.String()).
		Info("Fetching segment from S3")

	startTime := time.Now().UTC()
	content, err := r.s3.GetObject(readCtxt, sourceBucket, segmentObjectKey)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segemnt", segment.Name).
			WithField("segment-uri", fetchURI.String()).
			Error("Fetching segment failed")
		return err
	}
	endTime := time.Now().UTC()

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segemnt", segment.Name).
		WithField("segment-uri", fetchURI.String()).
		WithField("length", len(content)).
		Info("Read segment from S3")

	if err := returnCB(r.workerContext, segment.ID, content); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segemnt", segment.Name).
			WithField("segment-uri", fetchURI.String()).
			Error("Unable to pass on read segment content")
		return err
	}

	// Update metrics
	if r.segmentIOMetrics != nil {
		r.segmentIOMetrics.RecordSegment(len(content), map[string]string{
			"type": "s3", "source": segment.SourceID,
		})
	}
	if r.readLatencyMetrics != nil {
		r.readLatencyMetrics.
			With(prometheus.Labels{"type": "s3"}).
			Add(float64(endTime.Sub(startTime).Seconds()))
	}
	return nil
}

func (r *segmentReader) Stop(ctxt context.Context) error {
	r.workerCtxtCancel()
	return r.workers.StopEventLoop()
}
