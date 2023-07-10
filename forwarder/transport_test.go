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
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/forwarder"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
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

	uut, err := forwarder.NewHTTPSegmentSender(testURL, testClient)
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
			defer r.Body.Close()
			assert.Equal(testContent, payload)

			// Send response
			return httpmock.NewJsonResponse(http.StatusOK, goutils.RestAPIBaseResponse{Success: true})
		},
	)

	// Forward segment
	waitChan := make(chan bool, 1)

	lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Second)
	go func() {
		assert.Nil(uut.ForwardSegment(lclCtxt, testSourceID, testSegment, testContent))
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
