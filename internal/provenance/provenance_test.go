package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/homiakus/agctl/internal/model"
)

func TestVerifyDetectsChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	_ = os.WriteFile(path, []byte("a"), 0644)
	sum := sha256.Sum256([]byte("a"))
	lock := model.ProvenanceLock{ID: "x", Kind: "test", Path: root, Files: map[string]string{"a.txt": hex.EncodeToString(sum[:])}}
	v := Verify(lock)
	if !v.OK {
		t.Fatalf("expected ok %+v", v)
	}
	_ = os.WriteFile(path, []byte("b"), 0644)
	v = Verify(lock)
	if v.OK || len(v.Changed) == 0 {
		t.Fatalf("expected change %+v", v)
	}
}
