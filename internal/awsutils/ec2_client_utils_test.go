package awsutils

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

func TestEncodeUserData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple string", "hello", base64.StdEncoding.EncodeToString([]byte("hello"))},
		{"empty string", "", ""},
		{"json data", `{"key":"value"}`, base64.StdEncoding.EncodeToString([]byte(`{"key":"value"}`))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeUserData(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestReplaceHTMLEscapes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"less than", `\u003c`, "<"},
		{"greater than", `\u003e`, ">"},
		{"ampersand", `\u0026`, "&"},
		{"mixed", `a\u003cb\u003ec\u0026d`, "a<b>c&d"},
		{"no escapes", "hello world", "hello world"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceHTMLEscapes(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// mockS3API implements S3API for testing
type mockS3API struct {
	presignGetObjectFunc func(ctx context.Context, input *s3.GetObjectInput) (*v4.PresignedHTTPRequest, error)
}

func (m *mockS3API) PresignGetObject(ctx context.Context, input *s3.GetObjectInput) (*v4.PresignedHTTPRequest, error) {
	if m.presignGetObjectFunc != nil {
		return m.presignGetObjectFunc(ctx, input)
	}
	return &v4.PresignedHTTPRequest{URL: "https://example.com/signed"}, nil
}

func TestSignURL(t *testing.T) {
	mock := &mockS3API{
		presignGetObjectFunc: func(ctx context.Context, input *s3.GetObjectInput) (*v4.PresignedHTTPRequest, error) {
			if *input.Bucket != "my-bucket" {
				t.Errorf("expected bucket 'my-bucket', got %q", *input.Bucket)
			}
			if *input.Key != "my-key" {
				t.Errorf("expected key 'my-key', got %q", *input.Key)
			}
			return &v4.PresignedHTTPRequest{URL: "https://s3.amazonaws.com/my-bucket/my-key?signed=true"}, nil
		},
	}

	bucket := "my-bucket"
	key := "my-key"
	url, err := SignURL(context.Background(), mock, &bucket, &key)
	if err != nil {
		t.Fatalf("SignURL failed: %v", err)
	}
	if url != "https://s3.amazonaws.com/my-bucket/my-key?signed=true" {
		t.Errorf("unexpected URL: %q", url)
	}
}

func TestEC2RunInstancesUtil(t *testing.T) {
	mock := &mockEC2API{
		runInstancesFunc: func(ctx context.Context, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
			if *input.ImageId != "ami-12345" {
				t.Errorf("expected ImageId 'ami-12345', got %q", *input.ImageId)
			}
			if string(input.InstanceType) != "t3.medium" {
				t.Errorf("expected InstanceType 't3.medium', got %q", input.InstanceType)
			}
			if *input.SubnetId != "subnet-abc" {
				t.Errorf("expected SubnetId 'subnet-abc', got %q", *input.SubnetId)
			}
			if input.KeyName == nil {
				t.Error("expected KeyName to be set")
			}
			return &ec2.RunInstancesOutput{
				Instances: []types.Instance{
					{
						InstanceId:       aws.String("i-new"),
						PrivateIpAddress: aws.String("10.0.0.1"),
					},
				},
			}, nil
		},
	}

	resp, err := EC2RunInstancesUtil(
		context.Background(),
		"my-profile",
		"ami-12345",
		"t3.medium",
		"my-keypair",
		[]string{"sg-1"},
		"subnet-abc",
		nil,
		"userdata",
		mock,
	)
	if err != nil {
		t.Fatalf("EC2RunInstancesUtil failed: %v", err)
	}
	if *resp.Instances[0].InstanceId != "i-new" {
		t.Errorf("expected instance id 'i-new', got %q", *resp.Instances[0].InstanceId)
	}
}

func TestEC2RunInstancesUtilNoKeyPair(t *testing.T) {
	mock := &mockEC2API{
		runInstancesFunc: func(ctx context.Context, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
			if input.KeyName != nil {
				t.Error("expected KeyName to be nil when empty string passed")
			}
			return &ec2.RunInstancesOutput{
				Instances: []types.Instance{
					{
						InstanceId:       aws.String("i-new"),
						PrivateIpAddress: aws.String("10.0.0.1"),
					},
				},
			}, nil
		},
	}

	_, err := EC2RunInstancesUtil(
		context.Background(),
		"my-profile",
		"ami-12345",
		"t3.medium",
		"", // no key pair
		[]string{"sg-1"},
		"subnet-abc",
		nil,
		"userdata",
		mock,
	)
	if err != nil {
		t.Fatalf("EC2RunInstancesUtil failed: %v", err)
	}
}
