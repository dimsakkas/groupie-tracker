package geocode

import (
	"groupie-tracker/internal/models"
	"os"
	"path/filepath"
	"testing"
)

func modelsResult(loc string) models.GeoResult {
	return models.GeoResult{Location: loc, Lat: 1.5, Lng: 2.5, State: "S", Resolved: true}
}

func TestLoadCache_MissingFileStartsEmpty(t *testing.T) {
	c := LoadCache(filepath.Join(t.TempDir(), "nope.json"))
	if _, ok := c.Get("anything"); ok {
		t.Fatal("fresh cache must be empty")
	}
}

func TestCache_SaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "geocache.json") // the directory does not exist yet
	c := LoadCache(path)
	c.Set("texas-usa", modelsResult("texas-usa"))
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error(".tmp file must not be left behind after a successful Save")
	}

	got, ok := LoadCache(path).Get("texas-usa")
	if !ok || got != modelsResult("texas-usa") {
		t.Fatalf("round trip failed: ok=%v got=%+v", ok, got)
	}
}

func TestLoadCache_CorruptedFileIsRemovedAndIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geocache.json")
	if err := os.WriteFile(path, []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}
	c := LoadCache(path)
	if _, ok := c.Get("x"); ok {
		t.Fatal("corrupted cache must load as empty")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("corrupted file should have been deleted")
	}
}
