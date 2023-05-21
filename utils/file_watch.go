package utils

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"sync/atomic"

	"github.com/alwitt/goutils"
	"github.com/apex/log"
	"github.com/fsnotify/fsnotify"
)

// FileSystemWatcher monitor a DIR for file system changes
type FileSystemWatcher interface {
	/*
		Start begin the file system watch loop

			@param ctxt context.Context - execution context
			@param runtimeCtxt context.Context - runtime context for any background tasks
	*/
	Start(ctxt, runtimeCtxt context.Context) error

	/*
		Stop end the file system watch loop

			@param ctxt context.Context - execution context
	*/
	Stop(ctxt context.Context) error

	/*
		AddPath add path to list of path to watch

			@param ctxt context.Context - execution context
			@param newPath string - new path to watch
	*/
	AddPath(ctxt context.Context, newPath string) error

	/*
		RemovePath remove path from list of watched path

			@param ctxt context.Context - execution context
			@param path string - new path to watch
	*/
	RemovePath(ctxt context.Context, path string) error
}

/*
NewFileSystemWatcher define new FileSystemWatcher

	@param eventChan chan FSEvent - the channel to return file system events
	@returns watcher
*/
func NewFileSystemWatcher(eventChan chan FSEvent) (FileSystemWatcher, error) {
	logTags := log.Fields{"module": "utils", "component": "file-system-watcher"}

	// Define a watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define 'fsnotify' watcher")
		return nil, err
	}

	tmpCtxt, tmpCtxtCancel := context.WithCancel(context.Background())

	return &fileSystemWatchImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		eventChan:      eventChan,
		watcher:        watcher,
		watcherRunning: 0,
		watcherContext: tmpCtxt,
		contextCancel:  tmpCtxtCancel,
		wg:             nil,
	}, nil
}

// fileSystemWatchImpl implement FileSystemWatcher
type fileSystemWatchImpl struct {
	goutils.Component
	eventChan      chan FSEvent
	watcher        *fsnotify.Watcher
	watcherRunning uint32
	wg             *sync.WaitGroup
	watcherContext context.Context
	contextCancel  func()
}

// FSEvent wrapper around `fsnotify.Event` with metadata
type FSEvent struct {
	// The original event
	fsnotify.Event
	// Meta file metadata
	Meta fs.FileInfo
}

func (w *fileSystemWatchImpl) Start(ctxt, runtimeCtxt context.Context) error {
	logTags := w.GetLogTagsForContext(ctxt)

	if !atomic.CompareAndSwapUint32(&w.watcherRunning, 0, 1) {
		err := fmt.Errorf("file system watcher already running")
		return err
	}

	// Define the actual watcher context
	watcherContext, contextCancel := context.WithCancel(runtimeCtxt)
	w.watcherContext = watcherContext
	w.contextCancel = contextCancel

	w.wg = &sync.WaitGroup{}
	w.wg.Add(1)

	// File change processing
	go func() {
		defer w.wg.Done()

		log.WithFields(logTags).Info("Starting file system watcher")
		defer log.WithFields(logTags).Info("File system watcher stopped")

		for {
			select {
			case <-watcherContext.Done():
				log.WithFields(logTags).Info("Stopping file system watcher")
				return
			case event, ok := <-w.watcher.Events:
				if !ok {
					panic("File system watcher event queue returned error")
				}
				// Process event
				if event.Has(fsnotify.Create) {
					// Pass it on
					log.
						WithFields(logTags).
						WithField("path", event.Name).
						WithField("op", event.Op.String()).
						Debug("Observed new relevant event")
					stat, err := os.Stat(event.Name)
					if err == nil {
						w.eventChan <- FSEvent{Event: event, Meta: stat}
					} else {
						log.
							WithError(err).
							WithFields(logTags).
							WithField("path", event.Name).
							WithField("op", event.Op.String()).
							Error("Unable to `stat` file")
					}
				}
			case err, ok := <-w.watcher.Errors:
				if !ok {
					panic("File system error event queue returned error")
				}
				log.WithError(err).WithFields(logTags).Error("File system monitor returned error")
			}
		}
	}()

	return nil
}

func (w *fileSystemWatchImpl) Stop(ctxt context.Context) error {
	w.contextCancel()
	w.wg.Wait()
	_ = atomic.SwapUint32(&w.watcherRunning, 0)
	return nil
}

func (w *fileSystemWatchImpl) AddPath(ctxt context.Context, newPath string) error {
	return w.watcher.Add(newPath)
}

func (w *fileSystemWatchImpl) RemovePath(ctxt context.Context, path string) error {
	return w.watcher.Remove(path)
}
