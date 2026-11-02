package fetch

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// fetchLive is a variable so tests can replace the real network call.
var fetchLive = FetchAll

// FetchWithSnapshot loads the data from the live API and keeps a copy of it
// on disk. If the API cannot be reached (or answers with no artists), it
// falls back to the copy saved by an earlier successful run, so the server
// can still start. fromSnapshot tells the caller which one it got.
//
// Writing the copy is best effort: a failure is logged and never stops the
// server. Only when the API fails AND there is no usable copy do we return
// an error.
func FetchWithSnapshot(path string) (data RawData, fromSnapshot bool, err error) {
	live, liveErr := fetchLive()
	if liveErr == nil && len(live.Artists) == 0 {
		liveErr = errors.New("the API returned no artists")
	}
	if liveErr == nil {
		if err := saveSnapshot(path, live); err != nil {
			log.Printf("could not save the data snapshot (continuing): %v", err)
		}
		return live, false, nil
	}

	snap, snapErr := loadSnapshot(path)
	if snapErr != nil {
		return RawData{}, false, fmt.Errorf("live API failed (%v) and there is no usable snapshot (%v)", liveErr, snapErr)
	}
	log.Printf("live API failed (%v): using the saved snapshot %s", liveErr, path)
	return snap, true, nil
}

// saveSnapshot writes the data atomically (temporary file, then rename), so
// a crash in the middle can never leave a half-written snapshot behind.
func saveSnapshot(path string, data RawData) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, encoded, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// loadSnapshot reads a saved copy and rejects one that is unreadable,
// corrupted or has no artists.
func loadSnapshot(path string) (RawData, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return RawData{}, err
	}
	var data RawData
	if err := json.Unmarshal(encoded, &data); err != nil {
		return RawData{}, fmt.Errorf("snapshot is corrupted: %w", err)
	}
	if len(data.Artists) == 0 {
		return RawData{}, errors.New("snapshot has no artists")
	}
	return data, nil
}
