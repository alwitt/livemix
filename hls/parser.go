package hls

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alwitt/goutils"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
)

// PlaylistParser HLS playlist parser
type PlaylistParser interface {
	/*
		ParsePlaylist parse a HLS playlist to get the playlist properties, and the associated segments,

		The playlist is expected to be already parsed into a list of strings. The expected structure
		of a HLS playlist

		#EXTM3U
		#EXT-X-VERSION:6
		#EXT-X-TARGETDURATION:4
		#EXT-X-MEDIA-SEQUENCE:0
		#EXTINF:4.000000,
		vid-1684541470.ts
		#EXT-X-ENDLIST

			@param ctxt context.Context - execution context
			@param content []string - HLS playlist content
			@param timestamp time.Time - When the playlist is generated
			@param playlistName string - alias name assigned to associated with the playlist
			@param segmentBaseURI string - base URI of individual segments
			@returns parsed playlist
	*/
	ParsePlaylist(
		ctxt context.Context,
		content []string,
		timestamp time.Time,
		playlistName string,
		segmentBaseURI string,
	) (Playlist, error)
}

/*
NewPlaylistParser define new playlist parser

	@returns parser
*/
func NewPlaylistParser() PlaylistParser {
	return playlistParserImpl{
		Component: goutils.Component{
			LogTags: log.Fields{"module": "hls", "component": "playlist-parser"},
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		validator: validator.New(),
	}
}

// playlistParserImpl implements PlaylistParser
type playlistParserImpl struct {
	goutils.Component
	validator *validator.Validate
}

// buildSegmentURI construct a complete segment URI starting with a base URI and segment name
func buildSegmentURI(baseURI *url.URL, segmentName string) string {
	objectPath := filepath.Join(baseURI.EscapedPath(), segmentName)
	// Build new URI
	newURI := *baseURI
	newURI.Path = objectPath
	return newURI.String()
}

func (p playlistParserImpl) ParsePlaylist(
	ctxt context.Context,
	content []string,
	timestamp time.Time,
	playlistName string,
	segmentBaseURI string,
) (Playlist, error) {
	const (
		hlsParseIdle int = iota
		hlsParseReadType
		hlsParseReadSegmentDuration
		hlsParseReadSegmentFileName
		hlsParseReadAllSegments
	)

	logTags := p.GetLogTagsForContext(ctxt)

	playlist := Playlist{Name: playlistName, CreatedAt: timestamp}

	parsedBaseSegmentURI, err := url.Parse(segmentBaseURI)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment-base-uri", segmentBaseURI).
			Error("Base segment URI parse failure")
		return playlist, err
	}

	// Parse the playlist contents
	var oneSegment Segment

	parseState := hlsParseIdle
	for _, oneLine := range content {
		switch parseState {

		case hlsParseIdle:
			if oneLine == "#EXTM3U" {
				parseState = hlsParseReadType
			}

		case hlsParseReadType:
			switch {
			case strings.HasPrefix(oneLine, "#EXT-X-VERSION"):
				// Get version
				parts := strings.Split(oneLine, ":")
				if len(parts) == 2 {
					if val, err := strconv.Atoi(parts[1]); err == nil {
						playlist.Version = val
					}
				}
			case strings.HasPrefix(oneLine, "#EXT-X-TARGETDURATION"):
				// Get target duration
				parts := strings.Split(oneLine, ":")
				if len(parts) == 2 {
					if val, err := strconv.ParseFloat(parts[1], 32); err == nil {
						playlist.TargetSegDuration = val
					}
				}
			case strings.HasPrefix(oneLine, "#EXTINF"):
				// First segment
				actualDuration := 0.0
				n, err := fmt.Sscanf(oneLine, "#EXTINF:%f,", &actualDuration)
				if err == nil && n == 1 {
					oneSegment = Segment{}
					oneSegment.Length = actualDuration
					parseState = hlsParseReadSegmentDuration
				}
			default:
				break
			}

		case hlsParseReadSegmentDuration:
			if strings.HasPrefix(oneLine, "#") {
				err := fmt.Errorf("received another tag instead of a segment filename")
				logTags["current"] = oneLine
				log.WithError(err).WithFields(logTags).Error("HLS playlist parse failure")
				return playlist, err
			}
			// Expect the entire line is segment filename
			oneSegment.Name = oneLine
			parseState = hlsParseReadSegmentFileName
			playlist.Segments = append(playlist.Segments, oneSegment)

		case hlsParseReadSegmentFileName:
			// Process another segment
			if strings.HasPrefix(oneLine, "#EXTINF") {
				actualDuration := 0.0
				n, err := fmt.Sscanf(oneLine, "#EXTINF:%f,", &actualDuration)
				if err == nil && n == 1 {
					oneSegment = Segment{}
					oneSegment.Length = actualDuration
					parseState = hlsParseReadSegmentDuration
				}
			} else if strings.HasPrefix(oneLine, "#EXT-X-ENDLIST") {
				// End of segments
				parseState = hlsParseReadAllSegments
			} else {
				err := fmt.Errorf("segment entry followed by unexpected tag")
				logTags["current"] = oneLine
				log.WithError(err).WithFields(logTags).Error("HLS playlist parse failure")
				return playlist, err
			}

		default:
			err := fmt.Errorf("parsing state broke")
			logTags["current"] = oneLine
			log.WithError(err).WithFields(logTags).Error("HLS playlist parse failure")
			return playlist, err
		}

		if parseState == hlsParseReadAllSegments {
			break
		}
	}

	if parseState != hlsParseReadAllSegments && parseState != hlsParseReadSegmentFileName {
		err := fmt.Errorf("playlist has unexpected format")
		log.WithError(err).WithFields(logTags).Error("HLS playlist parse failure")
		return playlist, err
	}

	// Compute the timing information of each segment based on when the HLS playlist is generated
	currentTime := timestamp
	for itr := len(playlist.Segments) - 1; itr >= 0; itr-- {
		playlist.Segments[itr].EndTime = currentTime
		currentTime = currentTime.Add(-playlist.Segments[itr].GetDuration())
		playlist.Segments[itr].StartTime = currentTime
		// Update the segment URI
		playlist.Segments[itr].URI = buildSegmentURI(parsedBaseSegmentURI, playlist.Segments[itr].Name)
	}

	// Validate the complete playlist
	if err := p.validator.Struct(&playlist); err != nil {
		log.WithError(err).WithFields(logTags).Error("HLS playlist is invalid")
		return playlist, err
	}

	return playlist, nil
}
