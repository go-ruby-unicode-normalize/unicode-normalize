// Copyright (c) the go-ruby-unicode-normalize/unicode-normalize authors
//
// SPDX-License-Identifier: BSD-3-Clause

package normalize

import "testing"

// These deterministic, ruby-free tests pin the MRI-observed results (captured
// from ruby 4.0.5, Unicode 17.0.0) so the suite drives the 100% coverage gate on
// every platform, including the Windows and cross-arch CI lanes where the MRI
// oracle skips itself.

// hexForm decodes a string of two-hex-digit bytes; it keeps the test tables
// readable for the byte sequences that the source-file encoding would otherwise
// mangle (combining marks, astral-plane characters).
func hexBytes(t *testing.T, h string) string {
	t.Helper()
	if len(h)%2 != 0 {
		t.Fatalf("odd hex length %q", h)
	}
	b := make([]byte, len(h)/2)
	for i := range b {
		var v int
		for j := 0; j < 2; j++ {
			c := h[i*2+j]
			switch {
			case c >= '0' && c <= '9':
				v = v<<4 | int(c-'0')
			case c >= 'a' && c <= 'f':
				v = v<<4 | int(c-'a'+10)
			default:
				t.Fatalf("bad hex digit %q", string(c))
			}
		}
		b[i] = byte(v)
	}
	return string(b)
}

func toHex(s string) string {
	const hexdig = "0123456789abcdef"
	out := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		out = append(out, hexdig[s[i]>>4], hexdig[s[i]&0xf])
	}
	return string(out)
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string // hex bytes
		form Form
		want string // hex bytes
	}{
		// --- core canonical round-trips (the README "é" examples) ---
		{"nfc composed e-acute", "c3a9", NFC, "c3a9"},
		{"nfc decomposed e-acute", "65cc81", NFC, "c3a9"},
		{"nfd composed e-acute", "c3a9", NFD, "65cc81"},
		{"nfd decomposed e-acute", "65cc81", NFD, "65cc81"},

		// --- Hangul composition / decomposition ---
		{"nfc hangul jamo", "e18480e185a1", NFC, "eab080"}, // ga
		{"nfd hangul syllable", "eab080", NFD, "e18480e185a1"},
		{"nfc hangul lvt", "e18480e185a1e186a8", NFC, "eab081"}, // gag (L+V+T)

		// --- compatibility folding ---
		{"nfkc ligature fi", "efac81", NFKC, "6669"},    // fi -> fi
		{"nfkd ligature fi", "efac81", NFKD, "6669"},    // fi -> fi
		{"nfkc circled 1", "e291a0", NFKC, "31"},        // (1) -> 1
		{"nfkc fullwidth A", "efbca1", NFKC, "41"},      // FW A -> A
		{"nfc keeps ligature", "efac81", NFC, "efac81"}, // NFC does not fold
		{"nfd keeps ligature", "efac81", NFD, "efac81"}, // NFD does not fold
		{"nfkc roman numeral", "e285b0", NFKC, "69"},    // small roman i -> i
		{"nfkc angstrom", "e284ab", NFKC, "c385"},       // ANGSTROM -> A-ring

		// --- combining-mark canonical ordering ---
		// U+0323 (ccc 220) then U+0301 (ccc 230) reorder under NFD.
		{"nfd reorder marks", "0061cc81cca3", NFD, "0061cca3cc81"},

		// --- ascii / empty / passthrough ---
		{"empty nfc", "", NFC, ""},
		{"ascii nfkd", "68656c6c6f", NFKD, "68656c6c6f"},

		// --- Unicode 16/17 canonical-group decomposition (NFD/NFKD) ---
		// U+105C9 -> U+105D2 U+0307
		{"nfd new canonical", "f0909789", NFD, "f0909792cc87"},
		{"nfkd new canonical", "f0909789", NFKD, "f0909792cc87"},
		// U+16121 -> U+1611E U+1611E
		{"nfd new canonical pair", "f09684a1", NFD, "f096849ef096849e"},
		// U+16128 -> U+1611E U+1611E U+16120 (multi-element decomposition)
		{"nfd new canonical triple", "f09684a8", NFD, "f096849ef096849ef09684a0"},

		// --- Unicode 16/17 canonical-group composition (NFC/NFKC) ---
		// U+1611E U+1611E -> U+16121
		{"nfc new composition", "f096849ef096849e", NFC, "f09684a1"},
		// U+1611E U+1611E U+1611F -> U+16121 U+1611F -> U+16126 (stepwise)
		{"nfc new composition stepwise", "f096849ef096849ef096849f", NFC, "f09684a6"},
		{"nfkc new composition", "f096849ef096849e", NFKC, "f09684a1"},
		// the composed character is already minimal under NFC
		{"nfc new composed stays", "f09684a1", NFC, "f09684a1"},
		// U+105D2 U+0307 -> U+105C9 (composition with a real combining mark)
		{"nfc new composition with mark", "f0909792cc87", NFC, "f0909789"},

		// --- Unicode 16/17 compatibility-group folding (NFKC/NFKD) ---
		{"nfkc outlined A", "f09cb396", NFKC, "41"},  // U+1CCD6 -> A
		{"nfkd outlined A", "f09cb396", NFKD, "41"},  // U+1CCD6 -> A
		{"nfkc outlined 0", "f09cb3b0", NFKC, "30"},  // U+1CCF0 -> 0
		{"nfkc small s cross", "ea9fb1", NFKC, "53"}, // U+A7F1 -> S
		{"nfkd small s cross", "ea9fb1", NFKD, "53"},
		// canonical-group character keeps composing even under NFKC
		{"nfkc canonical kept composed", "f09684a1", NFKC, "f09684a1"},
		// compatibility-group character is untouched by the canonical forms
		{"nfc compat untouched", "f09cb396", NFC, "f09cb396"},
		{"nfd compat untouched", "f09cb396", NFD, "f09cb396"},
		// override character mixed with an ordinary one (exercises the plain
		// passthrough inside the expansion pass).
		{"nfkd ascii then outlined", "42f09cb396", NFKD, "4241"}, // B + outlined-A -> BA
		{"nfd ascii then new canonical", "42f09684a1", NFD, "42f096849ef096849e"},

		// --- composition blocking: a mark of class >= the candidate blocks it ---
		// U+1611E U+0301(ccc230) U+1611E : the second base is blocked from
		// composing onto the first by the intervening combining acute.
		{"nfc composition blocked", "f096849ecc81f096849e", NFC, "f096849ecc81f096849e"},

		// --- out-of-range Form falls through to NFC ---
		{"oob form acts as nfc", "65cc81", Form(99), "c3a9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Normalize(hexBytes(t, tc.in), tc.form)
			if want := hexBytes(t, tc.want); got != want {
				t.Fatalf("Normalize(%s, %v) = %s, want %s",
					tc.in, tc.form, toHex(got), tc.want)
			}
		})
	}
}

