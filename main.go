package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/alwitt/livemix/bin"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	apexJSON "github.com/apex/log/handlers/json"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
)

type ctrlNodeCliArgs struct {
	ConfigFile string `validate:"required,file"`
	DBPassword string
}

type edgeNodeCliArgs struct {
	ConfigFile        string `validate:"required,file"`
	OAuthClientID     string
	OAuthClientSecret string
}

type cliArgs struct {
	JSONLog      bool
	LogLevel     string `validate:"required,oneof=debug info warn error"`
	Hostname     string
	GCPCredsFile string `validate:"required,file"`
}

var s3CredsArgs common.S3Credentials

var ctrlNodeArgs ctrlNodeCliArgs

var edgeNodeArgs edgeNodeCliArgs

var cmdArgs cliArgs

var logTags log.Fields

// @title livemix
// @version v0.1.0-rc2
// @description HLS Live Streaming Proxy

// @host localhost:8080
// @BasePath /
// @query.collection.format multi
func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.WithError(err).Fatal("Unable to read hostname")
	}
	cmdArgs.Hostname = hostname
	logTags = log.Fields{
		"module":    "main",
		"component": "main",
		"instance":  hostname,
	}

	app := &cli.App{
		Version:     "v0.1.0",
		Usage:       "application entrypoint",
		Description: "HLS VOD relay framework with stream recording capability",
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
			// GCP Cred
			&cli.PathFlag{
				Name:        "gcp-cred",
				Usage:       "Not directly used by the application, this option provides GCP cred to SDK.",
				EnvVars:     []string{"GOOGLE_APPLICATION_CREDENTIALS"},
				Destination: &cmdArgs.GCPCredsFile,
				Required:    true,
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
		Commands: []*cli.Command{
			{
				Name:        "control",
				Aliases:     []string{"ctrl"},
				Usage:       "Run system control node",
				Description: "Start the system control node and its various API servers.",
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
				},
				Action: startControlNode,
			},
			{
				Name:        "edge",
				Usage:       "Run video proxy edge node",
				Description: "Start the edge node to monitor and proxy a video source",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "config-file",
						Usage:       "Application config file",
						Aliases:     []string{"c"},
						EnvVars:     []string{"CONFIG_FILE"},
						Destination: &edgeNodeArgs.ConfigFile,
						Required:    true,
					},
					&cli.StringFlag{
						Name:        "oauth-client-id",
						Usage:       "OAuth client ID",
						Aliases:     []string{"i"},
						EnvVars:     []string{"OAUTH_CLIENT_ID"},
						Value:       "",
						DefaultText: "",
						Destination: &edgeNodeArgs.OAuthClientID,
						Required:    false,
					},
					&cli.StringFlag{
						Name:        "oauth-client-secret",
						Usage:       "OAuth client secret",
						Aliases:     []string{"s"},
						EnvVars:     []string{"OAUTH_CLIENT_SECRET"},
						Value:       "",
						DefaultText: "",
						Destination: &edgeNodeArgs.OAuthClientSecret,
						Required:    false,
					},
				},
				Action: startEdgeNode,
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

func startControlNode(c *cli.Context) error {
	validate := validator.New()

	// Validate general config
	if err := validate.Struct(&cmdArgs); err != nil {
		return err
	}

	setupLogging()

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

	{
		t, _ := json.MarshalIndent(&configs, "", "  ")
		log.WithFields(logTags).Debugf("Running with config:\n%s", string(t))
	}

	// ================================================================================
	// Define system control node

	runtimeCtxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	configs.VODConfig.RecordingStorage.S3.Creds = &s3CredsArgs

	ctrlNode, err := bin.DefineControlNode(
		runtimeCtxt, cmdArgs.Hostname, configs, ctrlNodeArgs.DBPassword,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define and start system control node")
		return err
	}
	defer func() {
		if err := ctrlNode.Cleanup(runtimeCtxt); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failure during system control clean up")
		}
	}()

	// ================================================================================
	// Start HTTP servers

	wg := sync.WaitGroup{}
	defer wg.Wait()
	apiServers := map[string]*http.Server{}
	cleanUpTasks := map[string]func(context.Context) error{}

	defer func() {
		// Shutdown the servers
		for svrInstance, svr := range apiServers {
			ctx, cancel := context.WithTimeout(runtimeCtxt, time.Second*10)
			if err := svr.Shutdown(ctx); err != nil {
				log.
					WithError(err).
					WithFields(logTags).
					Errorf("Failure during HTTP Server %s shutdown", svrInstance)
			}
			cancel()
		}
		// Perform other clean up tasks
		for taskName, task := range cleanUpTasks {
			ctx, cancel := context.WithTimeout(runtimeCtxt, time.Second*10)
			if err := task(ctx); err != nil {
				log.WithError(err).WithFields(logTags).Errorf("Clean up task '%s' failed", taskName)
			}
			cancel()
		}
	}()

	// Start management HTTP server
	{
		svr := ctrlNode.MgmtAPIServer
		apiServers["mgmt-api"] = svr
		// Start the server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.WithError(err).Error("System Management API HTTP server failure")
			}
		}()
	}
	// Start VOD HTTP server
	{
		svr := ctrlNode.VODAPIServer
		apiServers["vod-api"] = svr
		// Start the server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.WithError(err).Error("VOD API HTTP server failure")
			}
		}()
	}
	// Start metrics HTTP server
	{
		svr := ctrlNode.MetricsServer
		apiServers["metrics-api"] = svr
		// Start the server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.WithError(err).Error("Metrics API HTTP server failure")
			}
		}()
	}

	// ------------------------------------------------------------------------------------
	// Wait for termination

	cc := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(cc, os.Interrupt)
	<-cc

	return nil
}

