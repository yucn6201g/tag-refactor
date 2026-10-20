// Package errkind defines the error kinds shared by every part of tag-refactor
// (contract C12). Parts return errors with a kind; only the cli package turns
// a kind into an exit code and a two-line message.
package errkind
