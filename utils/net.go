package utils

import (
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/go-resty/resty/v2"
)

/*
DefineHTTPClient helper function to define a resty HTTP client

	@param config common.HTTPClientConfig - HTTP client config
	@returns new resty client
*/
func DefineHTTPClient(config common.HTTPClientConfig) (*resty.Client, error) {
	newClient := resty.New()

	// Configure resty client retry setting
	newClient = newClient.
		SetRetryCount(config.Retry.MaxAttempts).
		SetRetryWaitTime(time.Second * time.Duration(config.Retry.InitWaitTimeInSec)).
		SetRetryMaxWaitTime(time.Second * time.Duration(config.Retry.MaxWaitTimeInSec))

	return newClient, nil
}
