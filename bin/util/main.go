package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
	apexJSON "github.com/apex/log/handlers/json"
	"github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
	"gorm.io/gorm/logger"
)

type newVideoSourceList struct {
	Sources []api.NewVideoSourceRequest `json:"sources" validate:"required,gte=1"`
}

type provisionVideoSourcArgs struct {
	DefinitionFile string `validate:"required,file"`
}

type ctrlNodeCliArgs struct {
	ConfigFile string `validate:"required,file"`
	DBPassword string
}

type cliArgs struct {
	JSONLog         bool
	LogLevel        string `validate:"required,oneof=debug info warn error"`
	APIBaseURL      string `validate:"required,url"`
	RequestIDHeader string `validate:"required"`
}

var s3CredsArgs common.S3Credentials

var ctrlNodeArgs ctrlNodeCliArgs

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
			{
				Name:        "purge-unknown-segments-from-obj-store",
				Aliases:     []string{"purge-unknown"},
				Usage:       "Purge unknown segments in object store",
				Description: "Purge segments in the object store which do not match any recording segment entries.",
				Flags: []cli.Flag{
					// Config file
					&cli.StringFlag{
						Name:        "config-file",
						Usage:       "Application config file",
						Aliases:     []string{"c"},
						EnvVars:     []string{"CONFIG_FILE"},
						Destination: &ctrlNodeArgs.ConfigFile,
						Required:    true,
					},
					// DB password
					&cli.StringFlag{
						Name:        "db-password",
						Usage:       "Database user password",
						Aliases:     []string{"p"},
						EnvVars:     []string{"DB_USER_PASSWORD"},
						Value:       "",
						DefaultText: "",
						Destination: &ctrlNodeArgs.DBPassword,
						Required:    false,
					},
					// S3 Creds
					&cli.StringFlag{
						Name:        "s3-access-key",
						Usage:       "S3 user access key",
						EnvVars:     []string{"AWS_ACCESS_KEY_ID"},
						Destination: &s3CredsArgs.AccessKey,
						Required:    true,
					},
					&cli.StringFlag{
						Name:        "s3-secret-access-key",
						Usage:       "S3 user secret access key",
						EnvVars:     []string{"AWS_SECRET_ACCESS_KEY"},
						Destination: &s3CredsArgs.SecretAccessKey,
						Required:    true,
					},
				},
				Action: purgeUnknownSegmentsFromObjStore,
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

func purgeUnknownSegmentsFromObjStore(c *cli.Context) error {
	validate := validator.New()

	// Validate general config
	if err := validate.Struct(&cmdArgs); err != nil {
		return err
	}

	setupLogging()

	runtimeCtxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ================================================================================
	// Process system control node config
	if err := validate.Struct(&ctrlNodeArgs); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Invalid parameters provided to start system control node")
		return err
	}

	// Process the config file
	common.InstallDefaultControlNodeConfigValues()
	var configs common.ControlNodeConfig
	viper.SetConfigFile(ctrlNodeArgs.ConfigFile)
	if err := viper.ReadInConfig(); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to load system control node config")
		return err
	}
	if err := viper.Unmarshal(&configs); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to parse system control node config")
		return err
	}

	// Validate system control node config
	if err := validate.Struct(&configs); err != nil {
		log.WithError(err).WithFields(logTags).Error("System control node config file is not valid")
		return err
	}

	storageConfig := configs.VODConfig.RecordingStorage
	storageConfig.S3.Creds = &s3CredsArgs

	// ================================================================================
	// Prepare DB connectors

	sqlDSN, err := db.GetPostgresDialector(configs.Postgres, ctrlNodeArgs.DBPassword)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define Postgres connection DSN")
		return err
	}

	dbConns, err := db.NewSQLConnection(sqlDSN, logger.Error)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define SQL connection manager")
		return err
	}

	// ================================================================================
	// Prepare S3 client

	s3Client, err := utils.NewS3Client(storageConfig.S3)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to create S3 client")
		return err
	}

	// ================================================================================
	// Get the current set of all known segments

	knownRecordingSegments := map[string]common.VideoSegment{}
	{
		db := dbConns.NewPersistanceManager()

		entries, err := db.ListAllRecordingSegments(runtimeCtxt)
		if err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to list all recording segments")
			return err
		}

		db.Close()

		for _, entry := range entries {
			knownRecordingSegments[entry.URI] = entry
		}
	}

	// ================================================================================
	// Get the current set of all segments in the object store

	objectsInStorage := map[string]string{}
	{
		objects, err := s3Client.ListObjects(
			runtimeCtxt, storageConfig.StorageBucket, &storageConfig.StorageObjectPrefix,
		)
		if err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to list all objects in storage")
			return err
		}

		for _, objectKey := range objects {
			fullURI := fmt.Sprintf("s3://%s/%s", storageConfig.StorageBucket, objectKey)
			objectsInStorage[fullURI] = objectKey
		}
	}

	// ================================================================================
	// Get list of all objects in storage that doesn't match an recording segment entry

	toDeleteObjKeys := []string{}
	for objFullURI, objKey := range objectsInStorage {
		if _, ok := knownRecordingSegments[objFullURI]; !ok {
			// Object is unknown, must remove
			toDeleteObjKeys = append(toDeleteObjKeys, objKey)
		}
	}
	log.
		WithFields(logTags).
		WithField("purge-objects", toDeleteObjKeys).
		Debug("Purging unknown segments")

	// ================================================================================
	// Remove the unknown objects from storage

	if err := s3Client.DeleteObjects(
		runtimeCtxt, storageConfig.StorageBucket, toDeleteObjKeys,
	); err != nil {
		errByObject := map[string]string{}
		for _, oneErr := range err {
			errByObject[oneErr.Object] = oneErr.Error()
		}
		log.
			WithFields(logTags).
			WithField("errors", errByObject).
			Error("Failed to delete unknown objects")
		return fmt.Errorf("failed to delete unknown objects")
	}

	return nil
}
