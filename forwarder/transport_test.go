package forwarder_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/forwarder"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHTTPSegmentSender(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	testClient := resty.New()
	// Install with mock
	httpmock.ActivateNonDefault(testClient.GetClient())

	testURL, err := url.Parse("http://ut.testing.dev/new-segment")
	assert.Nil(err)

	uut, err := forwarder.NewHTTPSegmentSender(utCtxt, testURL, "request-id", testClient, nil, nil)
	assert.Nil(err)

	timestamp := time.Now().UTC()
	testSourceID := uuid.NewString()
	testSegment := hls.Segment{
		Name:      uuid.NewString(),
		StartTime: timestamp,
		EndTime:   timestamp.Add(time.Second * 6),
		Length:    6.0,
		URI:       fmt.Sprintf("file:///tmp/%s.ts", uuid.NewString()),
	}
	testContent := []byte(uuid.NewString())

	// Prepare mock
	httpmock.RegisterResponder(
		"POST",
		testURL.String(),
		func(r *http.Request) (*http.Response, error) {
			// Verify the headers
			assert.NotEmpty(r.Header.Get("request-id"))
			assert.Equal(testSourceID, r.Header.Get(ipc.HTTPSegmentForwardHeaderSourceID))
			assert.Equal(testSegment.Name, r.Header.Get(ipc.HTTPSegmentForwardHeaderName))
			assert.Equal(
				fmt.Sprintf("%d", testSegment.StartTime.Unix()),
				r.Header.Get(ipc.HTTPSegmentForwardHeaderStartTS),
			)
			assert.Equal("6000", r.Header.Get(ipc.HTTPSegmentForwardHeaderLength))
			assert.Equal(testSegment.URI, r.Header.Get(ipc.HTTPSegmentForwardHeaderSegURI))

			// Verify payload
			payload, err := io.ReadAll(r.Body)
			assert.Nil(err)
			defer func() {
				assert.Nil(r.Body.Close())
			}()
			assert.Equal(testContent, payload)

			// Send response
			return httpmock.NewJsonResponse(http.StatusOK, goutils.RestAPIBaseResponse{Success: true})
		},
	)

	// Forward segment
	waitChan := make(chan bool, 1)

	lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Second)
	go func() {
		assert.Nil(uut.ForwardSegment(
			lclCtxt,
			common.VideoSegmentWithData{
				VideoSegment: common.VideoSegment{
					ID:       uuid.NewString(),
					SourceID: testSourceID,
					Segment:  testSegment,
				},
				Content: testContent,
			},
		))
		waitChan <- true
	}()

	select {
	case <-lclCtxt.Done():
		assert.False(true, "request timed out")
	case <-waitChan:
		break
	}
	lclCancel()
}

func TestS3SegmentSender(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	mockS3 := mocks.NewS3Client(t)
	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()

	uut, err := forwarder.NewS3SegmentSender(utCtxt, mockS3, mockSQL, nil, nil)
	assert.Nil(err)

	testSegmentID := uuid.NewString()
	testBucket := uuid.NewString()
	testObjectKey := fmt.Sprintf("segments/%s.ts", testSegmentID)
	testSegment := common.VideoSegmentWithData{
		VideoSegment: common.VideoSegment{
			ID:       testSegmentID,
			SourceID: uuid.NewString(),
			Segment: hls.Segment{
				Name: uuid.NewString(),
				URI:  fmt.Sprintf("s3://%s/%s", testBucket, testObjectKey),
			},
		},
		Content: []byte(uuid.NewString()),
	}

	// Prepare mock
	mockDB.On(
		"MarkLiveStreamSegmentsUploaded",
		mock.AnythingOfType("context.backgroundCtx"),
		[]string{testSegmentID},
	).Return(nil).Once()
	mockS3.On(
		"PutObject",
		mock.AnythingOfType("context.backgroundCtx"),
		testBucket,
		testObjectKey,
		testSegment.Content,
	).Return(nil).Once()

	// Make the request
	assert.Nil(uut.ForwardSegment(utCtxt, testSegment))
}
