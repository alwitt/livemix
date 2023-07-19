package forwarder

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
	"github.com/go-resty/resty/v2"
)

// SegmentSender video segment transmit client
type SegmentSender interface {
	/*
		ForwardSegment forward a video segment to a receiver

			@param ctxt context.Context - execution context
			@param segment common.VideoSegmentWithData - the video segment to forward
	*/
	ForwardSegment(ctxt context.Context, segment common.VideoSegmentWithData) error
}

// ======================================================================================
// HTTP Transport

// httpSegmentSender HTTP video segment transmit client implementing SegmentSender
type httpSegmentSender struct {
	goutils.Component
	receiverURI *url.URL
	client      *resty.Client
}

/*
NewHTTPSegmentSender define new HTTP video segment transmit client

	@param segmentReceiverURI *url.URL - the URL to send the segments to
	@param httpClient *resty.Client - HTTP client to use
	@returns new sender instance
*/
func NewHTTPSegmentSender(
	segmentReceiverURI *url.URL,
	httpClient *resty.Client,
) (SegmentSender, error) {
	logTags := log.Fields{
		"module":    "forwarder",
		"component": "http-segment-sender",
		"receiver":  segmentReceiverURI.String(),
	}

	// The assumption is that the HTTP client has been prepared for operation

	return &httpSegmentSender{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		receiverURI: segmentReceiverURI,
		client:      httpClient,
	}, nil
}

func (s *httpSegmentSender) ForwardSegment(
	ctxt context.Context, segment common.VideoSegmentWithData,
) error {
	logTags := s.GetLogTagsForContext(ctxt)

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment-name", segment.Name).
		Debug("Forwarding segment")

	// Make request
	resp, err := s.client.R().
		// Set request header for segment reception
		SetHeader(ipc.HTTPSegmentForwardHeaderSourceID, segment.SourceID).
		SetHeader(ipc.HTTPSegmentForwardHeaderName, segment.Name).
		SetHeader(ipc.HTTPSegmentForwardHeaderStartTS, fmt.Sprintf("%d", segment.StartTime.Unix())).
		SetHeader(ipc.HTTPSegmentForwardHeaderLength, fmt.Sprintf("%d", int(segment.Length*1000))).
		SetHeader(ipc.HTTPSegmentForwardHeaderSegURI, segment.URI).
		// Set request payload
		SetBody(segment.Content).
		// Setup error parsing
		SetError(goutils.RestAPIBaseResponse{}).
		Post(s.receiverURI.String())

	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment-name", segment.Name).
			Debug("Segment forward request failed on call")
		return err
	}

	// Segment forwarded
	if !resp.IsSuccess() {
		respError := resp.Error().(goutils.RestAPIBaseResponse)
		var err error
		if respError.Error != nil {
			err = fmt.Errorf(respError.Error.Detail)
		} else {
			err = fmt.Errorf("status code %d", resp.StatusCode())
		}
		log.WithError(err).WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment-name", segment.Name).
			Debug("Segment forward failed")
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment-name", segment.Name).
		Debug("Segment forwarded")

	return nil
}

// ======================================================================================
// S3 Transport

// s3SegmentSender S3 video segment transmit client implementing SegmentSender
type s3SegmentSender struct {
	goutils.Component
	client  utils.S3Client
	dbConns db.ConnectionManager
}

/*
NewS3SegmentSender define new S3 video segment transmit client

	@param s3Client utils.S3Client - S3 operations client
	@param dbConns db.ConnectionManager - DB connection manager
	@returns new sender instance
*/
func NewS3SegmentSender(
	s3Client utils.S3Client, dbConns db.ConnectionManager,
) (SegmentSender, error) {
	logTags := log.Fields{
		"module":    "forwarder",
		"component": "s3-segment-sender",
	}

	return &s3SegmentSender{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		client:  s3Client,
		dbConns: dbConns,
	}, nil
}

func (s *s3SegmentSender) ForwardSegment(
	ctxt context.Context, segment common.VideoSegmentWithData,
) error {
	logTags := s.GetLogTagsForContext(ctxt)

	dbClient := s.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Mark that the segment is uploaded
	if err := dbClient.MarkLiveStreamSegmentsUploaded(ctxt, []string{segment.ID}); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment-name", segment.Name).
			Error("Unable to mark segment is uploaded")
		dbClient.MarkExternalError(err)
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment-name", segment.Name).
		Debug("Forwarding segment")

	// Parse the segment URL for the S3 info
	// * Bucket name
	// * Object key

	parsedURL, err := url.Parse(segment.URI)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment-name", segment.Name).
			WithField("segment-uri", segment.URI).
			Error("Unable to parse segment target URI")
		dbClient.MarkExternalError(err)
		return err
	}

	targetBucket := parsedURL.Host
	targetObjectKey := parsedURL.Path
	// Remove any leading `/` from object key
	{
		parts := strings.Split(targetObjectKey, "/")
		keep := []string{}
		for _, onePart := range parts {
			if onePart != "" {
				keep = append(keep, onePart)
			}
		}
		targetObjectKey = strings.Join(keep, "/")
	}

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment-name", segment.Name).
		WithField("target-bucket", targetBucket).
		WithField("target-object-key", targetObjectKey).
		Debug("Uploading video segment")

	if err := s.client.PutObject(ctxt, targetBucket, targetObjectKey, segment.Content); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment-name", segment.Name).
			WithField("target-bucket", targetBucket).
			WithField("target-object-key", targetObjectKey).
			Debug("Failed to upload video segment")
		dbClient.MarkExternalError(err)
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment-name", segment.Name).
		WithField("target-bucket", targetBucket).
		WithField("target-object-key", targetObjectKey).
		Debug("Video segment uploaded")

	return nil
}
