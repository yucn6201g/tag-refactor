package rules

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yucn6201g/tag-refactor/internal/inventory"
)

// Performance tests of U3 (NFR1.11, NFR1.12). They run on the three
// operating systems in CI; under the race detector the input is ten times
// smaller and the time limit ten times longer.

// NFR1.11: from the collection to the bytes of the draft, 10,000 resources
// and 500 keys: half of the keys with 50 different values (no candidates),
// half with at most 20 values written two or three ways, so the draft goes
// past 64 KiB again and again and the adjustment works.
func TestPerformanceU3_NFR1_11_SuggestIsLinear(t *testing.T) {
	resources, keys := 10_000/perfScale, 500/perfScale
	in := suggestInput(suggestLoad(resources, keys)...)

	start := time.Now()
	d := Suggest(in)
	elapsed := time.Since(start)

	limit := 5 * time.Second * timeLimitFactor
	t.Logf("NFR1.11: %d resources, %d keys: %v (limit %v), %d bytes", resources, keys, elapsed, limit, len(d.Text))
	if elapsed > limit {
		t.Fatalf("rules suggest took %v, limit %v", elapsed, limit)
	}
	tooMany, omitted, kept := 0, 0, 0
	for _, k := range d.Keys {
		switch k.ValuesOmitted {
		case OmitTooManyValues:
			tooMany++
		case OmitSizeLimit:
			omitted++
		default:
			if len(k.Values) > 0 {
				kept++
			}
		}
	}
	if len(d.Keys) != keys || tooMany != keys/2 || len(d.Text) > MaxFileSize || d.TooLarge {
		t.Fatalf("%d keys, %d with too many values, %d bytes", len(d.Keys), tooMany, len(d.Text))
	}
	if perfScale == 1 && (omitted == 0 || kept == 0) {
		t.Fatalf("the adjustment must work: %d omitted, %d kept", omitted, kept)
	}
}

// suggestLoad builds the resources of NFR1.11: each resource carries ten of
// the keys; each key is written in three ways.
func suggestLoad(resources, keys int) []TaggedResource {
	out := make([]TaggedResource, 0, resources)
	for i := 0; i < resources; i++ {
		tags := make(map[string]string, 10)
		for j := 0; j < 10; j++ {
			k := (i*7 + j*(keys/10+1)) % keys
			var key string
			switch i % 3 {
			case 0:
				key = fmt.Sprintf("app-%03d", k)
			case 1:
				key = fmt.Sprintf("App_%03d", k)
			default:
				key = fmt.Sprintf("app %03d", k)
			}
			var value string
			if k%2 == 0 {
				// A key is on every keys-th resource, so the value moves along
				// the resources and with j to reach 50 values per key.
				value = fmt.Sprintf("identifier-%02d", (i*50/resources+j*5)%50)
			} else {
				spellings := []string{"service-name-%d", "Service_Name_%d", "SERVICE-NAME-%d"}
				value = fmt.Sprintf(spellings[i%3], (i/3)%6)
			}
			tags[key] = value
		}
		out = append(out, TaggedResource{ARN: fmt.Sprintf("arn:aws:sqs:us-east-1:123456789012:q%05d", i), Tags: tags})
	}
	return out
}

// NFR1.12: reading a rule file with 100 canonical keys, 300 aliases, 500
// value spellings and 20 required tags of 10 conditions each, and asking the
// three queries for 10,000 resources (every required tag, and every tag's
// canonical value and removal).
func TestPerformanceU3_NFR1_12_QueriesAreCheap(t *testing.T) {
	content := queryLoadRules()
	resources := queryLoadResources(10_000 / perfScale)

	start := time.Now()
	rs, err := Parse("load.yaml", []byte(content), Options{})
	if err != nil {
		t.Fatal(err)
	}
	resolved, changed, removed := 0, 0, 0
	for _, res := range resources {
		for _, req := range rs.RequiredTags {
			if rs.RequiredValue(req.Key, res).Status == StatusResolved {
				resolved++
			}
		}
		for k, v := range res.Tags {
			canonical, _ := rs.CanonicalKeyOf(k)
			if rs.CanonicalValue(canonical, v).Changed {
				changed++
			}
			if rs.IsRemoved(k).Remove {
				removed++
			}
		}
	}
	elapsed := time.Since(start)

	limit := 2 * time.Second * timeLimitFactor
	t.Logf("NFR1.12: %d resources: %v (limit %v); %d resolved, %d changed, %d removed", len(resources), elapsed, limit, resolved, changed, removed)
	if elapsed > limit {
		t.Fatalf("the queries took %v, limit %v", elapsed, limit)
	}
	if len(rs.KeyRules) != 100 || len(rs.RequiredTags) != 20 || resolved == 0 || changed == 0 || removed == 0 {
		t.Fatalf("the load did not exercise the queries: %d keys, %d required, %d/%d/%d", len(rs.KeyRules), len(rs.RequiredTags), resolved, changed, removed)
	}
}

func queryLoadRules() string {
	var b strings.Builder
	b.WriteString("version: 1\nkeys:\n")
	for k := 0; k < 100; k++ {
		fmt.Fprintf(&b, "  Key%03d:\n    aliases: [key%03d, key_%03d, KEY%03d]\n    values:\n", k, k, k, k)
		for v := 0; v < 5; v++ {
			fmt.Fprintf(&b, "      value%d: [Value%d]\n", v, v)
		}
	}
	b.WriteString("required:\n")
	for r := 0; r < 20; r++ {
		fmt.Fprintf(&b, "  Required%02d:\n    default: d%02d\n    when:\n", r, r)
		for c := 0; c < 10; c++ {
			switch c % 3 {
			case 0:
				fmt.Fprintf(&b, "      - match: {resourceType: \"svc%d:thing\"}\n", c)
			case 1:
				fmt.Fprintf(&b, "      - match: {region: us-east-%d}\n", c)
			default:
				fmt.Fprintf(&b, "      - match: {tags: {Key%03d: value%d}}\n", c, c%5)
			}
			fmt.Fprintf(&b, "        value: v%02d-%d\n", r, c)
		}
	}
	b.WriteString("remove: [tmp-a, tmp-b, tmp-c]\n")
	return b.String()
}

func queryLoadResources(n int) []Resource {
	out := make([]Resource, 0, n)
	for i := 0; i < n; i++ {
		tags := map[string]string{"tmp-a": "x", "Name": fmt.Sprint("n", i)}
		for j := 0; j < 8; j++ {
			k := (i + j*13) % 100
			key := fmt.Sprintf("key%03d", k)
			if j%2 == 0 {
				key = fmt.Sprintf("Key%03d", k)
			}
			tags[key] = fmt.Sprintf("Value%d", (i+j)%5)
		}
		out = append(out, Resource{ResourceType: fmt.Sprintf("svc%d:thing", i%10), Region: fmt.Sprintf("us-east-%d", i%10), Tags: tags})
	}
	return out
}

// Keep inventory.CanonicalKey in step with the load above: the three
// spellings of a key must make one group.
func TestThePerformanceLoadGroupsAsIntended(t *testing.T) {
	a, b, c := inventory.CanonicalKey("app-001"), inventory.CanonicalKey("App_001"), inventory.CanonicalKey("app 001")
	if a != b || b != c {
		t.Fatalf("%q %q %q", a, b, c)
	}
}
