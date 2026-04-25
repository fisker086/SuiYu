---
name: s3-tool
description: AWS S3 operations: list buckets, list/get/put/delete objects, head object, generate presigned URLs. Supports access key/secret or IAM role.
activation_keywords: [s3, aws, storage, bucket, object, presigned, upload, download]
execution_mode: server
---

# S3 Tool Skill

**Credentials:** Omit `access_key` and `secret_key` to use the default AWS chain (environment variables, `~/.aws/credentials` / `AWS_PROFILE`, or IAM role on the host). If no credentials are available, the tool returns a short setup message instead of a raw SDK error—relay that to the user and do not ask them to paste secrets in chat.

Provides AWS S3 operations for storage management:
- **list_buckets**: List all S3 buckets
- **list_objects**: List objects in a bucket (with prefix filter)
- **get_object**: Get object content and metadata
- **put_object**: Upload object to bucket
- **delete_object**: Delete object from bucket
- **head_object**: Get object metadata without content
- **get_presigned_url**: Generate presigned URL for temporary access

Use `builtin_s3_tool` tool with fields:
- `operation`: Operation name (list_buckets, list_objects, get_object, put_object, delete_object, head_object, get_presigned_url)
- `region`: AWS region for the S3 client; **must match the bucket's region** (default `us-east-1` only if the bucket is in us-east-1). Mismatch yields `IllegalLocationConstraintException`—use e.g. `ap-east-1` for Hong Kong buckets.
- `access_key`: (optional) AWS access key ID
- `secret_key`: (optional) AWS secret access key
- `session_token`: (optional) AWS session token for temporary credentials
- `bucket`: S3 bucket name
- `key`: Object key/path
- `content`: Content to upload for put_object
- `content_base64`: Base64 encoded content for put_object
- `content_type`: Content-Type for put_object (e.g., application/json)
- `prefix`: Prefix filter for list_objects
- `max_keys`: Max objects to list (default: 100)
- `download`: Return full content for get_object (true/1)
- `expires`: Presigned URL expiry in seconds (default: 3600)

Example - list buckets:
```
operation: "list_buckets"
region: "us-east-1"
access_key: "AKIAIOSFODNN7EXAMPLE"
secret_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
```

Example - list objects:
```
operation: "list_objects"
bucket: "my-bucket"
prefix: "logs/"
max_keys: "50"
```

Example - put object:
```
operation: "put_object"
bucket: "my-bucket"
key: "data/file.json"
content: '{"key": "value"}'
content_type: "application/json"
```

Example - get presigned URL:
```
operation: "get_presigned_url"
bucket: "my-bucket"
key: "private/data.csv"
expires: "3600"
```