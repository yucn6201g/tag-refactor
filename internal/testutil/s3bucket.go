package testutil

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// S3Bucket is an in-memory S3 bucket for the run records of apply (U6
// BR5.2). It answers s3:PutObject, including the conditional write with
// If-None-Match: * that makes the lock, s3:GetObject, s3:DeleteObject and
// s3:GetBucketLocation, and keeps every write request so the tests can check
// that none of them sets an ACL (U6 NFR8.25).
type S3Bucket struct {
	Name   string
	Region string

	mu       sync.Mutex
	objects  map[string][]byte
	puts     []Call
	answer   func(c Call) *Reply
	ifAbsent int
}

// NewS3Bucket returns an empty bucket in region.
func NewS3Bucket(name, region string) *S3Bucket {
	return &S3Bucket{Name: name, Region: region, objects: map[string][]byte{}}
}

// Object returns the content of key and whether it exists.
func (b *S3Bucket) Object(key string) ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.objects[key]
	return append([]byte(nil), data...), ok
}

// PutObjectDirect stores an object without a request, as another computer
// would have.
func (b *S3Bucket) PutObjectDirect(key string, data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.objects[key] = append([]byte(nil), data...)
}

// Keys returns the keys of the objects, sorted.
func (b *S3Bucket) Keys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	keys := make([]string, 0, len(b.objects))
	for k := range b.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Puts returns the s3:PutObject requests received so far.
func (b *S3Bucket) Puts() []Call {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]Call(nil), b.puts...)
}

// ConditionalStatus makes a conditional write (If-None-Match: *) answer
// status whatever the bucket holds: 412 PreconditionFailed, 409
// ConditionalRequestConflict or 501 NotImplemented. 0 goes back to the real
// behavior.
func (b *S3Bucket) ConditionalStatus(status int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ifAbsent = status
}

// ReplyInstead makes f decide the answer of a request to the bucket: a
// non-nil reply is given instead, and nothing changes.
func (b *S3Bucket) ReplyInstead(f func(c Call) *Reply) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.answer = f
}

// HandleS3Buckets makes the fake answer the object requests of the buckets
// and their locations, and allows the writes. A location request for another
// bucket goes to the handler registered before, if any.
func (f *FakeAWS) HandleS3Buckets(buckets ...*S3Bucket) {
	f.AllowWrites()
	byName := map[string]*S3Bucket{}
	for _, b := range buckets {
		byName[b.Name] = b
	}
	noSuchBucket := func(Call) Reply { return S3Error(http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist") }
	route := func(op string, h func(b *S3Bucket, c Call) Reply) {
		other := f.handler(op)
		if other == nil {
			other = noSuchBucket
		}
		f.Handle(op, func(c Call) Reply {
			b, ok := byName[c.Bucket]
			if !ok {
				return other(c)
			}
			if r := b.replyInstead(c); r != nil {
				return *r
			}
			return h(b, c)
		})
	}
	route(OpGetBucketLocation, func(b *S3Bucket, _ Call) Reply { return BucketLocation(b.Region) })
	route(OpPutObject, (*S3Bucket).put)
	route(OpGetObject, (*S3Bucket).get)
	route(OpDeleteObject, (*S3Bucket).delete)
}

func (b *S3Bucket) replyInstead(c Call) *Reply {
	b.mu.Lock()
	f := b.answer
	b.mu.Unlock()
	if f == nil {
		return nil
	}
	return f(c)
}

func (b *S3Bucket) put(c Call) Reply {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.puts = append(b.puts, c)
	if c.Key == "" {
		return S3Error(http.StatusBadRequest, "InvalidRequest", "no key")
	}
	if c.Header.Get("If-None-Match") != "" {
		if c.Header.Get("If-None-Match") != "*" {
			return S3Error(http.StatusNotImplemented, "NotImplemented", "only * is supported")
		}
		switch b.ifAbsent {
		case http.StatusPreconditionFailed:
			return S3Error(b.ifAbsent, "PreconditionFailed", "At least one of the pre-conditions you specified did not hold")
		case http.StatusConflict:
			return S3Error(b.ifAbsent, "ConditionalRequestConflict", "A conflicting conditional operation is in progress")
		case http.StatusNotImplemented:
			return S3Error(b.ifAbsent, "NotImplemented", "A header you provided implies functionality that is not implemented")
		}
		if _, exists := b.objects[c.Key]; exists {
			return S3Error(http.StatusPreconditionFailed, "PreconditionFailed", "At least one of the pre-conditions you specified did not hold")
		}
	}
	b.objects[c.Key] = append([]byte(nil), c.Body...)
	return Reply{Header: map[string]string{"ETag": fmt.Sprintf(`"%d"`, len(b.puts))}}
}

func (b *S3Bucket) get(c Call) Reply {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.objects[c.Key]
	if !ok {
		return S3Error(http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
	}
	return Reply{Body: string(data), ContentType: "application/octet-stream"}
}

func (b *S3Bucket) delete(c Call) Reply {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.objects, c.Key)
	return Reply{Status: http.StatusNoContent}
}

// HasACL reports whether a request sets an ACL, with a canned ACL or a grant
// header.
func HasACL(c Call) bool {
	for name := range c.Header {
		lower := strings.ToLower(name)
		if lower == "x-amz-acl" || strings.HasPrefix(lower, "x-amz-grant-") {
			return true
		}
	}
	return c.Query.Has("acl")
}
