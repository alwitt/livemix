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

func getS3ClientConfig(assert *assert.Assertions) common.S3Config {
	endpoint := os.Getenv("UNITTEST_S3_ENDPOINT")
	if endpoint == "" {
		endpoint = "127.0.0.1:9000"
	}
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	assert.NotEmpty(accessKey)
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	assert.NotEmpty(secretKey)
	return common.S3Config{
		ServerEndpoint: endpoint, UseTLS: false, Creds: &common.S3Credentials{
			AccessKey: accessKey, SecretAccessKey: secretKey,
		},
	}
}

func TestS3ClientBasicOperations(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	// Get S3 config
	config := getS3ClientConfig(assert)

	uut, err := utils.NewS3Client(config)
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

	// Case 4: delete the test bucket
	assert.Nil(uut.DeleteBucket(utCtxt, testBucket))
}

func TestS3ClientBulkDelete(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	// Get S3 config
	config := getS3ClientConfig(assert)

	uut, err := utils.NewS3Client(config)
	assert.Nil(err)

	testBucket := fmt.Sprintf("ut-s3-client-basic-%s", uuid.NewString())

	// Create a bucket for testing
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

	type testObject struct {
		Key     string
		Content []byte
	}

	testObjects := []testObject{
		{Key: uuid.NewString(), Content: []byte(uuid.NewString())},
		{Key: uuid.NewString(), Content: []byte(uuid.NewString())},
		{Key: uuid.NewString(), Content: []byte(uuid.NewString())},
	}

	// Create test objects
	for _, object := range testObjects {
		assert.Nil(uut.PutObject(utCtxt, testBucket, object.Key, object.Content))
	}
	for _, object := range testObjects {
		content, err := uut.GetObject(utCtxt, testBucket, object.Key)
		assert.Nil(err)
		assert.EqualValues(object.Content, content)
	}

	deleteObjectKeys := []string{}
	for _, object := range testObjects {
		deleteObjectKeys = append(deleteObjectKeys, object.Key)
	}
	// Add one unknown key
	deleteObjectKeys = append(deleteObjectKeys, uuid.NewString())

	// Delete objects
	_ = uut.DeleteObjects(utCtxt, testBucket, deleteObjectKeys)

	// Verify the objects are gone
	for _, object := range testObjects {
		_, err := uut.GetObject(utCtxt, testBucket, object.Key)
		assert.NotNil(err)
	}

	// Delete the test bucket
	assert.Nil(uut.DeleteBucket(utCtxt, testBucket))
}
