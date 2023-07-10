package forwarder

import (
	"context"
	"fmt"
	"net/url"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
	"github.com/go-resty/resty/v2"
)

// SegmentSender video segment transmit client
type SegmentSender interface {
	/*
		ForwardSegment forward a video segment to a receiver

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param segmentInfo hls.Segment - video segment metadata
			@param content []byte - video segment content
	*/
	ForwardSegment(
		ctxt context.Context, sourceID string, segmentInfo hls.Segment, content []byte,
	) error
}

// httpSegmentSender HTTP video segment transmit client implementing SegmentSender
type httpSegmentSender struct {
	goutils.Component
	receiverURI *url.URL
	client      *resty.Client
}

/*
NewHTTPSegmentSender define new HTP video segment transmit client

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
	ctxt context.Context,
	sourceID string,
	segmentInfo hls.Segment,
	content []byte,
) error {
	logTags := s.GetLogTagsForContext(ctxt)

	log.
		WithFields(logTags).
		WithField("source-id", sourceID).
		WithField("segment-name", segmentInfo.Name).
		Debug("Forwarding segment")

	// Make request
	resp, err := s.client.R().
		// Set request header for segment reception
		SetHeader(ipc.HTTPSegmentForwardHeaderSourceID, sourceID).
		SetHeader(ipc.HTTPSegmentForwardHeaderName, segmentInfo.Name).
		SetHeader(ipc.HTTPSegmentForwardHeaderStartTS, fmt.Sprintf("%d", segmentInfo.StartTime.Unix())).
		SetHeader(ipc.HTTPSegmentForwardHeaderLength, fmt.Sprintf("%d", int(segmentInfo.Length*1000))).
		SetHeader(ipc.HTTPSegmentForwardHeaderSegURI, segmentInfo.URI).
		// Set request payload
		SetBody(content).
		// Setup error parsing
		SetError(goutils.RestAPIBaseResponse{}).
		Post(s.receiverURI.String())

	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("segment-name", segmentInfo.Name).
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
			WithField("source-id", sourceID).
			WithField("segment-name", segmentInfo.Name).
			Debug("Segment forward failed")
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", sourceID).
		WithField("segment-name", segmentInfo.Name).
		Debug("Segment forwarded")

	return nil
}
