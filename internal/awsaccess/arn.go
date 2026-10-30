package awsaccess

import "strings"

// ARN is an ARN in its six parts:
// arn:<partition>:<service>:<region>:<account>:<resource>. The resource may
// hold colons; the region and the account are empty for some services, such
// as S3 buckets (U7 security design §2).
type ARN struct {
	Partition, Service, Region, Account, Resource string
}

// ParseARN splits an ARN on its first five colons, as the inventory reads the
// region of an ARN. It reports false when arn does not start with "arn:" or
// has fewer than six parts; it does not check that the parts are not empty.
func ParseARN(arn string) (ARN, bool) {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 || parts[0] != "arn" {
		return ARN{}, false
	}
	return ARN{Partition: parts[1], Service: parts[2], Region: parts[3], Account: parts[4], Resource: parts[5]}, true
}
