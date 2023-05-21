package utils_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestFileSystemWatcher(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	fsEvents := make(chan utils.FSEvent)

	uut, err := utils.NewFileSystemWatcher(fsEvents)
	assert.Nil(err)

	// Start the daemon loop
	assert.Nil(uut.Start(utCtxt, utCtxt))

	// Case 0: start daemon again
	assert.NotNil(uut.Start(utCtxt, utCtxt))

	// Case 1: no events
	{
		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*25)
		defer lclCancel()
		select {
		case <-lclCtxt.Done():
			break
		case <-fsEvents:
			log.Fatal("No events were expected")
		}
	}

	// Create a dir for testing
	utDIR1 := filepath.Join("/tmp", uuid.NewString())
	assert.Nil(os.MkdirAll(utDIR1, os.ModePerm))

	// Case 2: watch new DIR
	assert.Nil(uut.AddPath(utCtxt, utDIR1))

	// Case 3: create a file
	utFile1 := filepath.Join(utDIR1, fmt.Sprintf("%s.txt", uuid.NewString()))
	{
		fileHandle, err := os.Create(utFile1)
		assert.Nil(err)
		defer fileHandle.Close()
		_, err = fileHandle.WriteString(uuid.NewString())
		assert.Nil(err)

		// Wait for the event
		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*25)
		defer lclCancel()
		select {
		case <-lclCtxt.Done():
			log.Fatal("Expecting an event")
		case event, ok := <-fsEvents:
			assert.True(ok)
			assert.Equal(utFile1, event.Name)
			assert.True(event.Has(fsnotify.Create))
		}
	}

	// Create a dir for testing
	utDIR2 := filepath.Join("/tmp", uuid.NewString())
	assert.Nil(os.MkdirAll(utDIR2, os.ModePerm))

	// Case 4: watch new DIR
	assert.Nil(uut.AddPath(utCtxt, utDIR2))

	// Case 5: move a file
	utFile2 := filepath.Join(utDIR2, fmt.Sprintf("%s.txt", uuid.NewString()))
	{
		assert.Nil(os.Rename(utFile1, utFile2))

		// Wait for the event
		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*25)
		defer lclCancel()
		select {
		case <-lclCtxt.Done():
			log.Fatal("Expecting an event")
		case event, ok := <-fsEvents:
			assert.True(ok)
			assert.Equal(utFile2, event.Name)
			assert.True(event.Has(fsnotify.Create))
		}
	}

	assert.Nil(uut.Stop(utCtxt))
}
