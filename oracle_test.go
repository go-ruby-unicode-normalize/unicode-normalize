// Copyright (c) the go-ruby-unicode-normalize/unicode-normalize authors
//
// SPDX-License-Identifier: BSD-3-Clause

package normalize

import (
	"encoding/hex"
	"os/exec"
	"strings"
	"testing"
)

// The oracle tests check this package against MRI's String#unicode_normalize /
// String#unicode_normalized? differentially. They skip themselves when a usable
// ruby is absent (the Windows lane and the qemu cross-arch lanes), where the
// deterministic suite alone holds coverage at 100%. The gate also skips on Ruby
// older than 4.0 so the corpus is compared against the same Unicode 17.0.0 data
// this package targets.

// rubyBin locates a ruby new enough to match the targeted Unicode release.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	out, err := exec.Command(path, "-e", "print RUBY_VERSION").Output()
	if err != nil {
		t.Skipf("cannot determine ruby version: %v", err)
	}
	if v := string(out); v < "4.0" {
		t.Skipf("ruby %s < 4.0; skipping MRI oracle (Unicode version skew)", v)
	}
	return path
}

// formSym maps each Form to the Ruby symbol unicode_normalize expects.
var formSym = map[Form]string{
	NFC:  ":nfc",
	NFD:  ":nfd",
	NFKC: ":nfkc",
	NFKD: ":nfkd",
}

// rubyNormalize runs MRI over every "form<TAB>input-hex" line and returns a map
// from each line to "output-hex<TAB>normalized?", binmoding stdio so the Windows
// text layer (were it ever reached) cannot corrupt the UTF-8 bytes.
func rubyNormalize(t *testing.T, bin string, lines []string) map[string]string {
	t.Helper()
	const script = `
$stdout.binmode
$stdin.binmode
forms = {"nfc"=>:nfc, "nfd"=>:nfd, "nfkc"=>:nfkc, "nfkd"=>:nfkd}
STDIN.each_line do |line|
  form, inh = line.chomp.split("\t", 2)
  s = [inh].pack("H*").force_encoding("UTF-8")
  out = s.unicode_normalize(forms[form])
  norm = s.unicode_normalized?(forms[form])
  puts "#{form}\t#{inh}\t#{out.unpack1("H*")}\t#{norm}"
end
`
	cmd := exec.Command(bin, "-e", script)
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\noutput:\n%s", err, out)
	}
	res := make(map[string]string, len(lines))
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		f := strings.SplitN(l, "\t", 4)
		if len(f) != 4 {
			t.Fatalf("malformed ruby line: %q", l)
		}
		res[f[0]+"\t"+f[1]] = f[2] + "\t" + f[3]
	}
	return res
}

// goNormalize returns the same "output-hex<TAB>normalized?" string this package
// produces for a form and hex-encoded input.
func goNormalize(t *testing.T, formS, inh string) string {
	t.Helper()
	b, err := hex.DecodeString(inh)
	if err != nil {
		t.Fatalf("bad hex %q: %v", inh, err)
	}
	var form Form
	for f, sym := range formSym {
		if sym == ":"+formS {
			form = f
		}
	}
	s := string(b)
	got := hex.EncodeToString([]byte(Normalize(s, form)))
	normd := "false"
	if IsNormalized(s, form) {
		normd = "true"
	}
	return got + "\t" + normd
}

// checkCorpus drives a batch of inputs through both MRI and this package for all
// four forms and asserts byte-for-byte agreement of both Normalize and
// IsNormalized.
func checkCorpus(t *testing.T, bin string, inputs []string) {
	t.Helper()
	forms := []string{"nfc", "nfd", "nfkc", "nfkd"}
	var lines []string
	for _, inh := range inputs {
		for _, f := range forms {
			lines = append(lines, f+"\t"+inh)
		}
	}
	mri := rubyNormalize(t, bin, lines)
	for _, l := range lines {
		f := strings.SplitN(l, "\t", 2)
		got := goNormalize(t, f[0], f[1])
		if want := mri[l]; got != want {
			t.Errorf("form %s input %s: go=%q mri=%q", f[0], f[1], got, want)
		}
	}
}

