package utils_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type s3ClientConfig struct {
	common.S3Config
	accessKey string
	secretKey string
}

func getS3ClientConfig(assert *assert.Assertions) s3ClientConfig {
	endpoint := os.Getenv("UNITTEST_S3_ENDPOINT")
	if endpoint == "" {
		endpoint = "127.0.0.1:9000"
	}
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	assert.NotEmpty(accessKey)
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	assert.NotEmpty(secretKey)
	return s3ClientConfig{
		S3Config:  common.S3Config{ServerEndpoint: endpoint, UseTLS: false},
		accessKey: accessKey,
		secretKey: secretKey,
	}
}

func TestS3ClientBasicOperations(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	// Get S3 config
	config := getS3ClientConfig(assert)

	uut, err := utils.NewS3Client(config.ServerEndpoint, config.accessKey, config.secretKey, false)
	assert.Nil(err)

	testBucket := fmt.Sprintf("ut-s3-client-basic-%s", uuid.NewString())

	// Case 0: create a bucket for testing
	assert.Nil(uut.CreateBucket(utCtxt, testBucket))
	{
		buckets, err := uut.ListBuckets(utCtxt)
		assert.Nil(err)
		bucketByName := map[string]string{}
		for _, name := range buckets {
			bucketByName[name] = ""
		}
		assert.Contains(bucketByName, testBucket)
	}

	// Case 1: reading unknown object
	{
		_, err := uut.GetObject(utCtxt, testBucket, uuid.NewString())
		assert.NotNil(err)
	}

	// Case 2: create a object
	testObject := uuid.NewString()
	testContent := []byte(uuid.NewString())
	assert.Nil(uut.PutObject(utCtxt, testBucket, testObject, testContent))
	{
		content, err := uut.GetObject(utCtxt, testBucket, testObject)
		assert.Nil(err)
		assert.EqualValues(testContent, content)
	}

	// Case 3: delete object
	assert.Nil(uut.DeleteObject(utCtxt, testBucket, testObject))
	{
		_, err := uut.GetObject(utCtxt, testBucket, testObject)
		assert.NotNil(err)
	}
}
