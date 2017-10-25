package aws_helper

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// ConvertS3Path returns an S3 compatible path
func ConvertS3Path(path string) (string, error) {
	if !strings.HasPrefix(path, "s3://") {
		return path, nil
	}

	parts := strings.Split(path[5:], "/")

	session, err := CreateAwsSession("us-east-1", "")
	if err != nil {
		return formatS3Path(parts[0], "", parts[1:]...), err
	}
	svc := s3.New(session)

	answer, err := svc.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String(parts[0])})
	if err != nil {
		return formatS3Path(parts[0], "", parts[1:]...), err
	}

	region := ""
	if answer.LocationConstraint != nil {
		region = "-" + *answer.LocationConstraint
	}

	return formatS3Path(parts[0], region, parts[1:]...), nil
}

func formatS3Path(bucket, region string, parts ...string) string {
	key := strings.Join(parts, "/")
	if key != "" {
		key = "/" + key
	}
	if region == "-us-east-1" {
		// us-east-1 is considered as the default storage for S3, it is not necessary to specify it
		// In fact, that caused a bug with terraform 0.10.3 and up (see https://github.com/hashicorp/terraform/issues/16442#issuecomment-339379748)
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
			regexp.MustCompile(`^https?://s3.amazonaws.com/(?P<bucket>[^/\.]+?)(?:/(?P<key>.*))?$`),
			regexp.MustCompile(`^https?://s3-(?P<region>.*?).amazonaws.com/(?P<bucket>[^/\.]+?)(?:/(?P<key>.*))?$`),
		}
	}

	convertedURL, _ := ConvertS3Path(url)
	if !strings.HasPrefix(convertedURL, "http") {
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
	return nil, fmt.Errorf("Non valid bucket url %s", url)
}

// BucketInfo represents the basic information relative to an S3 URL
type BucketInfo struct {
	BucketName string
	Region     string
	Key        string
}

type bucketStatus struct {
	BucketInfo
	Etag         string
	Version      string
	LastModified time.Time
}

var s3Patterns []*regexp.Regexp

// SaveS3Status save the current state of the S3 bucket folder in the directory
func SaveS3Status(url, folder string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%s %v", url, err)
		}
	}()
	bucketInfo, err := GetBucketObjectInfoFromURL(url)
	if err != nil {
		return
	}

	if !strings.HasSuffix(bucketInfo.Key, "/") {
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
	err = ioutil.WriteFile(filepath.Join(folder, cacheFile), jsonString, 0644)
	return
}

// CheckS3Status compares the saved status with the current version of the bucket folder
// returns true if the objects has not changed
func CheckS3Status(folder string) (bool, error) {
	content, err := ioutil.ReadFile(filepath.Join(folder, cacheFile))
	if err != nil {
		return false, fmt.Errorf("Error reading file %s/%s: %v", folder, cacheFile, err)
	}

	var status bucketStatus
	err = json.Unmarshal(content, &status)
	fmt.Println("status =", status, err)
	if err != nil {
		return false, fmt.Errorf("Content of %s/%s is not valid JSON %v", folder, cacheFile, err)
	}

	s3Status, err := getS3Status(status.BucketInfo)
	fmt.Println("s3Status =", status, err)
	if err != nil {
		return false, fmt.Errorf("Error while reading %v: %v", status.BucketInfo, err)
	}

	return reflect.DeepEqual(status, *s3Status), nil
}

const cacheFile = ".terragrunt.cache"

func getS3Status(info BucketInfo) (*bucketStatus, error) {
	session, err := CreateAwsSession(info.Region, "")
	if err != nil {
		return nil, err
	}
	svc := s3.New(session)

	answer, err := svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(info.BucketName),
		Key:    aws.String(info.Key),
	})
	if err != nil {
		return nil, err
	}

	return &bucketStatus{
		BucketInfo:   info,
		Etag:         *answer.ETag,
		Version:      *answer.VersionId,
		LastModified: *answer.LastModified,
	}, nil
}