func startEdgeNode(c *cli.Context) error {
	validate := validator.New()

	// Validate general config
	if err := validate.Struct(&cmdArgs); err != nil {
		return err
	}

	setupLogging()

	// ================================================================================
	// Process edge node config
	if err := validate.Struct(&edgeNodeArgs); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Invalid parameters provided to start edge node")
		return err
	}

	// Process the config file
	common.InstallDefaultEdgeNodeConfigValues()
	var configs common.EdgeNodeConfig
	viper.SetConfigFile(edgeNodeArgs.ConfigFile)
	if err := viper.ReadInConfig(); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to load edge node config")
		return err
	}
	if err := viper.Unmarshal(&configs); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to parse edge node config")
		return err
	}

	// Inject the OAuth client creds if the OAuth section is defined
	if configs.Forwarder.Live.Remote.Client.OAuth != nil {
		configs.Forwarder.Live.Remote.Client.OAuth.ClientID = edgeNodeArgs.OAuthClientID
		configs.Forwarder.Live.Remote.Client.OAuth.ClientSecret = edgeNodeArgs.OAuthClientSecret
	}

	// Validate edge node config
	if err := validate.Struct(&configs); err != nil {
		log.WithError(err).WithFields(logTags).Error("Edge node config file is not valid")
		return err
	}

	{
		t, _ := json.MarshalIndent(&configs, "", "  ")
		log.WithFields(logTags).Debugf("Running with config:\n%s", string(t))
	}

	configs.Forwarder.Recording.RecordingStorage.S3.Creds = &s3CredsArgs

	// ================================================================================
	// Define edge node

	runtimeCtxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	edgeNode, err := bin.DefineEdgeNode(runtimeCtxt, cmdArgs.Hostname, configs)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define and start edge node")
		return err
	}
	defer func() {
		if err := edgeNode.Cleanup(runtimeCtxt); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failure during edge node clean up")
		}
	}()

	// ================================================================================
	// Start HTTP servers

	wg := sync.WaitGroup{}
	defer wg.Wait()
	apiServers := map[string]*http.Server{}
	cleanUpTasks := map[string]func(context.Context) error{}

	defer func() {
		// Shutdown the servers
		for svrInstance, svr := range apiServers {
			ctx, cancel := context.WithTimeout(runtimeCtxt, time.Second*10)
			if err := svr.Shutdown(ctx); err != nil {
				log.
					WithError(err).
					WithFields(logTags).
					Errorf("Failure during HTTP Server %s shutdown", svrInstance)
			}
			cancel()
		}
		// Perform other clean up tasks
		for taskName, task := range cleanUpTasks {
			ctx, cancel := context.WithTimeout(runtimeCtxt, time.Second*10)
			if err := task(ctx); err != nil {
				log.WithError(err).WithFields(logTags).Errorf("Clean up task '%s' failed", taskName)
			}
			cancel()
		}
	}()

	// Start local query API HTTP server
	{
		svr := edgeNode.QueryAPIServer
		apiServers["query-api"] = svr
		// Start the server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.WithError(err).Error("Local query API HTTP server failure")
			}
		}()
	}

	// Start playlist receiver HTTP server
	{
		svr := edgeNode.PlaylistReceiveServer
		apiServers["playlist-rx"] = svr
		// Start the server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.WithError(err).Error("Playlist receiver HTTP server failure")
			}
		}()
	}

	// Start local live VOD HTTP server
	{
		svr := edgeNode.VODServer
		apiServers["local-live-vod"] = svr
		// Start the server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.WithError(err).Error("Local live VOD HTTP server failure")
			}
		}()
	}

	// Start metrics HTTP server
	{
		svr := edgeNode.MetricsServer
		apiServers["metrics-api"] = svr
		// Start the server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.WithError(err).Error("Metrics API HTTP server failure")
			}
		}()
	}

	// ------------------------------------------------------------------------------------
	// Wait for termination

	cc := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(cc, os.Interrupt)
	<-cc

	return nil
}
