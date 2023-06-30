package hls_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alwitt/livemix/hls"
	"github.com/stretchr/testify/assert"
)

func TestPlaylistToString(t *testing.T) {
	assert := assert.New(t)

	utCtxt := context.Background()
	currentTime := time.Now().UTC()

	// Build a playlist
	source := []string{
		"#EXTM3U",
		"#EXT-X-VERSION:3",
		"#EXT-X-TARGETDURATION:62.000000",
		"#EXT-X-MEDIA-SEQUENCE:44",
		"#EXTINF:62.500000,",
		"vid-0.ts",
		"#EXTINF:23.500000,",
		"vid-1.ts",
		"#EXT-X-ENDLIST",
	}
	sourceLinked := ""
	{
		builder := strings.Builder{}
		for _, oneLine := range source {
			builder.WriteString(fmt.Sprintf("%s\n", oneLine))
		}
		sourceLinked = builder.String()
	}
	parser := hls.NewPlaylistParser()
	parsed, err := parser.ParsePlaylist(utCtxt, source, currentTime, "testing", "file:///vid")
	assert.Nil(err)

	// Add media-sequence separately
	sequence := 44
	parsed.MediaSequenceVal = &sequence

	// Convert the playlist back to string
	toString, err := parsed.String(false)
	assert.Nil(err)
	assert.Equal(sourceLinked, toString)
}
