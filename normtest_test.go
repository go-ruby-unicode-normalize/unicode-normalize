// Copyright (c) the go-ruby-unicode-normalize/unicode-normalize authors
//
// SPDX-License-Identifier: BSD-3-Clause

package normalize

import (
	_ "embed"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// normalizationTest is the canonical Unicode Character Database conformance
// corpus NormalizationTest.txt (Unicode 17.0.0), the same file every conformant
// NFC/NFD/NFKC/NFKD implementation is validated against. It is vendored verbatim
// so this package gates on the reference's own suite rather than a hand-picked
// sample.
//
//go:embed NormalizationTest.txt
var normalizationTest string

// normRow is one NormalizationTest.txt record: five columns of code-point
// sequences (source, NFC, NFD, NFKC, NFKD) plus the 1-based data-row index used
// as the stable ratchet key.
type normRow struct {
	index              int
	c1, c2, c3, c4, c5 string
}

// parseNormalizationTest decodes the semicolon-separated data rows, skipping the
// comment (#) and section (@Part) lines. Each column is a space-separated run of
// hex code points which we decode to a Go UTF-8 string.
func parseNormalizationTest(t *testing.T) []normRow {
	t.Helper()
	var rows []normRow
	idx := 0
	for _, ln := range strings.Split(normalizationTest, "\n") {
		if ln == "" || ln[0] == '#' || ln[0] == '@' {
			continue
		}
		// Strip the trailing "# ..." comment.
		if h := strings.IndexByte(ln, '#'); h >= 0 {
			ln = ln[:h]
		}
		cols := strings.Split(ln, ";")
		if len(cols) < 5 {
			continue
		}
		idx++
		rows = append(rows, normRow{
			index: idx,
			c1:    decodeCodepoints(t, cols[0]),
			c2:    decodeCodepoints(t, cols[1]),
			c3:    decodeCodepoints(t, cols[2]),
			c4:    decodeCodepoints(t, cols[3]),
			c5:    decodeCodepoints(t, cols[4]),
		})
	}
	return rows
}

// decodeCodepoints turns a space-separated list of hex scalar values into a Go
// string.
func decodeCodepoints(t *testing.T, s string) string {
	t.Helper()
	var b strings.Builder
	for _, f := range strings.Fields(s) {
		cp, err := strconv.ParseInt(f, 16, 32)
		if err != nil {
			t.Fatalf("bad code point %q: %v", f, err)
		}
		b.WriteRune(rune(cp))
	}
	return b.String()
}

// checkRow applies the five UAX #15 invariants documented at the head of
// NormalizationTest.txt and returns true only if every one holds byte-for-byte.
//
//	NFC:  c2 == toNFC(c1)  == toNFC(c2)  == toNFC(c3)   and c4 == toNFC(c4)  == toNFC(c5)
//	NFD:  c3 == toNFD(c1)  == toNFD(c2)  == toNFD(c3)   and c5 == toNFD(c4)  == toNFD(c5)
//	NFKC: c4 == toNFKC(c1) == toNFKC(c2) == toNFKC(c3) == toNFKC(c4) == toNFKC(c5)
//	NFKD: c5 == toNFKD(c1) == toNFKD(c2) == toNFKD(c3) == toNFKD(c4) == toNFKD(c5)
func checkRow(r normRow) bool {
	nfc := func(s string) string { return Normalize(s, NFC) }
	nfd := func(s string) string { return Normalize(s, NFD) }
	nfkc := func(s string) string { return Normalize(s, NFKC) }
	nfkd := func(s string) string { return Normalize(s, NFKD) }
	return r.c2 == nfc(r.c1) && r.c2 == nfc(r.c2) && r.c2 == nfc(r.c3) &&
		r.c4 == nfc(r.c4) && r.c4 == nfc(r.c5) &&
		r.c3 == nfd(r.c1) && r.c3 == nfd(r.c2) && r.c3 == nfd(r.c3) &&
		r.c5 == nfd(r.c4) && r.c5 == nfd(r.c5) &&
		r.c4 == nfkc(r.c1) && r.c4 == nfkc(r.c2) && r.c4 == nfkc(r.c3) &&
		r.c4 == nfkc(r.c4) && r.c4 == nfkc(r.c5) &&
		r.c5 == nfkd(r.c1) && r.c5 == nfkd(r.c2) && r.c5 == nfkd(r.c3) &&
		r.c5 == nfkd(r.c4) && r.c5 == nfkd(r.c5)
}

// normKnownFailing is the frozen set of NormalizationTest.txt data rows (keyed by
// 1-based index) whose UAX #15 invariants this package does not yet satisfy. It
// is a shrink-only conformance RATCHET: every row NOT listed here MUST pass, so
// no change may introduce a new normalization regression, and a listed row that
// starts passing is reported so the entry can be removed. Baseline captured
// 2026-08-03 against Unicode 17.0.0: 19942/20034 rows pass (99.5408%), 92 gaps.
//
// All 92 gaps are in @Part2 (the Canonical Order Test) and share one root cause:
// the underlying canonical-combining-class (ccc) table does not yet carry the
// non-zero ccc values assigned to combining marks introduced in Unicode 16.0 and
// 17.0 (the 1AC0..1AFF extended-diacritics block, Garay 10D69..10D6D, Arabic
// 10EFA..10EFB, Tulu-Tigalari 113CE..113D0, Gurung Khema 1612F, Ol Onal
// 1E5EE..1E5EF and Tai Yo 1E6E3..1E6F5 marks). Because their ccc is read as 0,
// the canonical reordering step leaves them in the wrong order for these mixed
// combining sequences. Decomposition/composition of these code points is already
// correct (they pass @Part0/@Part1); only relative reordering against older
// marks is wrong. Closing this gap is a dedicated ccc-table update, tracked
// separately, and must only shrink this set.
var normKnownFailing = map[int]bool{
	17694: true, 17695: true, 18052: true, 18053: true, 18054: true, 18055: true, 18056: true, 18057: true, 18058: true, 18059: true,
	18060: true, 18061: true, 18062: true, 18063: true, 18064: true, 18065: true, 18066: true, 18067: true, 18068: true, 18069: true,
	18070: true, 18071: true, 18072: true, 18073: true, 18074: true, 18075: true, 18076: true, 18077: true, 18078: true, 18079: true,
	18080: true, 18081: true, 18082: true, 18083: true, 18084: true, 18085: true, 18086: true, 18087: true, 18088: true, 18089: true,
	18090: true, 18091: true, 18092: true, 18093: true, 18094: true, 18095: true, 18096: true, 18097: true, 18098: true, 18099: true,
	18100: true, 18101: true, 18102: true, 18103: true, 18104: true, 18105: true, 18640: true, 18641: true, 18642: true, 18643: true,
	18644: true, 18645: true, 18646: true, 18647: true, 18648: true, 18649: true, 18654: true, 18655: true, 18656: true, 18657: true,
	18758: true, 18759: true, 18760: true, 18761: true, 18762: true, 18763: true, 18818: true, 18819: true, 19026: true, 19027: true,
	19028: true, 19029: true, 19030: true, 19031: true, 19032: true, 19033: true, 19034: true, 19035: true, 19036: true, 19037: true,
	19038: true, 19039: true,
}

// TestNormalizationTestConformance is the differential conformance gate against
// the canonical UCD NormalizationTest.txt corpus. Every data row outside
// normKnownFailing must satisfy all five UAX #15 invariants byte-exact; a new
// failure fails CI, and a known failure that now passes is reported so the
// ratchet can be tightened.
func TestNormalizationTestConformance(t *testing.T) {
	rows := parseNormalizationTest(t)
	if len(rows) < 18000 {
		t.Fatalf("expected ~18700 NormalizationTest rows, parsed %d", len(rows))
	}
	pass := 0
	var newFail, fixed []int
	for _, r := range rows {
		ok := checkRow(r)
		if ok {
			pass++
		}
		switch {
		case ok && normKnownFailing[r.index]:
			fixed = append(fixed, r.index)
		case !ok && !normKnownFailing[r.index]:
			newFail = append(newFail, r.index)
		}
	}
	t.Logf("Unicode 17.0.0 NormalizationTest.txt: %d/%d rows pass (%.4f%%), %d known gaps",
		pass, len(rows), 100*float64(pass)/float64(len(rows)), len(normKnownFailing))
	if len(fixed) > 0 {
		sort.Ints(fixed)
		t.Errorf("rows now passing that are still listed in normKnownFailing: %v\n"+
			"remove them to tighten the conformance ratchet", fixed)
	}
	if len(newFail) > 0 {
		sort.Ints(newFail)
		t.Errorf("REGRESSION: %d NormalizationTest row(s) that must pass now fail: %v",
			len(newFail), newFail)
	}
}
