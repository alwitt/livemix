package utils_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestReadingSegmentFromFile(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	uut, err := utils.NewSegmentReader(utCtxt, 2)
	assert.Nil(err)

	// Define test file
	testFile := fmt.Sprintf("/tmp/%s.txt", uuid.NewString())
	var testContent []byte
	{
		builder := strings.Builder{}
		for itr := 0; itr < 4; itr++ {
			builder.WriteString(uuid.NewString())
		}
		testContent = []byte(builder.String())
	}
	{
		file, err := os.Create(testFile)
		assert.Nil(err)
		_, err = file.Write(testContent)
		assert.Nil(err)
	}

	contentRx := make(chan []byte)
	passBackID := ""
	processRead := func(_ context.Context, segmentID string, content []byte) error {
		contentRx <- content
		passBackID = segmentID
		return nil
	}

	// Read the file
	testID := uuid.NewString()
	testFileURL, err := url.Parse(fmt.Sprintf("file://%s", testFile))
	assert.Nil(err)
	assert.Nil(uut.ReadSegment(utCtxt, testID, testFileURL, processRead))

	// Wait for process complete
	{
		waitCtxt, cancel := context.WithTimeout(utCtxt, time.Millisecond*100)
		defer cancel()
		select {
		case <-waitCtxt.Done():
			assert.True(false, "file read timed out")
		case readContent, ok := <-contentRx:
			assert.True(ok)
			assert.Equal(testID, passBackID)
			assert.EqualValues(testContent, readContent)
		}
	}

	assert.Nil(uut.Stop(utCtxt))
}
