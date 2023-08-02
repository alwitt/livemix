package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/control"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/vod"
	"github.com/gorilla/mux"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

/*
BuildMetricsCollectionServer create server to host metrics collection endpoint

	@param httpCfg common.HTTPServerConfig - HTTP server configuration
	@param metricsCollector goutils.MetricsCollector - metrics collector
	@param collectionEndpoint string - endpoint to expose the metrics on
	@param maxRESTRequests int - max number fo parallel requests to support
	@returns HTTP server instance
*/
func BuildMetricsCollectionServer(
	httpCfg common.HTTPServerConfig,
	metricsCollector goutils.MetricsCollector,
	collectionEndpoint string,
	maxRESTRequests int,
) (*http.Server, error) {
	router := mux.NewRouter()
	metricsCollector.ExposeCollectionEndpoint(router, collectionEndpoint, maxRESTRequests)

	serverListen := fmt.Sprintf(
		"%s:%d", httpCfg.ListenOn, httpCfg.Port,
	)
	httpSrv := &http.Server{
		Addr:         serverListen,
		WriteTimeout: time.Second * time.Duration(httpCfg.Timeouts.WriteTimeout),
		ReadTimeout:  time.Second * time.Duration(httpCfg.Timeouts.ReadTimeout),
		IdleTimeout:  time.Second * time.Duration(httpCfg.Timeouts.IdleTimeout),
		Handler:      h2c.NewHandler(router, &http2.Server{}),
	}

	return httpSrv, nil
}

// ====================================================================================
// System Manage Server

/*
BuildSystemManagementServer create system management API server

	@param httpCfg common.APIServerConfig - HTTP server configuration
	@param manager control.SystemManager - core system manager
	@param metrics goutils.HTTPRequestMetricHelper - metric collection agent
	@returns HTTP server instance
*/
func BuildSystemManagementServer(
	httpCfg common.APIServerConfig,
	manager control.SystemManager,
	metrics goutils.HTTPRequestMetricHelper,
) (*http.Server, error) {
	httpHandler, err := NewSystemManagerHandler(manager, httpCfg.APIs.RequestLogging, metrics)
	if err != nil {
		return nil, err
	}

	livenessHTTPHandler, err := NewSystemManagerLivenessHandler(manager, httpCfg.APIs.RequestLogging)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter()
	mainRouter := registerPathPrefix(router, httpCfg.APIs.Endpoint.PathPrefix, nil)
	livenessRounter := registerPathPrefix(mainRouter, "/liveness", nil)
	v1Router := registerPathPrefix(mainRouter, "/v1", nil)

	// --------------------------------------------------------------------------------
	// Health check

	_ = registerPathPrefix(livenessRounter, "/alive", map[string]http.HandlerFunc{
		"get": livenessHTTPHandler.AliveHandler(),
	})
	_ = registerPathPrefix(livenessRounter, "/ready", map[string]http.HandlerFunc{
		"get": livenessHTTPHandler.ReadyHandler(),
	})

	// --------------------------------------------------------------------------------
	// Video source
	videoSourceRouter := registerPathPrefix(v1Router, "/source", map[string]http.HandlerFunc{
		"post": httpHandler.DefineNewVideoSourceHandler(),
		"get":  httpHandler.ListVideoSourcesHandler(),
	})

	perSourceRouter := registerPathPrefix(
		videoSourceRouter, "/{sourceID}", map[string]http.HandlerFunc{
			"get":    httpHandler.GetVideoSourceHandler(),
			"delete": httpHandler.DeleteVideoSourceHandler(),
		},
	)

	_ = registerPathPrefix(perSourceRouter, "/name", map[string]http.HandlerFunc{
		"put": httpHandler.UpdateVideoSourceNameHandler(),
	})

	_ = registerPathPrefix(perSourceRouter, "/streaming", map[string]http.HandlerFunc{
		"put": httpHandler.ChangeSourceStreamingStateHandler(),
	})

	_ = registerPathPrefix(perSourceRouter, "/recording", map[string]http.HandlerFunc{
		"post": httpHandler.StartNewRecordingHandler(),
		"get":  httpHandler.ListRecordingsOfSourceHandler(),
	})

	// --------------------------------------------------------------------------------
	// Video recording
	recordingRouter := registerPathPrefix(v1Router, "/recording", map[string]http.HandlerFunc{
		"get": httpHandler.ListRecordingsHandler(),
	})

	perRecordingRouter := registerPathPrefix(
		recordingRouter, "/{recordingID}", map[string]http.HandlerFunc{
			"get":    httpHandler.GetRecordingHandler(),
			"delete": httpHandler.DeleteRecordingHandler(),
		},
	)

	_ = registerPathPrefix(perRecordingRouter, "/stop", map[string]http.HandlerFunc{
		"post": httpHandler.StopRecordingHandler(),
	})

	_ = registerPathPrefix(perRecordingRouter, "/segment", map[string]http.HandlerFunc{
		"get": httpHandler.ListSegmentsOfRecordingHandler(),
	})

	// --------------------------------------------------------------------------------
	// Middleware

	v1Router.Use(func(next http.Handler) http.Handler {
		return httpHandler.LoggingMiddleware(next.ServeHTTP)
	})
	livenessRounter.Use(func(next http.Handler) http.Handler {
		return livenessHTTPHandler.LoggingMiddleware(next.ServeHTTP)
	})

	// --------------------------------------------------------------------------------
	// HTTP Server

	serverListen := fmt.Sprintf(
		"%s:%d", httpCfg.Server.ListenOn, httpCfg.Server.Port,
	)
	httpSrv := &http.Server{
		Addr:         serverListen,
		WriteTimeout: time.Second * time.Duration(httpCfg.Server.Timeouts.WriteTimeout),
		ReadTimeout:  time.Second * time.Duration(httpCfg.Server.Timeouts.ReadTimeout),
		IdleTimeout:  time.Second * time.Duration(httpCfg.Server.Timeouts.IdleTimeout),
		Handler:      h2c.NewHandler(router, &http2.Server{}),
	}

	return httpSrv, nil
}

