package mongodb

import "testing"

func TestParseVersionUsesMajorMinorPatch(t *testing.T) {
	version, err := ParseVersion("3.6.23")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if version.Major != 3 || version.Minor != 6 || version.Patch != 23 {
		t.Fatalf("unexpected version: %+v", version)
	}
	if version.String() != "3.6.23" {
		t.Fatalf("unexpected string: %q", version.String())
	}
}

func TestParseVersionRejectsInvalidVersion(t *testing.T) {
	_, err := ParseVersion("not-a-version")

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVersionAtLeastComparesMajorMinorPatch(t *testing.T) {
	version := Version{Major: 3, Minor: 6, Patch: 23}

	if !version.AtLeast(3, 6, 0) {
		t.Fatal("expected 3.6.23 to be at least 3.6.0")
	}
	if version.AtLeast(4, 0, 0) {
		t.Fatal("expected 3.6.23 to be lower than 4.0.0")
	}
}
