package cdk

import (
	"os"
	"strings"
)

// The environment of cdk synth and of the helper (BR1.3, contract C5, security
// design §2). Every variable whose name starts with AWS_ and the variables of
// GitHub tokens are taken out, comparing the names in upper case (on Windows
// aws_profile and AWS_PROFILE are the same variable); the AWS files point at
// empty files; and the variables that stop the instance metadata, the version
// check and the telemetry of the CDK CLI are added. The rest stays: PATH,
// HOME, USERPROFILE and what Node.js needs.
//
// The names below are compared with, never read: this package does not read
// any token (NFR1.1).

// tokenVariables are the variables that can carry a GitHub token.
var tokenVariables = []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN",
	"TAG_REFACTOR_TEST_REPO_TOKEN"}

// addedVariables are set by isolate; a value of the parent for them goes.
var addedVariables = []string{"CDK_DISABLE_VERSION_CHECK", "CDK_DISABLE_CLI_TELEMETRY"}

// isolated is the environment of one call, with its two empty files.
type isolated struct {
	vars  []string
	files []string
}

// isolate makes the environment of a call from parent. The caller removes the
// empty files with close, whatever happens to the call.
func isolate(parent []string) (*isolated, error) {
	env := &isolated{}
	for _, kv := range parent {
		name, _, _ := strings.Cut(kv, "=")
		if !removed(name) {
			env.vars = append(env.vars, kv)
		}
	}
	for _, name := range []string{"AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE"} {
		f, err := os.CreateTemp("", "tag-refactor-aws-")
		if err != nil {
			env.close()
			return nil, err
		}
		env.files = append(env.files, f.Name())
		if err := f.Close(); err != nil {
			env.close()
			return nil, err
		}
		env.vars = append(env.vars, name+"="+f.Name())
	}
	env.vars = append(env.vars, "AWS_EC2_METADATA_DISABLED=true", "CDK_DISABLE_VERSION_CHECK=1", "CDK_DISABLE_CLI_TELEMETRY=true")
	return env, nil
}

// removed reports a variable that does not reach synth and the helper.
func removed(name string) bool {
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "AWS_") {
		return true
	}
	for _, n := range tokenVariables {
		if upper == n {
			return true
		}
	}
	for _, n := range addedVariables {
		if upper == n {
			return true
		}
	}
	return false
}

// names lists the names of the variables, for --debug: never their values.
func (e *isolated) names() string {
	names := make([]string, 0, len(e.vars))
	for _, kv := range e.vars {
		name, _, _ := strings.Cut(kv, "=")
		names = append(names, name)
	}
	return strings.Join(names, " ")
}

// close removes the empty files.
func (e *isolated) close() {
	for _, f := range e.files {
		_ = os.Remove(f)
	}
}
