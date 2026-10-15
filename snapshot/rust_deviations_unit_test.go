package snapshot_test

import (
	"errors"
	"strings"
	"testing"
)

func TestJudgeReference(t *testing.T) {
	const fp = "1111111111111111111111111111111111111111111111111111111111111111"
	const otherFP = "2222222222222222222222222222222222222222222222222222222222222222"
	bug := &rustDeviation{Kind: deviationBug, Fingerprint: fp}
	intentional := &rustDeviation{Kind: deviationIntentional, Fingerprint: fp}
	unsupported := &rustDeviation{Kind: deviationUnsupported, ErrorContains: "not implemented"}
	compileErr := errors.New("spirv: per-entry-point output not implemented")

	tests := []struct {
		name        string
		dev         *rustDeviation
		err         error
		differs     bool
		fingerprint string
		want        referenceResult
		wantKind    deviationKind
		wantMessage string
	}{
		{"match", nil, nil, false, "", resultPass, 0, ""},
		{"undeclared difference", nil, nil, true, fp, resultFail, 0, fp},
		{"undeclared compile error", nil, compileErr, false, "", resultFail, 0, "not implemented"},
		{"declared unsupported", unsupported, compileErr, false, "", resultDeviation, deviationUnsupported, ""},
		{"unsupported with another error", unsupported, errors.New("boom"), false, "", resultFail, 0, "boom"},
		{"unsupported now compiles", unsupported, nil, false, "", resultFail, 0, "remove"},
		{"declared intentional", intentional, nil, true, fp, resultDeviation, deviationIntentional, ""},
		{"declared bug", bug, nil, true, fp, resultDeviation, deviationBug, ""},
		{"declared difference changed", bug, nil, true, otherFP, resultFail, 0, otherFP},
		{"declared difference fixed", bug, nil, false, "", resultFail, 0, "remove"},
		{"declared difference, compile error", bug, compileErr, false, "", resultFail, 0, "not implemented"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := judgeReference(tt.dev, tt.err, tt.differs, tt.fingerprint)
			if got.result != tt.want || got.kind != tt.wantKind {
				t.Fatalf("judgeReference() = %+v, want result %v kind %v", got, tt.want, tt.wantKind)
			}
			if !strings.Contains(got.message, tt.wantMessage) {
				t.Errorf("message %q does not mention %q", got.message, tt.wantMessage)
			}
		})
	}
}

func TestReferenceFingerprint(t *testing.T) {
	a := referenceFingerprint("ref\n", "ours\n")
	if len(a) != 64 {
		t.Fatalf("fingerprint %q is not a SHA-256 hex digest", a)
	}
	if a != referenceFingerprint("ref\r\n", "ours\r\n") {
		t.Error("line endings must not change the fingerprint")
	}
	if a == referenceFingerprint("ours\n", "ref\n") {
		t.Error("swapping reference and output must change the fingerprint")
	}
	if a == referenceFingerprint("ref", "\nours\n") {
		t.Error("moving text across the separator must change the fingerprint")
	}
}

// TestRustDeviationsWellFormed validates the deviation table itself: each
// entry names a compared output — an expected output of a vendored shader, or
// a Rust-derived reference of one of our own shaders — carries the evidence
// its kind needs, and appears once.
func TestRustDeviationsWellFormed(t *testing.T) {
	outputs := map[string]bool{} // "shader/file" of every compared output
	for _, shader := range loadCorpus(t) {
		params := rustParams{targets: rustTargetsWGSLDefault}
		if shader.origin == originRust {
			var err error
			if params, err = loadRustParams(shader.name); err != nil {
				t.Fatal(err)
			}
		}
		module, err := tryCompileToIR(shader.source)
		if err != nil {
			continue // reported by TestRustReference
		}
		for _, o := range expectedRustOutputs(shader.name, params, module.EntryPoints) {
			if _, ok := rustReferencePath(o.kind, o.file); ok || shader.origin == originRust {
				outputs[shader.name+"/"+o.file] = true
			}
		}
	}

	seen := map[string]bool{}
	for _, d := range rustDeviations {
		key := d.Shader + "/" + d.File
		if seen[key] {
			t.Errorf("%s: duplicate entry", key)
		}
		seen[key] = true
		if !outputs[key] {
			t.Errorf("%s: not an output Rust writes for this shader (stale entry or non-target)", key)
		}
		if strings.TrimSpace(d.Reason) == "" {
			t.Errorf("%s: empty Reason", key)
		}
		switch d.Kind {
		case deviationIntentional, deviationBug:
			if !isSHA256Hex(d.Fingerprint) || d.ErrorContains != "" {
				t.Errorf("%s: %v needs a Fingerprint and no ErrorContains", key, d.Kind)
			}
		case deviationUnsupported:
			if d.ErrorContains == "" || d.Fingerprint != "" {
				t.Errorf("%s: unsupported needs ErrorContains and no Fingerprint", key)
			}
		default:
			t.Errorf("%s: unknown kind %v", key, d.Kind)
		}
		if d.Kind != deviationIntentional && d.Card == "" {
			t.Errorf("%s: %v needs a tracking Card", key, d.Kind)
		}
	}
}
