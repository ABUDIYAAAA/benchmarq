package router

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// testSnapshotDir is the snapshot root of the most recent setupServer.
var testSnapshotDir string

// uploadSnapshots posts a multipart form as the desktop app does.
func (c *apiClient) uploadSnapshots(parts map[string][]byte) int {
	c.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, data := range parts {
		w, err := mw.CreateFormFile(name, name+".jpg")
		if err != nil {
			c.t.Fatal(err)
		}
		w.Write(data)
	}
	mw.Close()

	req, _ := http.NewRequest("POST", c.base+"/runtime/snapshots", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestSnapshotUploads(t *testing.T) {
	srv, pool := setupServer(t)

	owner, _ := signUp(t, srv, "snap-owner@benchmarq.io")
	cand, candID := signUp(t, srv, "snap-cand@benchmarq.io")
	org := decode[struct {
		ID string `json:"id"`
	}](t, owner.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Snapshot Org"}))
	addMember(t, pool, org.ID, candID, "candidate")
	examID, _ := publishExam(t, owner, org.ID, candID, obj{
		"schedule": obj{"schedule_type": "open_ended", "total_duration_minutes": 30, "min_time_before_submission_minutes": 0},
	}, sectionSpec{obj{"name": "Quant", "section_type": "objective"}, []obj{mcqQ("Q1")}})

	dev := desktop(t, srv, "device-snapshots-000", "")
	dev.openSession(launchCode(t, cand, examID))

	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	if code := dev.uploadSnapshots(map[string][]byte{"webcam": jpeg}); code != http.StatusConflict {
		t.Fatalf("upload before the attempt starts: got %d, want 409", code)
	}

	started := decode[struct {
		Attempt struct {
			ID string `json:"id"`
		} `json:"attempt"`
	}](t, dev.must(http.StatusOK, "POST", "/runtime/start", nil))

	if code := dev.uploadSnapshots(map[string][]byte{"webcam": jpeg, "screen": jpeg}); code != http.StatusOK {
		t.Fatalf("upload: got %d", code)
	}
	dir := filepath.Join(testSnapshotDir, org.ID, examID, started.Attempt.ID)
	for _, kind := range []string{"webcam", "screen"} {
		matches, _ := filepath.Glob(filepath.Join(dir, "*_"+kind+".jpg"))
		if len(matches) != 1 {
			t.Fatalf("expected one %s snapshot in %s, found %v", kind, dir, matches)
		}
		if data, _ := os.ReadFile(matches[0]); !bytes.Equal(data, jpeg) {
			t.Fatalf("%s snapshot content mismatch", kind)
		}
	}

	if code := dev.uploadSnapshots(map[string][]byte{"webcam": jpeg}); code != http.StatusConflict {
		t.Fatalf("immediate second upload should be throttled, got %d", code)
	}
	if code := dev.uploadSnapshots(map[string][]byte{"webcam": []byte("not a jpeg")}); code != http.StatusUnprocessableEntity {
		t.Fatalf("non-JPEG upload: got %d, want 422", code)
	}
	if code := dev.uploadSnapshots(map[string][]byte{"avatar": jpeg}); code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown part only: got %d, want 422", code)
	}

	stranger := desktop(t, srv, "device-stranger-0000", "")
	if code := stranger.uploadSnapshots(map[string][]byte{"webcam": jpeg}); code != http.StatusUnauthorized {
		t.Fatalf("upload without a session: got %d, want 401", code)
	}
}
