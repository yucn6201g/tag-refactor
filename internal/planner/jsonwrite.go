package planner

import (
	"encoding/json"
	"io"
)

// JSONObject writes a JSON object one field at a time, so a large plan is
// never held as one value in memory (U4 NFR design Q2): an array is written
// item by item, each item marshaled on its own line. The layout is fixed, so
// the same input always gives the same bytes (NFR9.6). It writes ASCII
// syntax only; strings come from encoding/json, which escapes <, > and &.
// The first error stops every later write and is kept for Close.
type JSONObject struct {
	w      io.Writer
	indent string
	fields int
	err    error
}

// NewJSONObject starts an object on w. indent is the indent of its closing
// brace; fields get two more spaces.
func NewJSONObject(w io.Writer, indent string) *JSONObject {
	o := &JSONObject{w: w, indent: indent}
	o.write("{")
	return o
}

func (o *JSONObject) write(s string) {
	if o.err != nil {
		return
	}
	_, o.err = io.WriteString(o.w, s)
}

func (o *JSONObject) writeBytes(b []byte) {
	if o.err != nil {
		return
	}
	_, o.err = o.w.Write(b)
}

func (o *JSONObject) marshal(v any) {
	if o.err != nil {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		o.err = err
		return
	}
	o.writeBytes(b)
}

// name writes the separator and the name of the next field.
func (o *JSONObject) name(name string) {
	if o.fields > 0 {
		o.write(",")
	}
	o.fields++
	o.write("\n" + o.indent + "  ")
	o.marshal(name)
	o.write(": ")
}

// Field writes one field with its value marshaled on one line.
func (o *JSONObject) Field(name string, v any) {
	o.name(name)
	o.marshal(v)
}

// Array writes an array field with n items, each marshaled on its own line.
func (o *JSONObject) Array(name string, n int, item func(i int) any) {
	o.name(name)
	if n == 0 {
		o.write("[]")
		return
	}
	o.write("[")
	for i := 0; i < n && o.err == nil; i++ {
		if i > 0 {
			o.write(",")
		}
		o.write("\n" + o.indent + "    ")
		o.marshal(item(i))
	}
	o.write("\n" + o.indent + "  ]")
}

// Object writes an object field whose fields fill writes.
func (o *JSONObject) Object(name string, fill func(*JSONObject)) {
	o.name(name)
	if o.err != nil {
		return
	}
	inner := NewJSONObject(o.w, o.indent+"  ")
	fill(inner)
	o.err = inner.Close()
}

// Close writes the closing brace and returns the first error.
func (o *JSONObject) Close() error {
	if o.fields > 0 {
		o.write("\n" + o.indent)
	}
	o.write("}")
	return o.err
}
