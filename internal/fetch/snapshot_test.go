package fetch

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"groupie-tracker/internal/models"
)

func sampleData() RawData {
	return RawData{
		Artists: []models.RawArtist{{ID: 1, Name: "Queen", Members: []string{"Freddie Mercury", "Brian May"},
			CreationDate: 1970, FirstAlbum: "14-12-1973", Image: "http://x/queen.jpeg"}},
		Locations: []models.RawLocationResp{{ID: 1, Locations: []string{"london-uk"}}},
		Dates:     []models.RawDatesResp{{ID: 1, Dates: []string{"*23-08-2019"}}},
		Relations: []models.RawRelationsResp{{ID: 1, DatesLocations: map[string][]string{"london-uk": {"23-08-2019"}}}},
	}
}

// withLive replaces the network call for one test.
func withLive(t *testing.T, data RawData, err error) {
	t.Helper()
	old := fetchLive
	fetchLive = func() (RawData, error) { return data, err }
	t.Cleanup(func() { fetchLive = old })
}

func TestFetchWithSnapshot_LiveSuccessSavesACopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "upstream.json") // the folder does not exist yet
	withLive(t, sampleData(), nil)

	got, fromSnapshot, err := FetchWithSnapshot(path)
	if err != nil || fromSnapshot {
		t.Fatalf("err=%v fromSnapshot=%v, want nil false", err, fromSnapshot)
	}
	if !reflect.DeepEqual(got, sampleData()) {
		t.Errorf("returned data differs from the live data")
	}

	saved, err := loadSnapshot(path)
	if err != nil || !reflect.DeepEqual(saved, sampleData()) {
		t.Errorf("snapshot did not round-trip: err=%v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temporary file was left behind")
	}
}

func TestFetchWithSnapshot_APIDownUsesTheSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upstream.json")
	if err := saveSnapshot(path, sampleData()); err != nil {
		t.Fatal(err)
	}
	withLive(t, RawData{}, errors.New("status 404"))

	got, fromSnapshot, err := FetchWithSnapshot(path)
	if err != nil || !fromSnapshot {
		t.Fatalf("err=%v fromSnapshot=%v, want nil true", err, fromSnapshot)
	}
	if !reflect.DeepEqual(got, sampleData()) {
		t.Error("snapshot data differs from what was saved")
	}
}

func TestFetchWithSnapshot_EmptyAnswerCountsAsFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upstream.json")
	if err := saveSnapshot(path, sampleData()); err != nil {
		t.Fatal(err)
	}
	withLive(t, RawData{}, nil) // HTTP 200 but no artists

	_, fromSnapshot, err := FetchWithSnapshot(path)
	if err != nil || !fromSnapshot {
		t.Errorf("err=%v fromSnapshot=%v, want the snapshot to be used", err, fromSnapshot)
	}
	// And the empty answer must not overwrite the good snapshot.
	if saved, err := loadSnapshot(path); err != nil || len(saved.Artists) != 1 {
		t.Errorf("good snapshot was damaged: %v", err)
	}
}

func TestFetchWithSnapshot_NothingToFallBackOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	withLive(t, RawData{}, errors.New("no such app"))

	_, _, err := FetchWithSnapshot(path)
	if err == nil || !strings.Contains(err.Error(), "no such app") || !strings.Contains(err.Error(), "no usable snapshot") {
		t.Errorf("error should mention both failures, got: %v", err)
	}
}

func TestFetchWithSnapshot_CorruptSnapshotIsAnErrorNotAPanic(t *testing.T) {
	for name, content := range map[string]string{
		"garbage":    "{not json",
		"no artists": `{"Artists":[],"Locations":[],"Dates":[],"Relations":[]}`,
		"empty file": "",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "upstream.json")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			withLive(t, RawData{}, errors.New("down"))
			if _, _, err := FetchWithSnapshot(path); err == nil {
				t.Error("expected an error for an unusable snapshot")
			}
		})
	}
}

func TestFetchWithSnapshot_SavingFailureDoesNotStopTheServer(t *testing.T) {
	// A regular file where the folder should be: MkdirAll must fail.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	withLive(t, sampleData(), nil)

	got, fromSnapshot, err := FetchWithSnapshot(filepath.Join(blocker, "upstream.json"))
	if err != nil || fromSnapshot || len(got.Artists) != 1 {
		t.Errorf("err=%v fromSnapshot=%v artists=%d, want live data and no error", err, fromSnapshot, len(got.Artists))
	}
}
