package utils

import (
	"bufio"
	"net/url"
	"os"
	"path/filepath"
)

/*
ReadPlaylistFile parse a HLS playlist file

	@param playlistFile string - playlist file name
	@returns playlist file URI
	@returns content of the file
*/
func ReadPlaylistFile(playlistFile string) (uri string, content []string, err error) {
	absPath, err := filepath.Abs(playlistFile)
	if err != nil {
		return "", nil, err
	}

	fileURI := url.URL{Scheme: "file", Path: absPath}

	// Parse the file
	fileHandle, err := os.Open(absPath)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = fileHandle.Close() }()

	scanner := bufio.NewScanner(fileHandle)
	for scanner.Scan() {
		oneLine := scanner.Text()
		if len(oneLine) > 0 && oneLine[0] != '\n' {
			content = append(content, oneLine)
		}
	}

	if scanner.Err() != nil {
		return "", nil, scanner.Err()
	}

	return fileURI.String(), content, nil
}
