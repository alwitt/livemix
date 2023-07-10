package ipc

const (
	// HTTPSegmentForwardHeaderSourceID standard HTTP header of source ID parameter used
	// when forwarding video segments from edge node to system control node
	HTTPSegmentForwardHeaderSourceID = "Source-ID"

	// HTTPSegmentForwardHeaderName standard HTTP header of segment name parameter used
	// when forwarding video segments from edge node to system control node
	HTTPSegmentForwardHeaderName = "Segment-Name"

	// HTTPSegmentForwardHeaderStartTS standard HTTP header of segment start timestamp
	// parameter used when forwarding video segments from edge node to system control node
	HTTPSegmentForwardHeaderStartTS = "Segment-Start-TS"

	// HTTPSegmentForwardHeaderLength standard HTTP header of segment length in msec
	// parameter used when forwarding video segments from edge node to system control node
	HTTPSegmentForwardHeaderLength = "Segment-Length-MSec"

	// HTTPSegmentForwardHeaderSegURI standard HTTP header of segment URI parameter used
	// when forwarding video segments from edge node to system control node
	HTTPSegmentForwardHeaderSegURI = "Segment-URI"
)
