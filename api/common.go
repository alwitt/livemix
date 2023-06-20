package api

import (
	"net/http"

	"github.com/alwitt/goutils"
	"github.com/gorilla/mux"
)

// MethodHandlers DICT of method-endpoint handler
type MethodHandlers map[string]http.HandlerFunc

// RegisterPathPrefix registers new method handler for a path prefix
func RegisterPathPrefix(parent *mux.Router, prefix string, handler MethodHandlers) *mux.Router {
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
