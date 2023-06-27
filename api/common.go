package api

import (
	"net/http"

	"github.com/alwitt/goutils"
	"github.com/gorilla/mux"
)

// methodHandlers DICT of method-endpoint handler
type methodHandlers map[string]http.HandlerFunc

// registerPathPrefix registers new method handler for a path prefix
func registerPathPrefix(parent *mux.Router, prefix string, handler methodHandlers) *mux.Router {
	router := parent.PathPrefix(prefix).Subrouter()
	for method, handler := range handler {
		router.Methods(method).Path("").HandlerFunc(handler)
	}
	return router
}

// RequestResponseClient mirrors the goutils.RequestResponseClient
type RequestResponseClient interface {
	goutils.RequestResponseClient
}
