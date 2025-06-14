package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Client client for interacting with S3
type S3Client interface {
	/*
		ListBuckets get a list of available buckets at the server

			@param ctxt context.Context - execution context
			@returns list of bucket names
	*/
	ListBuckets(ctxt context.Context) ([]string, error)

	/*
		ListObjects get a list of objects in a bucket

			@param ctxt context.Context - execution context
			@param bucket string - the bucket name
			@param prefix *string - optionally, specify the object prefix to filter on
			@return list of bucket objects
	*/
	ListObjects(ctxt context.Context, bucket string, prefix *string) ([]string, error)

	/*
		CreateBucket create a bucket

			@param ctxt context.Context - execution context
			@param bucketName string - new bucket name
	*/
	CreateBucket(ctxt context.Context, bucketName string) error

	/*
		DeleteBucket delete a bucket

			@param ctxt context.Context - execution context
			@param bucketName string - new bucket name
	*/
	DeleteBucket(ctxt context.Context, bucketName string) error

	/*
		PutObject put a new object into a bucket

			@param ctxt context.Context - execution context
			@param bucketName string - target bucket name
			@param objectKey string - target object name within the bucket
			@param content []byte - object
	*/
	PutObject(ctxt context.Context, bucketName, objectKey string, content []byte) error

	/*
		GetObject get an object from a bucket

			@param ctxt context.Context - execution context
			@param bucketName string - target bucket name
			@param objectKey string - target object name within the bucket
			@returns object content
	*/
	GetObject(ctxt context.Context, bucketName, objectKey string) ([]byte, error)

	/*
		DeleteObject delete an object from a bucket

			@param ctxt context.Context - execution context
			@param bucketName string - target bucket name
			@param objectKey string - target object name within the bucket
	*/
	DeleteObject(ctxt context.Context, bucketName, objectKey string) error

	/*
		DeleteObjects delete a group of objects from a bucket

			@param ctxt context.Context - execution context
			@param bucketName string - target bucket name
			@param objectKeys []string - target object names within the bucket
	*/
	DeleteObjects(
		ctxt context.Context, bucketName string, objectKeys []string,
	) []S3BulkDeleteError
}

// s3ClientImpl implements S3Client
type s3ClientImpl struct {
	goutils.Component
	s3 *minio.Client
}

/*
NewS3Client define new S3 operation client

	@param config common.S3Config - S3 client config
	@returns new client
*/
func NewS3Client(config common.S3Config) (S3Client, error) {
	logTags := log.Fields{
		"module":    "utils",
		"component": "s3-client",
		"instance":  config.ServerEndpoint,
	}

	// Define the core minio client
	client, err := minio.New(config.ServerEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.Creds.AccessKey, config.Creds.SecretAccessKey, ""),
		Secure: config.UseTLS,
	})
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define minio S3 client")
		return nil, err
	}

	return &s3ClientImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		}, s3: client,
	}, nil
}

func (s *s3ClientImpl) ListBuckets(ctxt context.Context) ([]string, error) {
	buckets, err := s.s3.ListBuckets(ctxt)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, bucket := range buckets {
		result = append(result, bucket.Name)
	}
	return result, nil
}

func (s *s3ClientImpl) ListObjects(
	ctxt context.Context, bucket string, prefix *string,
) ([]string, error) {
	options := minio.ListObjectsOptions{
		Recursive: true,
	}
	if prefix != nil {
		options.Prefix = *prefix
	}

	objReadCh := s.s3.ListObjects(ctxt, bucket, options)

	result := []string{}
	for objInfo := range objReadCh {
		if objInfo.Err != nil {
			return nil, objInfo.Err
		}
		result = append(result, objInfo.Key)
	}

	return result, nil
}

func (s *s3ClientImpl) CreateBucket(ctxt context.Context, bucketName string) error {
	return s.s3.MakeBucket(ctxt, bucketName, minio.MakeBucketOptions{})
}

func (s *s3ClientImpl) PutObject(
	ctxt context.Context, bucketName, objectKey string, content []byte,
) error {
	_, err := s.s3.PutObject(
		ctxt,
		bucketName,
		objectKey,
		bytes.NewBuffer(content),
		int64(len(content)),
		minio.PutObjectOptions{SendContentMd5: true},
	)
	return err
}

func (s *s3ClientImpl) DeleteBucket(ctxt context.Context, bucketName string) error {
	return s.s3.RemoveBucket(ctxt, bucketName)
}

func (s *s3ClientImpl) GetObject(
	ctxt context.Context, bucketName, objectName string,
) ([]byte, error) {
	object, err := s.s3.GetObject(ctxt, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = object.Close()
	}()
	content, err := io.ReadAll(object)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (s *s3ClientImpl) DeleteObject(ctxt context.Context, bucketName, objectKey string) error {
	return s.s3.RemoveObject(ctxt, bucketName, objectKey, minio.RemoveObjectOptions{})
}

// S3BulkDeleteError a wrapper object returned if bulk object delete failed for any
// particular objects
type S3BulkDeleteError struct {
	Err    error
	Object string
}

// Error implement the error interface
func (e S3BulkDeleteError) Error() string {
	return fmt.Sprintf("failed to delete object '%s' due to ['%s']", e.Object, e.Err.Error())
}

func (s *s3ClientImpl) DeleteObjects(
	ctxt context.Context, bucketName string, objectKeys []string,
) []S3BulkDeleteError {
	objectFeed := make(chan minio.ObjectInfo, len(objectKeys))
	for _, objectKey := range objectKeys {
		objectFeed <- minio.ObjectInfo{Key: objectKey}
	}
	close(objectFeed)

	resultFeed := s.s3.RemoveObjects(ctxt, bucketName, objectFeed, minio.RemoveObjectsOptions{})

	var delete []S3BulkDeleteError

	// Wait for request complete
	complete := false
	for !complete {
		select {
		case <-ctxt.Done():
			return []S3BulkDeleteError{{Err: fmt.Errorf("bulk delete context expired"), Object: "ALL"}}
		case err, ok := <-resultFeed:
			if !ok {
				complete = true
				break
			}
			delete = append(delete, S3BulkDeleteError{Err: err.Err, Object: err.ObjectName})
		}
	}

	return delete
}
