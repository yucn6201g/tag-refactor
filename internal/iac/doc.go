// Package iac reflects a saved plan (contract C2) in the infrastructure code
// that defines the resources: CloudFormation templates (YAML and JSON) and the
// root module of a Terraform configuration (U8). It never calls AWS, terraform
// or a stack update; it reads files, changes only the byte ranges of the tags,
// and writes only with --write, after git has said the files are committed.
package iac
