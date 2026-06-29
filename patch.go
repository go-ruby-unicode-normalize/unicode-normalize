// Copyright (c) the go-ruby-unicode-normalize/unicode-normalize authors
//
// SPDX-License-Identifier: BSD-3-Clause

package normalize

import "golang.org/x/text/unicode/norm"

// patchUnicode17 normalizes s to form while bridging the gap between MRI 4.0.5
// (Unicode 17.0.0) and the Unicode 15.0.0 fallback tables that
// golang.org/x/text uses before go1.27. The strategy is:
//
//  1. expand, in the raw input, the new decompositions x/text lacks (canonical
//     for every form, plus compatibility for the K-forms);
//  2. run x/text's normalizer for the form, which now sees only Unicode<=15
//     characters and so canonically orders and (for the C-forms) composes them
//     exactly as MRI does;
//  3. for the C-forms, apply the new canonical compositions x/text cannot.
//
// On go1.27+ x/text already knows Unicode 17, the override runes never trigger a
// rewrite, and steps 1 and 3 are no-ops, so the result is identical.
//
// The patched characters split into a canonical group (new canonical
// decompositions in some Indic and historic scripts, and their inverse
// compositions) and a compatibility group (U+A7F1 and the outlined alphanumerics
// U+1CCD6..U+1CCF9, which fold to ASCII).
func patchUnicode17(s string, form Form) string {
	switch form {
	case NFD:
		return norm.NFD.String(expand(s, true, false))
	case NFKD:
		return norm.NFKD.String(expand(s, true, true))
	case NFKC:
		return compose(norm.NFKC.String(expand(s, true, true)))
	default: // NFC
		return compose(norm.NFC.String(expand(s, true, false)))
	}
}

// expand rewrites the override decompositions present in s. canon enables the
// canonical group, compat the compatibility group. The result is fed to x/text,
// which re-imposes canonical ordering, so expand need not order anything itself.
// When s holds none of the active override runes it is returned unchanged.
func expand(s string, canon, compat bool) string {
	if !containsDecomp(s, canon, compat) {
		return s
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if canon {
			if d, ok := canonicalDecomp[r]; ok {
				out = append(out, d...)
				continue
			}
		}
		if compat {
			if d, ok := compatDecomp[r]; ok {
				out = append(out, d...)
				continue
			}
		}
		out = append(out, r)
	}
	return string(out)
}

// containsDecomp reports whether s holds any rune the active decomposition tables
// rewrite, so the common path skips reallocating.
func containsDecomp(s string, canon, compat bool) bool {
	for _, r := range s {
		if canon {
			if _, ok := canonicalDecomp[r]; ok {
				return true
			}
		}
		if compat {
			if _, ok := compatDecomp[r]; ok {
				return true
			}
		}
	}
	return false
}

// compose applies the Unicode 16.0/17.0 canonical compositions that x/text
// (<=Unicode 15) does not know, on top of an already NFC/NFKC-normalised string.
// It follows the UAX #15 canonical-composition rule: a character is blocked from
// composing with the preceding starter when an intervening character has a
// combining class greater than or equal to it (a starter always blocks). The
// composite is itself a starter, so a run may compose several characters in
// sequence (e.g. U+1611E U+1611E U+1611F -> U+16121 U+1611F -> U+16126).
func compose(s string) string {
	if !composable(s) {
		return s
	}
	rs := []rune(s)
	out := make([]rune, 0, len(rs))
	// starterIdx is the index in out of the most recent starter eligible for
	// composition, or -1. lastCCC is the combining class of the last character
	// appended after that starter (0 while none follows it yet).
	starterIdx := -1
	lastCCC := uint8(0)
	for _, r := range rs {
		cc := ccc(r)
		if starterIdx >= 0 && (lastCCC == 0 || lastCCC < cc) {
			if c, ok := newComposition[[2]rune{out[starterIdx], r}]; ok {
				out[starterIdx] = c
				lastCCC = 0
				continue
			}
		}
		out = append(out, r)
		if cc == 0 {
			starterIdx = len(out) - 1
			lastCCC = 0
		} else {
			lastCCC = cc
		}
	}
	return string(out)
}

// composable reports whether s contains a rune that can start a new composition
// pair, used as a cheap pre-filter before the composition scan.
func composable(s string) bool {
	for _, r := range s {
		if compositionOperand[r] {
			return true
		}
	}
	return false
}

// ccc returns the canonical combining class of r via x/text's Unicode 15.0.0
// properties. Every new composition operand is a pre-Unicode-16 character, so its
// class is known regardless of toolchain.
func ccc(r rune) uint8 {
	return norm.NFC.PropertiesString(string(r)).CCC()
}
