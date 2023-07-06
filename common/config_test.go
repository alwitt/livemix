package common_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/alwitt/livemix/common"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestControlNodeConfig(t *testing.T) {
	assert := assert.New(t)

	validate := validator.New()

	// Case 0: by default the config is not valid
	{
		cfg := common.ControlNodeConfig{}
		assert.NotNil(validate.Struct(&cfg))
	}

	// Install defaults
	common.InstallDefaultControlNodeConfigValues()

	{
		_, err := os.Create("/tmp/psql_ca.pem")
		assert.Nil(err)
	}

	viper.SetConfigType("yaml")

	// Case 1: a complete valid case
	{
		config := []byte(`---
postgres:
  host: postgres
  db: postgres
  user: postgres
  ssl:
    enabled: true
    caFile: /tmp/psql_ca.pem

management:
  requestResponse:
    gcpProject: rpi-cam
    self:
      topic: control
      msgTTL: 900

broadcast:
  pubsub:
    topic: system-events`)
		assert.Nil(viper.ReadConfig(bytes.NewBuffer(config)))
		var cfg common.ControlNodeConfig
		assert.Nil(viper.Unmarshal(&cfg))
		err := validate.Struct(&cfg)
		assert.Nil(err)

		// Verify the some fields
		assert.Equal(60, cfg.Management.APIServer.Server.Timeouts.IdleTimeout)
		assert.Equal("postgres", cfg.Postgres.User)
		assert.NotNil(cfg.Postgres.SSL.CAFile)
		assert.Equal("/tmp/psql_ca.pem", *cfg.Postgres.SSL.CAFile)
		assert.Equal("control", cfg.Management.RRClient.InboudRequestTopic.Topic)
	}

	// Case 2: missing a config parameter
	{
		config := []byte(`---
postgres:
  host: postgres
  db: postgres
  user: postgres
  ssl:
    enabled: true
    caFile: /tmp/psql_ca.pem

management:
  requestResponse:
    gcpProject: rpi-cam
    self:
      msgTTL: 900

broadcast:
  pubsub:
    topic: system-events`)
		assert.Nil(viper.ReadConfig(bytes.NewBuffer(config)))
		var cfg common.ControlNodeConfig
		assert.Nil(viper.Unmarshal(&cfg))
		err := validate.Struct(&cfg)
		assert.NotNil(err)
	}

	// Case 3: value fail constraint
	{
		config := []byte(`---
postgres:
  host: postgres
  db: postgres
  user: postgres
  ssl:
    enabled: true
    caFile: /tmp/psql_ca.pem

management:
  requestResponse:
    gcpProject: rpi-cam
    self:
      topic: control
      msgTTL: 300

broadcast:
  pubsub:
    topic: system-events`)
		assert.Nil(viper.ReadConfig(bytes.NewBuffer(config)))
		var cfg common.ControlNodeConfig
		assert.Nil(viper.Unmarshal(&cfg))
		err := validate.Struct(&cfg)
		assert.NotNil(err)
	}
}
