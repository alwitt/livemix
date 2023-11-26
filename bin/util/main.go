package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	apexJSON "github.com/apex/log/handlers/json"
	"github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/oklog/ulid/v2"
	"github.com/urfave/cli/v2"
)

type newVideoSourceList struct {
	Sources []api.NewVideoSourceRequest `json:"sources" validate:"required,gte=1"`
}

type provisionVideoSourcArgs struct {
	DefinitionFile string `validate:"required,file"`
}

type cliArgs struct {
	JSONLog         bool
	LogLevel        string `validate:"required,oneof=debug info warn error"`
	APIBaseURL      string `validate:"required,url"`
	RequestIDHeader string `validate:"required"`
}

var cmdArgs cliArgs

var logTags log.Fields

var provSrcArgs provisionVideoSourcArgs

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.WithError(err).Fatal("Unable to read hostname")
	}
	logTags = log.Fields{
		"module":    "main",
		"component": "main",
		"instance":  hostname,
	}

	app := &cli.App{
		Version:     "v0.1.0",
		Usage:       "application entrypoint",
		Description: "Livemix OPS support utility application",
		Flags: []cli.Flag{
			// LOGGING
			&cli.BoolFlag{
				Name:        "json-log",
				Usage:       "Whether to log in JSON format",
				Aliases:     []string{"j"},
				EnvVars:     []string{"LOG_AS_JSON"},
				Value:       false,
				DefaultText: "false",
				Destination: &cmdArgs.JSONLog,
				Required:    false,
			},
			&cli.StringFlag{
				Name:        "log-level",
				Usage:       "Logging level: [debug info warn error]",
				Aliases:     []string{"l"},
				EnvVars:     []string{"LOG_LEVEL"},
				Value:       "warn",
				DefaultText: "warn",
				Destination: &cmdArgs.LogLevel,
				Required:    false,
			},
			// Controller base URL
			&cli.StringFlag{
				Name:        "api-base-url",
				Usage:       "Livemix system controller API base URL",
				Aliases:     []string{"u"},
				EnvVars:     []string{"CTRL_API_BASE_URL"},
				Value:       "http://127.0.0.1:8080",
				DefaultText: "http://127.0.0.1:8080",
				Destination: &cmdArgs.APIBaseURL,
				Required:    false,
			},
			&cli.StringFlag{
				Name:        "request-id-header",
				Usage:       "HTTP header for request ID",
				Aliases:     []string{"i"},
				EnvVars:     []string{"REQUEST_ID_HTTP_HEADER"},
				Value:       "X-Request-ID",
				DefaultText: "X-Request-ID",
				Destination: &cmdArgs.RequestIDHeader,
				Required:    false,
			},
		},
		Commands: []*cli.Command{
			{
				Name:        "provision-video-source",
				Aliases:     []string{"prov-src"},
				Usage:       "Provision video sources",
				Description: "Provision new video sources in the system.",
				Flags: []cli.Flag{
					// Config file
					&cli.StringFlag{
						Name:        "definition-file",
						Usage:       "New video source definition file",
						Aliases:     []string{"c"},
						EnvVars:     []string{"DEFINITION_FILE"},
						Destination: &provSrcArgs.DefinitionFile,
						Required:    true,
					},
				},
				Action: provisionVideoSources,
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.WithError(err).WithFields(logTags).Fatal("Program shutdown")
	}
}

// setupLogging helper function to prepare the app logging
func setupLogging() {
	if cmdArgs.JSONLog {
		log.SetHandler(apexJSON.New(os.Stderr))
	}
	switch cmdArgs.LogLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.ErrorLevel)
	}
}

func provisionVideoSources(c *cli.Context) error {
	validate := validator.New()

	// Validate general config
	if err := validate.Struct(&cmdArgs); err != nil {
		return err
	}

	setupLogging()

	if err := validate.Struct(&provSrcArgs); err != nil {
		return err
	}

	// Process video source definition files
	var definitionFile newVideoSourceList
	if theFile, err := os.Open(provSrcArgs.DefinitionFile); err != nil {
		return err
	} else if err := json.NewDecoder(theFile).Decode(&definitionFile); err != nil {
		return err
	}

	{
		t, _ := json.Marshal(definitionFile.Sources)
		log.WithFields(logTags).WithField("sources", string(t)).Info("Provision video sources")
	}

	targetURL, err := url.Parse(fmt.Sprintf("%s/v1/source", cmdArgs.APIBaseURL))
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-define-url", fmt.Sprintf("%s/v1/source", cmdArgs.APIBaseURL)).
			Error("Unable to parse video source define URL")
		return err
	}

	client := resty.New()

	reqID := ulid.Make().String()

	// Get all known video sources
	resp, err := client.R().
		// Set request header
		SetHeader(cmdArgs.RequestIDHeader, reqID).
		// Setup error parsing
		SetError(goutils.RestAPIBaseResponse{}).
		Get(targetURL.String())
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("request-id", reqID).
			Error("Video source query failed on call")
		return err
	}
	if resp.IsError() {
		respError := resp.Error().(*goutils.RestAPIBaseResponse)
		var err error
		if respError.Error != nil {
			err = fmt.Errorf(respError.Error.Detail)
		} else {
			err = fmt.Errorf("status code %d", resp.StatusCode())
		}
		log.
			WithError(err).
			WithFields(logTags).
			WithField("request-id", reqID).
			Error("Video source query failed")
		return err
	}
	var existingSources api.VideoSourceInfoListResponse
	if err := json.Unmarshal(resp.Body(), &existingSources); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to parse video source query response")
		return err
	} else if err := validate.Struct(&existingSources); err != nil {
		log.WithError(err).WithFields(logTags).Error("Invalid video source query response")
		return err
	}

	videoSourceByName := map[string]common.VideoSource{}
	for _, source := range existingSources.Sources {
		videoSourceByName[source.Name] = source
	}

	// Go through each source
	for _, source := range definitionFile.Sources {
		payload, _ := json.Marshal(&source)
		// Check whether a source already exist
		if _, ok := videoSourceByName[source.Name]; ok {
			log.
				WithFields(logTags).
				WithField("source", string(payload)).
				Info("Video source already exist")
			continue
		}

		reqID = ulid.Make().String()

		// Define the missing source
		resp, err := client.R().
			// Set request header
			SetHeader(cmdArgs.RequestIDHeader, reqID).
			// Set request payload
			SetBody(payload).
			// Setup error parsing
			SetError(goutils.RestAPIBaseResponse{}).
			Post(targetURL.String())

		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source", string(payload)).
				WithField("request-id", reqID).
				Error("Video source define failed on call")
			return err
		}

		if resp.IsError() {
			respError := resp.Error().(*goutils.RestAPIBaseResponse)
			var err error
			if respError.Error != nil {
				err = fmt.Errorf(respError.Error.Detail)
			} else {
				err = fmt.Errorf("status code %d", resp.StatusCode())
			}
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source", string(payload)).
				WithField("request-id", reqID).
				Error("Video source define failed")
			return err
		}

		log.
			WithFields(logTags).
			WithField("source", string(payload)).
			WithField("request-id", reqID).
			Info("Video source defined")
	}

	return nil
}