// ====================================================================================
// Playlist Receiver Server

/*
BuildPlaylistReceiverServer create edge node playlist receive server

	@param parentCtxt context.Context - REST handler parent context
	@param httpCfg common.APIServerConfig - HTTP server configuration
	@param forwardCB PlaylistForwardCB - callback to forward newly received playlists
	@param metrics goutils.HTTPRequestMetricHelper - metric collection agent
	@returns HTTP server instance
*/
func BuildPlaylistReceiverServer(
	parentCtxt context.Context,
	httpCfg common.APIServerConfig,
	forwardCB PlaylistForwardCB,
	metrics goutils.HTTPRequestMetricHelper,
) (*http.Server, error) {
	httpHandler, err := NewPlaylistReceiveHandler(
		parentCtxt, hls.NewPlaylistParser(), forwardCB, httpCfg.APIs.RequestLogging, metrics,
	)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter()
	mainRouter := registerPathPrefix(router, httpCfg.APIs.Endpoint.PathPrefix, nil)
	v1Router := registerPathPrefix(mainRouter, "/v1", nil)

	// --------------------------------------------------------------------------------
	// Playlist input
	_ = registerPathPrefix(v1Router, "/playlist", map[string]http.HandlerFunc{
		"post": httpHandler.NewPlaylistHandler(),
	})

	// --------------------------------------------------------------------------------
	// Middleware

	router.Use(func(next http.Handler) http.Handler {
		return httpHandler.LoggingMiddleware(next.ServeHTTP)
	})

	// --------------------------------------------------------------------------------
	// HTTP Server

	serverListen := fmt.Sprintf(
		"%s:%d", httpCfg.Server.ListenOn, httpCfg.Server.Port,
	)
	httpSrv := &http.Server{
		Addr:         serverListen,
		WriteTimeout: time.Second * time.Duration(httpCfg.Server.Timeouts.WriteTimeout),
		ReadTimeout:  time.Second * time.Duration(httpCfg.Server.Timeouts.ReadTimeout),
		IdleTimeout:  time.Second * time.Duration(httpCfg.Server.Timeouts.IdleTimeout),
		Handler:      h2c.NewHandler(router, &http2.Server{}),
	}

	return httpSrv, nil
}

// ====================================================================================
// Segment Receiver Server