func TestIsNormalized(t *testing.T) {
	cases := []struct {
		name string
		in   string
		form Form
		want bool
	}{
		{"composed is nfc", "c3a9", NFC, true},
		{"decomposed is not nfc", "65cc81", NFC, false},
		{"decomposed is nfd", "65cc81", NFD, true},
		{"composed is not nfd", "c3a9", NFD, false},
		{"ligature is not nfkc", "efac81", NFKC, false},
		{"folded is nfkc", "6669", NFKC, true},
		{"ascii normalized all forms", "68656c6c6f", NFC, true},
		{"empty is normalized", "", NFD, true},
		// Unicode 16/17 override characters.
		{"new composed is nfc", "f09684a1", NFC, true},
		{"new decomposed is not nfc", "f096849ef096849e", NFC, false},
		{"new decomposed is nfd", "f096849ef096849e", NFD, true},
		{"new composed is not nfd", "f09684a1", NFD, false},
		{"outlined A is not nfkc", "f09cb396", NFKC, false},
		{"outlined A is nfc", "f09cb396", NFC, true},
		{"oob form treated as nfc", "65cc81", Form(99), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNormalized(hexBytes(t, tc.in), tc.form); got != tc.want {
				t.Fatalf("IsNormalized(%s, %v) = %v, want %v",
					tc.in, tc.form, got, tc.want)
			}
		})
	}
}

func TestFormString(t *testing.T) {
	cases := map[Form]string{
		NFC:      ":nfc",
		NFD:      ":nfd",
		NFKC:     ":nfkc",
		NFKD:     ":nfkd",
		Form(99): ":nfc",
	}
	for f, want := range cases {
		if got := f.String(); got != want {
			t.Fatalf("Form(%d).String() = %q, want %q", int(f), got, want)
		}
	}
}
