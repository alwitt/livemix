package hls_test

import (
	"context"
	"testing"
	"time"

	"github.com/alwitt/livemix/hls"
	"github.com/stretchr/testify/assert"
)

func TestHLSParsing(t *testing.T) {
	assert := assert.New(t)

	uut := hls.NewPlaylistParser()

	utCtxt, ctxtCancel := context.WithCancel(context.Background())
	defer ctxtCancel()

	currentTime := time.Now().UTC()

	// Case 1: blank file
	{
		_, err := uut.ParsePlaylist(utCtxt, []string{}, currentTime, "testing", "file:///vid")
		assert.NotNil(err)
	}

	// Case 2: Parse wrong file
	{
		testfile := []string{
			"hello world",
			"#EXT-X-VERSION:3",
		}
		_, err := uut.ParsePlaylist(utCtxt, testfile, currentTime, "testing", "file:///vid")
		assert.NotNil(err)
	}

	// Case 3: Parse file with no header or segment
	{
		testfile := []string{
			"#EXTM3U",
			"#EXT-X-ENDLIST",
		}
		_, err := uut.ParsePlaylist(utCtxt, testfile, currentTime, "testing", "file:///vid")
		assert.NotNil(err)
	}

	// Case 4: Parse file with no segment
	{
		testfile := []string{
			"#EXTM3U",
			"#EXT-X-VERSION:3",
			"#EXT-X-TARGETDURATION:62",
			"#EXT-X-MEDIA-SEQUENCE:0",
			"#EXT-X-ENDLIST",
		}
		_, err := uut.ParsePlaylist(utCtxt, testfile, currentTime, "testing", "file:///vid")
		assert.NotNil(err)
	}

	// Case 5: Parse file with segment but missing required header
	{
		testfile := []string{
			"#EXTM3U",
			"#EXT-X-VERSION:3",
			"#EXTINF:62.500000,",
			"vid-0.ts",
			"#EXT-X-ENDLIST",
		}
		_, err := uut.ParsePlaylist(utCtxt, testfile, currentTime, "testing", "file:///vid")
		assert.NotNil(err)
	}

	// Case 6: Parse with segment but no end
	{
		testfile := []string{
			"#EXTM3U",
			"#EXT-X-VERSION:3",
			"#EXT-X-TARGETDURATION:62",
			"#EXT-X-MEDIA-SEQUENCE:0",
			"#EXTINF:62.500000,",
			"vid-0.ts",
		}
		parsed, err := uut.ParsePlaylist(utCtxt, testfile, currentTime, "testing", "file:///vid")
		assert.Nil(err)
		assert.Equal("testing", parsed.Name)
		assert.Equal(3, parsed.Version)
		assert.InDelta(62.0, parsed.TargetSegDuration, 1e-6)
		assert.Len(parsed.Segments, 1)
		assert.Equal("vid-0.ts", parsed.Segments[0].Name)
		assert.Equal("file:///vid/vid-0.ts", parsed.Segments[0].URI)
		assert.InDelta(62.5, parsed.Segments[0].Length, 1e-6)
	}

	// Case 7: Everything accounted for
	{
		testfile := []string{
			"#EXTM3U",
			"#EXT-X-VERSION:3",
			"#EXT-X-TARGETDURATION:62",
			"#EXT-X-MEDIA-SEQUENCE:0",
			"#EXTINF:62.500000,",
			"vid-0.ts",
			"#EXT-X-ENDLIST",
		}
		parsed, err := uut.ParsePlaylist(utCtxt, testfile, currentTime, "testing", "file:///vid")
		assert.Nilf(err, "Got %s", err)
		assert.Equal("testing", parsed.Name)
		assert.Equal(3, parsed.Version)
		assert.InDelta(62.0, parsed.TargetSegDuration, 1e-6)
		assert.Len(parsed.Segments, 1)
		assert.Equal("vid-0.ts", parsed.Segments[0].Name)
		assert.Equal("file:///vid/vid-0.ts", parsed.Segments[0].URI)
		assert.InDelta(62.5, parsed.Segments[0].Length, 1e-6)
	}

	// Case 8: Multiple segments
	{
		testfile := []string{
			"#EXTM3U",
			"#EXT-X-VERSION:3",
			"#EXT-X-TARGETDURATION:62",
			"#EXT-X-MEDIA-SEQUENCE:0",
			"#EXTINF:62.500000,",
			"vid-0.ts",
			"#EXTINF:23.500000,",
			"vid-1.ts",
			"#EXT-X-ENDLIST",
		}
		parsed, err := uut.ParsePlaylist(utCtxt, testfile, currentTime, "testing", "file:///vid")
		assert.Nilf(err, "Got %s", err)
		assert.Equal("testing", parsed.Name)
		assert.Equal(3, parsed.Version)
		assert.InDelta(62.0, parsed.TargetSegDuration, 1e-6)
		assert.Len(parsed.Segments, 2)
		assert.Equal("vid-0.ts", parsed.Segments[0].Name)
		assert.Equal("file:///vid/vid-0.ts", parsed.Segments[0].URI)
		assert.InDelta(62.5, parsed.Segments[0].Length, 1e-6)
		assert.Equal("vid-1.ts", parsed.Segments[1].Name)
		assert.Equal("file:///vid/vid-1.ts", parsed.Segments[1].URI)
		assert.InDelta(23.5, parsed.Segments[1].Length, 1e-6)
		// Verify the start and stop time
		assert.Equal(currentTime, parsed.Segments[1].EndTime)
		assert.Equal(currentTime.Add(-parsed.Segments[1].GetDuration()), parsed.Segments[1].StartTime)
		t := currentTime.Add(-parsed.Segments[1].GetDuration())
		assert.Equal(t, parsed.Segments[0].EndTime)
		assert.Equal(t.Add(-parsed.Segments[0].GetDuration()), parsed.Segments[0].StartTime)
	}
}
