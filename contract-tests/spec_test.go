package contracttests

import "testing"

// TestSpecIsValid runs without network or key: it catches a spec that does
// not even parse, independently of the Redocly lint.
func TestSpecIsValid(t *testing.T) {
	s := loadSpec(t)
	if s.doc.Info.Version == "" {
		t.Fatal("info.version is empty")
	}
	t.Logf("openapi.yaml %s, %d paths", s.doc.Info.Version, s.doc.Paths.Len())
}
