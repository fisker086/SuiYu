package skills

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolS3Tool = "builtin_s3_tool"

var allowedS3Ops = map[string]bool{
	"list_buckets":      true,
	"list_objects":      true,
	"get_object":        true,
	"put_object":        true,
	"delete_object":     true,
	"head_object":       true,
	"get_presigned_url": true,
}

// awsCredentialMissingUserMessage is returned as tool output (nil error) so the model can
// guide the user to configure credentials without exposing raw SDK errors.
func awsCredentialMissingUserMessage() string {
	return "[S3] AWS credentials are not available. Configure credentials on this runtime: " +
		"set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY (and AWS_SESSION_TOKEN if using temporary creds), " +
		"or use ~/.aws/credentials with AWS_PROFILE, or attach an IAM role (EC2/ECS/Lambda). " +
		"Do not paste access keys into chat. After configuring, retry the S3 operation."
}

func isLikelyCredentialError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "failed to refresh cached credentials") ||
		strings.Contains(s, "static credentials are empty") ||
		strings.Contains(s, "NoCredentialProviders") ||
		strings.Contains(s, "could not load credentials") ||
		strings.Contains(s, "get credentials")
}

// s3RegionMismatchUserMessage handles IllegalLocationConstraintException when the client
// region (e.g. default us-east-1) does not match the bucket's region.
func s3RegionMismatchUserMessage(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	s := err.Error()
	if !strings.Contains(s, "IllegalLocationConstraintException") {
		return "", false
	}
	bucketRegion := ""
	const prefix = "The "
	const suffix = " location constraint"
	if i := strings.Index(s, prefix); i >= 0 {
		rest := s[i+len(prefix):]
		if j := strings.Index(rest, suffix); j > 0 {
			candidate := strings.TrimSpace(rest[:j])
			if len(candidate) >= 6 && strings.Contains(candidate, "-") {
				bucketRegion = candidate
			}
		}
	}
	if bucketRegion != "" {
		return fmt.Sprintf("[S3] The bucket is in region %q but the request used a different region. "+
			"Set `region` to %q and retry. (Default `us-east-1` only applies to buckets in us-east-1.) "+
			"You can confirm the region in S3 console → bucket → Properties → AWS Region.",
			bucketRegion, bucketRegion), true
	}
	return "[S3] The bucket's region does not match the client region. Set `region` to the bucket's AWS Region " +
		"(S3 console → bucket → Properties) and retry; default us-east-1 only works for buckets in us-east-1.", true
}

// s3InvokeErrorOutcome returns a user-facing tool message when err should not surface as a raw SDK failure.
func s3InvokeErrorOutcome(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	if isLikelyCredentialError(err) {
		return awsCredentialMissingUserMessage(), true
	}
	if msg, ok := s3RegionMismatchUserMessage(err); ok {
		return msg, true
	}
	return "", false
}

