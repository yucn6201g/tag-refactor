package iac

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// How new keys and values are written (BR4.4, security design §5). The kind of
// quote of the text they replace stays, unless the new string cannot be written
// with it.

var (
	// yamlPlainChars is what a plain scalar may hold here: no ":" and no space,
	// so base 60 numbers and dates with a time are out already.
	yamlPlainChars = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./@-]*$`)
	yamlNumber     = regexp.MustCompile(`^(?:0[xX][0-9a-fA-F_]+|0[oO][0-7_]+|0[bB][01_]+|[0-9][0-9_]*(?:\.[0-9_]*)?(?:[eE][-+]?[0-9_]+)?)$`)
	yamlDate       = regexp.MustCompile(`^[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}$`)
	hclName        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
)

// yamlPlainOK reports whether s can be written without quotes and read as a
// string by both YAML 1.1 and 1.2.
func yamlPlainOK(s string) bool {
	if !yamlPlainChars.MatchString(s) || yamlNumber.MatchString(s) || yamlDate.MatchString(s) {
		return false
	}
	switch strings.ToLower(s) {
	case "y", "n", "yes", "no", "on", "off", "true", "false":
		return false
	}
	switch s {
	case "null", "Null", "NULL":
		return false
	}
	return true
}

// yamlControl reports a character that cannot stand in a single-quoted scalar:
// control characters and the line breaks of YAML.
func yamlControl(r rune) bool {
	return r < 0x20 || r == 0x7f || r == 0x85 || r == 0x2028 || r == 0x2029 || r == 0xfeff
}

// yamlSingle writes s between single quotes, or double ones when it holds a
// character single quotes cannot show.
func yamlSingle(s string) string {
	if strings.ContainsFunc(s, yamlControl) {
		return yamlDouble(s)
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// yamlDouble writes s between double quotes with the escapes of YAML.
func yamlDouble(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r <= 0xff && yamlControl(r):
			fmt.Fprintf(&b, `\x%02X`, r)
		case yamlControl(r):
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// jsonText writes s as a JSON string, without the HTML escapes.
func jsonText(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		// A string always encodes.
		return `""`
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// hclText writes s as a quoted string of HCL, with ${ and %{ doubled so that
// they do not start an interpolation.
func hclText(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		case (r == '$' || r == '%') && i+1 < len(s) && s[i+1] == '{':
			b.WriteRune(r)
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
		i += size
	}
	b.WriteByte('"')
	return b.String()
}

// hclKey writes a key of tags = { ... }: as a name when it was one and can stay
// one, otherwise as a string. The words the parser reads as something else
// cannot be names.
func hclKey(q quoteStyle, s string) string {
	if q == quotePlain && hclName.MatchString(s) {
		switch s {
		case "for", "null", "true", "false":
		default:
			return s
		}
	}
	return hclText(s)
}

// tsText writes s as a TypeScript string in the quote of the text it replaces
// (v0.2.0 BR3.1): the quote, the backslash and the line breaks are escaped,
// and in a template literal ${ too, so that it stays without substitutions.
func tsText(q quoteStyle, s string) string {
	quote := byte('\'')
	switch q {
	case quoteDouble:
		quote = '"'
	case quoteBacktick:
		quote = '`'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for i, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteByte(quote)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f || r == 0x2028 || r == 0x2029:
			fmt.Fprintf(&b, `\u%04X`, r)
		case quote == '`' && r == '$' && strings.HasPrefix(s[i:], "${"):
			b.WriteString(`\$`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// writeTextForTest, when set, replaces what writeText writes, so that a test can
// see a fault of this tool end at the check after the edits.
var writeTextForTest func(string) string

// writeText writes a new key or value in the quote of the text it replaces.
func writeText(format fileFormat, q quoteStyle, s string, isKey bool) string {
	if writeTextForTest != nil {
		return writeTextForTest(s)
	}
	switch format {
	case formatJSON:
		return jsonText(s)
	case formatTypeScript:
		return tsText(q, s)
	case formatTerraform:
		if isKey {
			return hclKey(q, s)
		}
		return hclText(s)
	}
	switch q {
	case quoteSingle:
		return yamlSingle(s)
	case quoteDouble:
		return yamlDouble(s)
	}
	if yamlPlainOK(s) {
		return s
	}
	return yamlDouble(s)
}
