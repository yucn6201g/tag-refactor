package awsaccess

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// The run records in S3 (contract C6 ObjectStore, U6 BR5.2). Every call goes
// through invoke. No call sets an ACL, and the encryption is the bucket's
// default (U6 NFR8.25).

// ParseS3URI splits s3://bucket/key into the bucket and the key.
func ParseS3URI(uri string) (bucket, key string, ok bool) {
	rest, found := strings.CutPrefix(uri, "s3://")
	if !found {
		return "", "", false
	}
	bucket, key, _ = strings.Cut(rest, "/")
	return bucket, key, bucket != "" && key != ""
}

// object names the bucket, key and region of uri, and returns the S3 client
// of that region. The region of a bucket is read once
// (s3:GetBucketLocation) and kept.
func (w *Writer) object(ctx context.Context, op, uri string) (*s3.Client, string, string, string, error) {
	bucket, key, ok := ParseS3URI(uri)
	if !ok {
		return nil, "", "", "", &errkind.Error{Kind: errkind.InvalidInput, Op: op, Subject: uri, Detail: "not an S3 URI of an object"}
	}
	w.mu.Lock()
	region, known := w.bucketRegions[bucket]
	w.mu.Unlock()
	if !known {
		r, err := w.s.GetBucketRegion(ctx, bucket)
		if err != nil {
			return nil, "", "", "", storeError(err, uri)
		}
		region = r
		w.mu.Lock()
		w.bucketRegions[bucket] = region
		w.mu.Unlock()
	}
	return w.s.clientsIn(region).s3, bucket, key, region, nil
}

// storeError keeps the kinds the callers of the store act on (AccessDenied,
// NotFound, Conflict, CredentialsMissing) and makes every other failure
// StorageFailed (contract C6), naming the object.
func storeError(err error, uri string) error {
	e := errkind.Ensure(err, "")
	switch e.Kind {
	case errkind.AccessDenied, errkind.NotFound, errkind.Conflict, errkind.CredentialsMissing, errkind.StorageFailed:
	default:
		e = &errkind.Error{Kind: errkind.StorageFailed, Reason: e.Reason, Op: e.Op, Region: e.Region, Detail: e.Detail, Cause: e}
	}
	if e.Subject == "" {
		e.Subject = uri
	}
	return e
}

// Put replaces the object in one s3:PutObject.
func (w *Writer) Put(ctx context.Context, uri string, data []byte) error {
	return w.put(ctx, uri, data, false)
}

// PutIfAbsent writes the object with If-None-Match: *, so S3 refuses it when
// the name exists (412 PreconditionFailed) or another conditional write of
// the name is running (409 ConditionalRequestConflict): both are Conflict. A
// store without conditional writes (501 NotImplemented) is StorageFailed.
func (w *Writer) PutIfAbsent(ctx context.Context, uri string, data []byte) error {
	return w.put(ctx, uri, data, true)
}

func (w *Writer) put(ctx context.Context, uri string, data []byte, ifAbsent bool) error {
	client, bucket, key, region, err := w.object(ctx, OpPutObject, uri)
	if err != nil {
		return err
	}
	in := &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))), ContentType: aws.String("application/json")}
	if ifAbsent {
		in.IfNoneMatch = aws.String("*")
	}
	err = w.s.invoke(ctx, OpPutObject, region, func(ctx context.Context, add apiOption) error {
		_, err := client.PutObject(ctx, in, func(o *s3.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return storeError(err, uri)
	}
	return nil
}

// Get reads the object; a missing object or bucket is NotFound.
func (w *Writer) Get(ctx context.Context, uri string) ([]byte, error) {
	client, bucket, key, region, err := w.object(ctx, OpGetObject, uri)
	if err != nil {
		return nil, err
	}
	var data []byte
	err = w.s.invoke(ctx, OpGetObject, region, func(ctx context.Context, add apiOption) error {
		out, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)},
			func(o *s3.Options) { o.APIOptions = append(o.APIOptions, add) })
		if err != nil {
			return err
		}
		defer func() { _ = out.Body.Close() }()
		data, err = io.ReadAll(out.Body)
		return err
	})
	if err != nil {
		return nil, storeError(err, uri)
	}
	return data, nil
}

// Delete removes the object (s3:DeleteObject).
func (w *Writer) Delete(ctx context.Context, uri string) error {
	client, bucket, key, region, err := w.object(ctx, OpDeleteObject, uri)
	if err != nil {
		return err
	}
	err = w.s.invoke(ctx, OpDeleteObject, region, func(ctx context.Context, add apiOption) error {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)},
			func(o *s3.Options) { o.APIOptions = append(o.APIOptions, add) })
		return err
	})
	if err != nil {
		return storeError(err, uri)
	}
	return nil
}