func execBuiltinS3Tool(ctx context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		return "", fmt.Errorf("missing operation")
	}

	if !allowedS3Ops[op] {
		return "", fmt.Errorf("operation %q not allowed (allowed: %v)", op, allowedS3Ops)
	}

	region := strArg(in, "region", "aws_region")
	if region == "" {
		region = "us-east-1"
	}

	accessKey := strArg(in, "access_key", "aws_access_key")
	secretKey := strArg(in, "secret_key", "aws_secret_key")
	awsSessionToken := strArg(in, "session_token", "token")
	bucket := strArg(in, "bucket", "bucket_name")

	if (accessKey != "" && secretKey == "") || (accessKey == "" && secretKey != "") {
		return "", fmt.Errorf("provide both access_key and secret_key, or omit both to use the default AWS credential chain (env, ~/.aws/credentials, IAM role)")
	}

	loadOpts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if accessKey != "" && secretKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, awsSessionToken)))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	creds, cerr := cfg.Credentials.Retrieve(ctx)
	if cerr != nil || !creds.HasKeys() {
		return awsCredentialMissingUserMessage(), nil
	}

	client := s3.NewFromConfig(cfg)

	switch op {
	case "list_buckets":
		resp, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
		if err != nil {
			if msg, ok := s3InvokeErrorOutcome(err); ok {
				return msg, nil
			}
			return "", fmt.Errorf("list buckets failed: %w", err)
		}
		if len(resp.Buckets) == 0 {
			return "No S3 buckets found", nil
		}
		var b strings.Builder
		b.WriteString("S3 Buckets:\n\n")
		for _, bkt := range resp.Buckets {
			b.WriteString(fmt.Sprintf("  - %s\n", *bkt.Name))
		}
		return b.String(), nil

	case "list_objects":
		if bucket == "" {
			return "", fmt.Errorf("missing bucket")
		}
		prefix := strArg(in, "prefix", "path")
		maxKeys := strArg(in, "max_keys", "limit")
		max := int32(100)
		if maxKeys != "" {
			var m int64
			fmt.Sscanf(maxKeys, "%d", &m)
			max = int32(m)
		}

		input := &s3.ListObjectsV2Input{
			Bucket:  aws.String(bucket),
			MaxKeys: aws.Int32(max),
		}
		if prefix != "" {
			input.Prefix = aws.String(prefix)
		}

		resp, err := client.ListObjectsV2(ctx, input)
		if err != nil {
			if msg, ok := s3InvokeErrorOutcome(err); ok {
				return msg, nil
			}
			return "", fmt.Errorf("list objects failed: %w", err)
		}
		if len(resp.Contents) == 0 {
			return fmt.Sprintf("No objects found in bucket '%s'", bucket), nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Objects in s3://%s (prefix: '%s'):\n\n", bucket, prefix))
		for _, obj := range resp.Contents {
			size := *obj.Size
			b.WriteString(fmt.Sprintf("  %s (%.2f KB, %s)\n", *obj.Key, float64(size)/1024, obj.LastModified.Format("2006-01-02 15:04")))
		}
		if resp.IsTruncated != nil && *resp.IsTruncated {
			b.WriteString("\n  (more objects exist, increase max_keys to see more)")
		}
		return b.String(), nil

	case "get_object":
		if bucket == "" {
			return "", fmt.Errorf("missing bucket")
		}
		key := strArg(in, "key", "object_key", "path")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		download := strArg(in, "download", "save")
		bytesRange := strArg(in, "range")

		input := &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}
		if bytesRange != "" {
			input.Range = aws.String(bytesRange)
		}

		resp, err := client.GetObject(ctx, input)
		if err != nil {
			if msg, ok := s3InvokeErrorOutcome(err); ok {
				return msg, nil
			}
			return "", fmt.Errorf("get object failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read object body: %w", err)
		}

		if download == "true" || download == "1" {
			return fmt.Sprintf("Object s3://%s/%s (size: %d bytes):\n\n[Object content - base64 encoded]\n%s",
				bucket, key, len(body), base64.StdEncoding.EncodeToString(body)), nil
		}

		contentType := ""
		if resp.ContentType != nil {
			contentType = *resp.ContentType
		}

		maxSize := 50 * 1024
		if len(body) > maxSize {
			body = body[:maxSize]
			return fmt.Sprintf("Object s3://%s/%s\nContent-Type: %s\nSize: %d bytes\n\n%s\n\n[... truncated to %d bytes]",
				bucket, key, contentType, len(body), string(body), maxSize), nil
		}

		return fmt.Sprintf("Object s3://%s/%s\nContent-Type: %s\nSize: %d bytes\n\n%s",
			bucket, key, contentType, len(body), string(body)), nil

	case "put_object":
		if bucket == "" {
			return "", fmt.Errorf("missing bucket")
		}
		key := strArg(in, "key", "object_key", "path")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		content := strArg(in, "content", "body", "data")
		if content == "" {
			contentBase64 := strArg(in, "content_base64")
			if contentBase64 != "" {
				decoded, err := base64.StdEncoding.DecodeString(contentBase64)
				if err != nil {
					return "", fmt.Errorf("invalid base64 content: %w", err)
				}
				content = string(decoded)
			}
			if content == "" {
				return "", fmt.Errorf("missing content or content_base64")
			}
		}
		contentType := strArg(in, "content_type", "mime_type")

		input := &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   strings.NewReader(content),
		}
		if contentType != "" {
			input.ContentType = aws.String(contentType)
		}

		_, err = client.PutObject(ctx, input)
		if err != nil {
			if msg, ok := s3InvokeErrorOutcome(err); ok {
				return msg, nil
			}
			return "", fmt.Errorf("put object failed: %w", err)
		}
		return fmt.Sprintf("Object s3://%s/%s: uploaded successfully", bucket, key), nil

	case "delete_object":
		if bucket == "" {
			return "", fmt.Errorf("missing bucket")
		}
		key := strArg(in, "key", "object_key", "path")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}

		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			if msg, ok := s3InvokeErrorOutcome(err); ok {
				return msg, nil
			}
			return "", fmt.Errorf("delete object failed: %w", err)
		}
		return fmt.Sprintf("Object s3://%s/%s: deleted successfully", bucket, key), nil

	case "head_object":
		if bucket == "" {
			return "", fmt.Errorf("missing bucket")
		}
		key := strArg(in, "key", "object_key", "path")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}

		resp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			if msg, ok := s3InvokeErrorOutcome(err); ok {
				return msg, nil
			}
			return "", fmt.Errorf("head object failed: %w", err)
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("Object s3://%s/%s:\n\n", bucket, key))
		b.WriteString(fmt.Sprintf("  Size: %d bytes\n", *resp.ContentLength))
		b.WriteString(fmt.Sprintf("  Content-Type: %s\n", *resp.ContentType))
		if resp.LastModified != nil {
			b.WriteString(fmt.Sprintf("  Last-Modified: %s\n", resp.LastModified.Format("2006-01-02 15:04:05")))
		}
		if resp.ETag != nil {
			b.WriteString(fmt.Sprintf("  ETag: %s\n", *resp.ETag))
		}
		if resp.Metadata != nil {
			b.WriteString("  Metadata:\n")
			for k, v := range resp.Metadata {
				b.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
			}
		}
		return b.String(), nil

	case "get_presigned_url":
		if bucket == "" {
			return "", fmt.Errorf("missing bucket")
		}
		key := strArg(in, "key", "object_key", "path")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		expires := strArg(in, "expires", "expire_seconds")
		expireSec := 3600
		if expires != "" {
			fmt.Sscanf(expires, "%d", &expireSec)
		}

		presignClient := s3.NewPresignClient(client)
		url, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(time.Duration(expireSec)*time.Second))
		if err != nil {
			if msg, ok := s3InvokeErrorOutcome(err); ok {
				return msg, nil
			}
			return "", fmt.Errorf("presign failed: %w", err)
		}
		return fmt.Sprintf("Presigned URL for s3://%s/%s (expires in %ds):\n\n%s", bucket, key, expireSec, url.URL), nil

	default:
		return "", fmt.Errorf("unsupported operation: %s", op)
	}
}