// TestOracleCurated checks the documented edge cases against MRI: composed vs
// decomposed round-trips, Hangul, compatibility folding, and the Unicode 16/17
// override characters this package patches into x/text.
func TestOracleCurated(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{
		"",                         // empty
		"68656c6c6f",               // "hello"
		"c3a9",                     // composed é
		"65cc81",                   // decomposed e + acute
		"eab080",                   // Hangul 가 (precomposed)
		"e18480e185a1",             // Hangul L+V jamo
		"e18480e185a1e186a8",       // Hangul L+V+T jamo
		"efac81",                   // ligature ﬁ
		"e291a0",                   // circled 1
		"efbca1",                   // fullwidth A
		"e284ab",                   // ANGSTROM SIGN
		"0061cc81cca3",             // base + two marks needing canonical reorder
		"f09684a1",                 // U+16121 (new canonical, precomposed)
		"f096849ef096849e",         // U+1611E U+1611E (composes to U+16121)
		"f096849ef096849ef096849f", // stepwise composition
		"f0909789",                 // U+105C9 (new canonical via combining mark)
		"f0909792cc87",             // U+105D2 U+0307 (composes to U+105C9)
		"f09cb396",                 // U+1CCD6 (outlined A, NFKC -> A)
		"ea9fb1",                   // U+A7F1 (NFKC -> S)
		"f096849ecc81f096849e",     // composition blocked by intervening mark
		"42f09cb396",               // ordinary char + override char
	}
	checkCorpus(t, bin, inputs)
}

// TestOracleAllSingleCodepoints normalizes every assigned scalar value against
// MRI in all four forms. This is the differential equivalent of the official
// NormalizationTest single-character rows and is what proves the x/text+override
// composition matches MRI exactly (the surrogate range is skipped as it has no
// scalar value).
func TestOracleAllSingleCodepoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping exhaustive codepoint sweep in -short mode")
	}
	bin := rubyBin(t)
	var inputs []string
	for cp := rune(0); cp <= 0x10FFFF; cp++ {
		if cp >= 0xD800 && cp <= 0xDFFF {
			continue // surrogates are not scalar values
		}
		inputs = append(inputs, hex.EncodeToString([]byte(string(cp))))
	}
	// Batch so a single ruby invocation does not buffer megabytes at once.
	const batch = 4096
	for i := 0; i < len(inputs); i += batch {
		end := i + batch
		if end > len(inputs) {
			end = len(inputs)
		}
		checkCorpus(t, bin, inputs[i:end])
	}
}

// TestOracleCombiningSequences fuzzes short sequences drawn from a pool of bases,
// combining marks, Hangul jamo, compatibility characters and the Unicode 16/17
// override characters and their decomposition targets, exercising canonical
// ordering, composition blocking and the new compositions together.
func TestOracleCombiningSequences(t *testing.T) {
	bin := rubyBin(t)
	pool := []rune{
		0x0041, 0x0065, 0xAC00, 0x1100, 0x1161, 0x11A8, // bases + jamo
		0x0301, 0x0300, 0x0307, 0x0323, 0x031B, 0x0316, // combining marks (varied ccc)
		0x00E9, 0xFB01, 0x2460, 0xFF21, 0x212B, 0x1E0A, // composed / compatibility
		0x105C9, 0x105D2, 0x16121, 0x1611E, 0x1611F, 0x16120, // new-canonical group + targets
		0x16D68, 0x16D63, 0x16D67, 0x1CCD6, 0xA7F1, 0x113C2, // more group members
	}
	// Deterministic LCG so the corpus is identical on every run / platform.
	seed := uint64(0x9E3779B97F4A7C15)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int(seed>>33) % n
	}
	var inputs []string
	for i := 0; i < 6000; i++ {
		n := next(5) + 1
		var b []byte
		for j := 0; j < n; j++ {
			b = append(b, []byte(string(pool[next(len(pool))]))...)
		}
		inputs = append(inputs, hex.EncodeToString(b))
	}
	// De-duplicate to keep the ruby round-trip lean.
	seen := make(map[string]bool, len(inputs))
	uniq := inputs[:0]
	for _, in := range inputs {
		if !seen[in] {
			seen[in] = true
			uniq = append(uniq, in)
		}
	}
	checkCorpus(t, bin, uniq)
}
