package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/control"
	"github.com/alwitt/livemix/hls"
	"github.com/gorilla/mux"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// ====================================================================================
// System Manage Server

/*
BuildSystemManagementServer create system management API server

	@param httpCfg common.APIServerConfig - HTTP server configuration
	@param manager control.SystemManager - core system manager
	@returns HTTP server instance
*/
func BuildSystemManagementServer(
	httpCfg common.APIServerConfig,
	manager control.SystemManager,
) (*http.Server, error) {
	httpHandler, err := NewSystemManagerHandler(manager, httpCfg.APIs.RequestLogging)
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter()
	mainRouter := registerPathPrefix(router, httpCfg.APIs.Endpoint.PathPrefix, nil)
	v1Router := registerPathPrefix(mainRouter, "/v1", nil)

	// --------------------------------------------------------------------------------
	// Health check
	_ = registerPathPrefix(v1Router, "/alive", map[string]http.HandlerFunc{
		"get": httpHandler.AliveHandler(),
	})
	_ = registerPathPrefix(v1Router, "/ready", map[string]http.HandlerFunc{
		"get": httpHandler.ReadyHandler(),
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
// Playlist Receiver Server

/*
BuildPlaylistReceiverServer create edge node playlist receive server

	@param httpCfg common.APIServerConfig - HTTP server configuration
	@param forwardCB PlaylistForwardCB - callback to forward newly received playlists
	@returns HTTP server instance
*/
func BuildPlaylistReceiverServer(
	httpCfg common.APIServerConfig, forwardCB PlaylistForwardCB,
) (*http.Server, error) {
	httpHandler, err := NewPlaylistReceiveHandler(
		hls.NewPlaylistParser(), forwardCB, httpCfg.APIs.RequestLogging,
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
