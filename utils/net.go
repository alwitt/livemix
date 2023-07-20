package utils

import (
	"context"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	"github.com/go-resty/resty/v2"
)

// setHTTPClientRetryParam helper function to install retry on HTTP client
func setHTTPClientRetryParam(
	client *resty.Client, config common.HTTPClientRetryConfig,
) *resty.Client {
	return client.
		SetRetryCount(config.MaxAttempts).
		SetRetryWaitTime(config.InitWaitTime()).
		SetRetryMaxWaitTime(config.MaxWaitTime())
}

// installHTTPClientAuthMiddleware install OAuth middleware on HTTP client
func installHTTPClientAuthMiddleware(
	parentCtxt context.Context,
	parentClient *resty.Client,
	config common.HTTPClientAuthConfig,
	retryCfg common.HTTPClientRetryConfig,
) error {
	// define client specifically to be used by
	oauthHTTPClient := setHTTPClientRetryParam(resty.New(), retryCfg)

	// define OAuth token manager
	oauthMgmt, err := goutils.GetNewClientCredOAuthTokenManager(
		parentCtxt, oauthHTTPClient, goutils.ClientCredOAuthTokenManagerParam{
			IDPIssuerURL:   config.IssuerURL,
			ClientID:       config.ClientID,
			ClientSecret:   config.ClientSecret,
			TargetAudience: config.TargetAudience,
			LogTags: log.Fields{
				"module": "go-utils", "component": "oauth-token-manager", "instance": "client-cred-flow",
			},
		},
	)
	if err != nil {
		return err
	}

	// Build middleware for parent client
	parentClient.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
		// Fetch OAuth token for request
		token, err := oauthMgmt.GetToken(parentCtxt, time.Now().UTC())
		if err != nil {
			return err
		}

		// Apply the token
		r.SetAuthToken(token)

		return nil
	})

	return nil
}

/*
DefineHTTPClient helper function to define a resty HTTP client

	@param parentCtxt context.Context - caller context
	@param config common.HTTPClientConfig - HTTP client config
	@returns new resty client
*/
func DefineHTTPClient(
	parentCtxt context.Context,
	config common.HTTPClientConfig,
) (*resty.Client, error) {
	newClient := resty.New()

	// Configure resty client retry setting
	newClient = setHTTPClientRetryParam(newClient, config.Retry)

	// Install OAuth middleware
	if config.OAuth != nil {
		err := installHTTPClientAuthMiddleware(parentCtxt, newClient, *config.OAuth, config.Retry)
		if err != nil {
			return nil, err
		}
	}

	return newClient, nil
}
