package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

type Verification struct {
	Lock         string   `json:"lock"`
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	OK           bool     `json:"ok"`
	Changed      []string `json:"changed,omitempty"`
	Missing      []string `json:"missing,omitempty"`
	Unverifiable bool     `json:"unverifiable,omitempty"`
}

func List(p paths.Paths) ([]model.ProvenanceLock, error) {
	entries, err := os.ReadDir(p.LocksRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []model.ProvenanceLock
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		lock, err := jsonx.Read(filepath.Join(p.LocksRoot, e.Name()), model.ProvenanceLock{})
		if err == nil && lock.ID != "" {
			out = append(out, lock)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func VerifyAll(p paths.Paths) ([]Verification, error) {
	entries, err := os.ReadDir(p.LocksRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Verification
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(p.LocksRoot, e.Name())
		lock, err := jsonx.Read(path, model.ProvenanceLock{})
		if err != nil {
			out = append(out, Verification{Lock: e.Name(), OK: false, Changed: []string{err.Error()}})
			continue
		}
		v := Verify(lock)
		v.Lock = e.Name()
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Lock < out[j].Lock })
	return out, nil
}

func Verify(lock model.ProvenanceLock) Verification {
	v := Verification{ID: lock.ID, Kind: lock.Kind, OK: true}
	if lock.Path == "" || len(lock.Files) == 0 {
		v.OK = false
		v.Unverifiable = true
		return v
	}
	for rel, want := range lock.Files {
		path := filepath.Join(lock.Path, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			v.OK = false
			v.Missing = append(v.Missing, rel)
			continue
		}
		s := sha256.Sum256(b)
		got := hex.EncodeToString(s[:])
		if !strings.EqualFold(got, want) {
			v.OK = false
			v.Changed = append(v.Changed, fmt.Sprintf("%s expected=%s got=%s", rel, want, got))
		}
	}
	sort.Strings(v.Changed)
	sort.Strings(v.Missing)
	return v
}
