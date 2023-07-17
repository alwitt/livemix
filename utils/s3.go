package utils

import (
	"bytes"
	"context"
	"io"

	"github.com/alwitt/goutils"
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
		CreateBucket create a bucket

			@param ctxt context.Context - execution context
			@param bucketName string - new bucket name
	*/
	CreateBucket(ctxt context.Context, bucketName string) error

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
}

// s3ClientImpl implements S3Client
type s3ClientImpl struct {
	goutils.Component
	s3 *minio.Client
}

/*
NewS3Client define new S3 operation client

	@param serverEndpoint string - S3 server endpoint
	@param accessKey string - S3 access key
	@param secretKey string - S3 secret key
	@param withTLS bool - whether to use TLS
	@returns new client
*/
func NewS3Client(serverEndpoint, accessKey, secretKey string, withTLS bool) (S3Client, error) {
	logTags := log.Fields{
		"module":    "utils",
		"component": "s3-client",
		"instance":  serverEndpoint,
	}

	// Define the core minio client
	client, err := minio.New(serverEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: withTLS,
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

func (s *s3ClientImpl) GetObject(
	ctxt context.Context, bucketName, objectName string,
) ([]byte, error) {
	object, err := s.s3.GetObject(ctxt, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()
	content, err := io.ReadAll(object)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (s *s3ClientImpl) DeleteObject(ctxt context.Context, bucketName, objectKey string) error {
	return s.s3.RemoveObject(ctxt, bucketName, objectKey, minio.RemoveObjectOptions{})
}
