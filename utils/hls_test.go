package utils_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/alwitt/livemix/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHLSPlaylistRead(t *testing.T) {
	assert := assert.New(t)

	// Create test file
	content := `
#EXTM3U

#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:62
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:62.500000,
vid-0.ts
#EXTINF:23.500000,
vid-1.ts

#EXT-X-ENDLIST
`
	utFile := filepath.Join("/tmp", fmt.Sprintf("%s.txt", uuid.NewString()))
	{
		fileHandle, err := os.Create(utFile)
		assert.Nil(err)
		_, err = fileHandle.Write([]byte(content))
		assert.Nil(err)
	}

	// Read it back
	fileURI, readContent, err := utils.ReadPlaylistFile(utFile)
	assert.Nil(err)
	assert.Equal(fmt.Sprintf("file://%s", utFile), fileURI)
	assert.EqualValues(
		[]string{
			"#EXTM3U",
			"#EXT-X-VERSION:3",
			"#EXT-X-TARGETDURATION:62",
			"#EXT-X-MEDIA-SEQUENCE:0",
			"#EXTINF:62.500000,",
			"vid-0.ts",
			"#EXTINF:23.500000,",
			"vid-1.ts",
			"#EXT-X-ENDLIST",
		},
		readContent,
	)
}
