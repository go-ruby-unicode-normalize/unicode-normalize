// Copyright (c) the go-ruby-unicode-normalize/unicode-normalize authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package normalize implements MRI-compatible Unicode normalization, matching
// Ruby's String#unicode_normalize / String#unicode_normalized? (the
// unicode_normalize standard library).
//
// The four normalization forms NFC, NFD, NFKC and NFKD are the four forms
// defined by Unicode Standard Annex #15. Ruby's String#unicode_normalize
// accepts the corresponding symbols :nfc, :nfd, :nfkc and :nfkd, with :nfc the
// default when no argument is given.
//
// The work is delegated to golang.org/x/text/unicode/norm, a pure-Go (CGO=0)
// implementation of the same Unicode standard. MRI 4.0.5 normalizes against
// Unicode 17.0.0. x/text ships Unicode 17.0.0 tables but selects them by the
// go1.27 build tag (Unicode 15.0.0 otherwise); this module requires go 1.27.1,
// so the Unicode 17.0.0 tables are the only ones it can be built with, and
// TestUnicodeVersion fails if that ever stops being true. The oracle test suite
// verifies the agreement differentially against MRI.
package normalize

import "golang.org/x/text/unicode/norm"

// Form selects a Unicode normalization form. The zero value is NFC, matching the
// default form of Ruby's String#unicode_normalize.
type Form int

const (
	// NFC is Normalization Form C (canonical decomposition followed by
	// canonical composition). It is the default form, matching :nfc.
	NFC Form = iota
	// NFD is Normalization Form D (canonical decomposition), matching :nfd.
	NFD
	// NFKC is Normalization Form KC (compatibility decomposition followed by
	// canonical composition), matching :nfkc.
	NFKC
	// NFKD is Normalization Form KD (compatibility decomposition), matching :nfkd.
	NFKD
)

// String reports the Ruby symbol name of the form (":nfc", ":nfd", ":nfkc" or
// ":nfkd"). It is used in diagnostics and by the rbgo binding layer.
func (f Form) String() string {
	switch f {
	case NFD:
		return ":nfd"
	case NFKC:
		return ":nfkc"
	case NFKD:
		return ":nfkd"
	default:
		return ":nfc"
	}
}

// Normalize returns s normalized to the given form, matching the result of
// Ruby's s.unicode_normalize(form). Bytes that are not valid UTF-8 are passed
// through unchanged, as x/text does; the Ruby binding layer is responsible for
// raising ArgumentError on invalid byte sequences to match MRI.
func Normalize(s string, form Form) string {
	switch form {
	case NFD:
		return norm.NFD.String(s)
	case NFKC:
		return norm.NFKC.String(s)
	case NFKD:
		return norm.NFKD.String(s)
	default: // NFC, and any out-of-range Form
		return norm.NFC.String(s)
	}
}

// IsNormalized reports whether s is already in the given normalization form,
// matching the result of Ruby's s.unicode_normalized?(form). A string is
// normalized for a form iff normalizing it leaves it unchanged.
func IsNormalized(s string, form Form) bool {
	// A string is in a normalization form exactly when normalizing it is a
	// no-op.
	return Normalize(s, form) == s
}