/*
BuildCentralVODServer create control node VOD server. It is responsible for
* segment receive for video stream proxy
* VOD server

	@param parentCtxt context.Context - REST handler parent context
	@param httpCfg common.APIServerConfig - HTTP server configuration
	@param forwardCB VideoSegmentForwardCB - callback to forward newly received segments
	@param dbConns db.ConnectionManager - DB connection manager
	@param manager vod.PlaylistManager - video playlist manager
	@param metrics goutils.HTTPRequestMetricHelper - metric collection agent
	@returns HTTP server instance
*/
func BuildCentralVODServer(
	parentCtxt context.Context,
	httpCfg common.APIServerConfig,
	forwardCB VideoSegmentForwardCB,
	dbConns db.ConnectionManager,
	manager vod.PlaylistManager,
	metrics goutils.HTTPRequestMetricHelper,
) (*http.Server, error) {
	segmentRXHandler, err := NewSegmentReceiveHandler(
		parentCtxt, forwardCB, httpCfg.APIs.RequestLogging, metrics,
	)
	if err != nil {
		return nil, err
	}
	vodHandler, err := NewVODHandler(
		dbConns, manager, httpCfg.APIs.RequestLogging, metrics,
	)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter()
	mainRouter := registerPathPrefix(router, httpCfg.APIs.Endpoint.PathPrefix, nil)
	v1Router := registerPathPrefix(mainRouter, "/v1", nil)

	// --------------------------------------------------------------------------------
	// Segment input
	_ = registerPathPrefix(v1Router, "/new-segment", map[string]http.HandlerFunc{
		"post": segmentRXHandler.NewSegmentHandler(),
	})

	// --------------------------------------------------------------------------------
	// VOD endpoint
	_ = registerPathPrefix(
		v1Router, "/vod/live/{videoSourceID}/{fileName}", map[string]http.HandlerFunc{
			"get": vodHandler.GetLiveStreamVideoFilesHandler(),
		},
	)

	_ = registerPathPrefix(
		v1Router, "/vod/recording/{recordingID}/{fileName}", map[string]http.HandlerFunc{
			"get": vodHandler.GetRecordingVideoFilesHandler(),
		},
	)

	// --------------------------------------------------------------------------------
	// Middleware

	router.Use(func(next http.Handler) http.Handler {
		return segmentRXHandler.LoggingMiddleware(next.ServeHTTP)
	})

	// --------------------------------------------------------------------------------
	// HTTP Server

	serverListen := fmt.Sprintf(
		"%s:%d", httpCfg.Server.ListenOn, httpCfg.Server.Port,
	)
	httpSrv := &http.Server{
		Addr:         serverListen,
		WriteTimeout: time.Second * time.Duration(httpCfg.Server.Timeouts.WriteTimeout),
		ReadTimeout:  time.Second * time.Duration(httpCfg.Server.Timeouts.ReadTimeout),
		IdleTimeout:  time.Second * time.Duration(httpCfg.Server.Timeouts.IdleTimeout),
		Handler:      h2c.NewHandler(router, &http2.Server{}),
	}

	return httpSrv, nil
}

// ====================================================================================
// VOD Server

/*
BuildVODServer create HLS VOD server

	@param httpCfg common.APIServerConfig - HTTP server configuration
	@param dbConns db.ConnectionManager - DB connection manager
	@param manager vod.PlaylistManager - video playlist manager
	@param metrics goutils.HTTPRequestMetricHelper - metric collection agent
	@returns HTTP server instance
*/
func BuildVODServer(
	httpCfg common.APIServerConfig,
	dbConns db.ConnectionManager,
	manager vod.PlaylistManager,
	metrics goutils.HTTPRequestMetricHelper,
) (*http.Server, error) {
	httpHandler, err := NewVODHandler(
		dbConns, manager, httpCfg.APIs.RequestLogging, metrics,
	)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter()
	mainRouter := registerPathPrefix(router, httpCfg.APIs.Endpoint.PathPrefix, nil)
	v1Router := registerPathPrefix(mainRouter, "/v1", nil)

	// --------------------------------------------------------------------------------
	// VOD endpoint
	_ = registerPathPrefix(
		v1Router, "/vod/live/{videoSourceID}/{fileName}", map[string]http.HandlerFunc{
			"get": httpHandler.GetLiveStreamVideoFilesHandler(),
		},
	)

	// --------------------------------------------------------------------------------
	// Middleware

	router.Use(func(next http.Handler) http.Handler {
		return httpHandler.LoggingMiddleware(next.ServeHTTP)
	})

	// --------------------------------------------------------------------------------
	// HTTP Server

	serverListen := fmt.Sprintf(
		"%s:%d", httpCfg.Server.ListenOn, httpCfg.Server.Port,
	)
	httpSrv := &http.Server{
		Addr:         serverListen,
		WriteTimeout: time.Second * time.Duration(httpCfg.Server.Timeouts.WriteTimeout),
		ReadTimeout:  time.Second * time.Duration(httpCfg.Server.Timeouts.ReadTimeout),
		IdleTimeout:  time.Second * time.Duration(httpCfg.Server.Timeouts.IdleTimeout),
		Handler:      h2c.NewHandler(router, &http2.Server{}),
	}

	return httpSrv, nil
}
