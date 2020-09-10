package awshelper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
)

// ErrS3PathNotFoundError is the error that will be returned if an object doesn't exist in S3
var ErrS3PathNotFoundError = errors.New("HeadObject failed on the given object")

// CacheFile is the name of the file that hold the S3 source configuration for the folder
const CacheFile = ".terragrunt.cache"

var bucketRegionsCache sync.Map

// ConvertS3Path returns an S3 compatible path
func ConvertS3Path(path string) (string, error) {
	if !strings.HasPrefix(path, "s3://") {
		return path, nil
	}

	parts := strings.Split(path[5:], "/")

	region, err := getBucketRegion(parts[0])
	if err != nil {
		return formatS3Path(parts[0], "", parts[1:]...), err
	}

	return formatS3Path(parts[0], region, parts[1:]...), nil
}

func formatS3Path(bucket, region string, parts ...string) string {
	key := strings.Join(parts, "/")
	if key != "" {
		key = "/" + key
	}
	if region != "" && region != "us-east-1" {
		// us-east-1 is considered as the default storage for S3, it is not necessary to specify it
		// In fact, that caused a bug with terraform 0.10.3 and up (see https://github.com/hashicorp/terraform/issues/16442#issuecomment-339379748)
		region = "-" + region
	} else {
		region = ""
	}
	return fmt.Sprintf("%s.s3%s.amazonaws.com%s", bucket, region, key)
}

// GetBucketObjectInfoFromURL retrieve the components of the bucket (name, key, region) from an URL
func GetBucketObjectInfoFromURL(url string) (*BucketInfo, error) {
	if s3Patterns == nil {
		s3Patterns = []*regexp.Regexp{
			regexp.MustCompile(`^https?://(?P<bucket>[^/\.]+?).s3.amazonaws.com(?:/(?P<key>.*))?$`),
			regexp.MustCompile(`^https?://(?P<bucket>[^/\.]+?).s3-(?P<region>.*?).amazonaws.com(?:/(?P<key>.*))?$`),
			regexp.MustCompile(`^(s3::)?https?://s3.amazonaws.com/(?P<bucket>[^/\.]+?)(?:/(?P<key>.*))?$`),
			regexp.MustCompile(`^(s3::)?https?://s3-(?P<region>.*?).amazonaws.com/(?P<bucket>[^/\.]+?)(?:/(?P<key>.*))?$`),
		}
	}

	convertedURL, _ := ConvertS3Path(url)
	if !strings.HasPrefix(convertedURL, "http") && !strings.HasPrefix(convertedURL, "s3::") {
		convertedURL = "https://" + convertedURL
	}

	for _, pattern := range s3Patterns {
		matches := pattern.FindStringSubmatch(convertedURL)
		if matches != nil {
			result := &BucketInfo{}
			for i, part := range pattern.SubexpNames() {
				switch part {
				case "bucket":
					result.BucketName = matches[i]
				case "key":
					result.Key = matches[i]
				case "region":
					result.Region = matches[i]
				}
			}

			return result, nil
		}
	}
	return nil, fmt.Errorf("non valid bucket url %s", url)
}

// BucketInfo represents the basic information relative to an S3 URL
type BucketInfo struct {
	BucketName string
	Region     string
	Key        string
}

// Path returns a valid path from a bucket object
func (b *BucketInfo) String() string {
	return formatS3Path(b.BucketName, b.Region, b.Key)
}

type bucketStatus struct {
	BucketInfo
	Etag         string
	Version      string
	LastModified time.Time
}

var s3Patterns []*regexp.Regexp

// SaveS3Status save the current state of the S3 bucket folder in the directory
func SaveS3Status(bucketInfo *BucketInfo, folder string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%s %w", bucketInfo, err)
		}
	}()

	if !strings.HasSuffix(bucketInfo.Key, "/") && !strings.HasSuffix(bucketInfo.Key, ".zip") {
		bucketInfo.Key += "/"
	}

	status, err := getS3Status(*bucketInfo)
	if err != nil {
		return
	}

	jsonString, err := json.Marshal(status)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(filepath.Join(folder, CacheFile), jsonString, 0644)
	return
}

// CheckS3Status compares the saved status with the current version of the bucket folder
// returns true if the objects has not changed
func CheckS3Status(sourceBucketInfo *BucketInfo, folder string) error {
	// Check for the status of the object in S3. If it doesn't exist in S3, there's no point in reading the local/saved status
	s3Status, err := getS3Status(*sourceBucketInfo)
	if err != nil {
		var awsError awserr.Error
		if errors.As(err, &awsError) {
			if awsError.Code() == "NotFound" {
				return fmt.Errorf("%s does not exist: %w", *sourceBucketInfo, ErrS3PathNotFoundError)
			}
		}
		return fmt.Errorf("error while reading %s: %w", *sourceBucketInfo, err)
	}

	content, err := ioutil.ReadFile(filepath.Join(folder, CacheFile))
	if err != nil {
		return err
	}

	var status bucketStatus
	err = json.Unmarshal(content, &status)
	if err != nil {
		return fmt.Errorf("content of %s/%s (%s) is not valid JSON: %w", folder, CacheFile, content, err)
	}

	if !reflect.DeepEqual(status, *s3Status) {
		return fmt.Errorf("checksum changed for %s", status.BucketInfo)
	}
	return nil
}

func getS3Status(info BucketInfo) (*bucketStatus, error) {
	var err error
	if info.Region == "" {
		info.Region, err = getBucketRegion(info.BucketName)
		if err != nil {
			return nil, err
		}
	}
	session, err := CreateAwsSession(info.Region, "")
	if err != nil {
		return nil, err
	}
	svc := s3.New(session)

	answer, err := svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(info.BucketName),
		Key:    aws.String(info.Key),
	})
	if !strings.HasSuffix(info.Key, "/") {
		// Retrying with / at the end
		answer, err = svc.HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(info.BucketName),
			Key:    aws.String(info.Key + "/"),
		})
	}
	if err != nil {
		clearSessionCache()
		return nil, fmt.Errorf("caught error while calling HeadObject on key %s (with or without slash) of bucket %s: %w", info.Key, info.BucketName, err)
	}

	return &bucketStatus{
		BucketInfo:   info,
		Etag:         *answer.ETag,
		Version:      *answer.VersionId,
		LastModified: *answer.LastModified,
	}, nil
}

func getBucketRegion(bucketName string) (string, error) {
	if region, ok := bucketRegionsCache.Load(bucketName); ok {
		return region.(string), nil
	}

	session, err := CreateAwsSession("", "")
	if err != nil {
		return "", err
	}
	svc := s3.New(session)

	result, err := svc.GetBucketLocationWithContext(
		context.Background(),
		&s3.GetBucketLocationInput{
			Bucket: aws.String(bucketName),
		},
		s3.WithNormalizeBucketLocation,
	)
	if err != nil {
		return "", err
	}
	region := *result.LocationConstraint
	bucketRegionsCache.Store(bucketName, region)
	return region, nil
}
