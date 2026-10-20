// Package awsaccess is the only part of tag-refactor that talks to AWS
// (contract C6). It builds the SDK configuration, applies time limits and the
// SDK's standard retries, and exposes read-only interfaces. The write
// interface is a separate type and has no implementation in U1.
package awsaccess
