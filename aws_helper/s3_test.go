package aws_helper

import (
	"reflect"
	"testing"
)

func TestConvertS3Path(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"Non S3 path", args{"/folder/test"}, "/folder/test", false},
		{"Non available S3", args{"s3://bucket/test"}, "bucket.s3.amazonaws.com/test", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertS3Path(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertS3Path() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertS3Path() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBucketObjectInfoFromURL(t *testing.T) {
	tests := []struct {
		url     string
		want    *BucketInfo
		wantErr bool
	}{
		{"/bucket", nil, true},
		{"s3://bucket", &BucketInfo{"bucket", "", ""}, false},
		{"s3://bucket/key", &BucketInfo{"bucket", "", "key"}, false},
		{"bucket.s3.amazonaws.com", &BucketInfo{"bucket", "", ""}, false},
		{"bucket.s3-us-east-1.amazonaws.com/key/file", &BucketInfo{"bucket", "us-east-1", "key/file"}, false},
		{"s3.amazonaws.com/bucket", &BucketInfo{"bucket", "", ""}, false},
		{"s3-us-east-1.amazonaws.com/bucket/key/file", &BucketInfo{"bucket", "us-east-1", "key/file"}, false},
		{"http://bucket.s3.amazonaws.com", &BucketInfo{"bucket", "", ""}, false},
		{"http://bucket.s3.amazonaws.com/key", &BucketInfo{"bucket", "", "key"}, false},
		{"https://bucket.s3.amazonaws.com", &BucketInfo{"bucket", "", ""}, false},
		{"https://bucket.s3.amazonaws.com/key", &BucketInfo{"bucket", "", "key"}, false},
		{"https://bucket.s3-us-east-1.amazonaws.com", &BucketInfo{"bucket", "us-east-1", ""}, false},
		{"https://bucket.s3-us-west-2.amazonaws.com/key", &BucketInfo{"bucket", "us-west-2", "key"}, false},
		{"https://s3-us-east-2.amazonaws.com/bucket", &BucketInfo{"bucket", "us-east-2", ""}, false},
	}
	for _, tt := range tests {
		t.Run(" ("+tt.url+")", func(t *testing.T) {
			got, err := GetBucketObjectInfoFromURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Error = %v, Expected %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Got %v, want %v", got, tt.want)
			}
		})
	}
}