func NewBuiltinS3Tool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolS3Tool,
			Desc: "AWS S3 operations: list buckets, list/get/put/delete objects, head object, generate presigned URLs. Supports access key/secret or IAM role.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":      {Type: einoschema.String, Desc: "Operation: list_buckets, list_objects, get_object, put_object, delete_object, head_object, get_presigned_url", Required: true},
				"region":         {Type: einoschema.String, Desc: "AWS region for S3 API endpoint; must match the bucket's region (default: us-east-1). Wrong region causes IllegalLocationConstraintException.", Required: false},
				"access_key":     {Type: einoschema.String, Desc: "AWS access key ID", Required: false},
				"secret_key":     {Type: einoschema.String, Desc: "AWS secret access key", Required: false},
				"session_token":  {Type: einoschema.String, Desc: "AWS session token (for temporary credentials)", Required: false},
				"bucket":         {Type: einoschema.String, Desc: "S3 bucket name", Required: false},
				"key":            {Type: einoschema.String, Desc: "Object key/path", Required: false},
				"content":        {Type: einoschema.String, Desc: "Content to upload for put_object", Required: false},
				"content_base64": {Type: einoschema.String, Desc: "Base64 encoded content for put_object", Required: false},
				"content_type":   {Type: einoschema.String, Desc: "Content-Type for put_object (e.g., application/json)", Required: false},
				"prefix":         {Type: einoschema.String, Desc: "Prefix filter for list_objects", Required: false},
				"max_keys":       {Type: einoschema.String, Desc: "Max objects to list (default: 100)", Required: false},
				"download":       {Type: einoschema.String, Desc: "Return full content for get_object (true/1)", Required: false},
				"expires":        {Type: einoschema.String, Desc: "Presigned URL expiry in seconds (default: 3600)", Required: false},
			}),
		},
		execBuiltinS3Tool,
	)
}
