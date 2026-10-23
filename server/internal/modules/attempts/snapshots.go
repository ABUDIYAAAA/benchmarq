package attempts

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/internal/storage"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/google/uuid"
)

// Proctoring snapshots: the desktop app uploads a webcam frame and a screen
// capture every 30 seconds while an attempt runs.

const (
	// MaxSnapshotBytes caps one image. Snapshots are JPEGs at low quality,
	// typically well under 300 KB.
	MaxSnapshotBytes = 1 << 20
	// minSnapshotGap throttles uploads per attempt so a client cannot fill
	// the disk; the desktop app uploads every 30 seconds.
	minSnapshotGap = 10 * time.Second
)

var (
	errSnapshotsDisabled = apperr.Conflict("Snapshot uploads are not enabled on this server")
	errSnapshotTooSoon   = apperr.Conflict("Snapshots are being uploaded too frequently")
	errSnapshotInvalid   = apperr.Invalid("Snapshots must be JPEG images")
)

// SnapshotKinds are the accepted image kinds.
var SnapshotKinds = map[string]bool{"webcam": true, "screen": true}

type SnapshotUpload struct {
	Kind string
	Data []byte
}

type SnapshotResult struct {
	Saved   []string  `json:"saved"`
	TakenAt time.Time `json:"taken_at"`
}

// snapshotThrottle remembers the last upload per attempt. It is per process;
// across several API instances each enforces the gap independently.
type snapshotThrottle struct {
	mu   sync.Mutex
	last map[uuid.UUID]time.Time
}

func (t *snapshotThrottle) allow(attemptID uuid.UUID, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.last == nil {
		t.last = make(map[uuid.UUID]time.Time)
	}
	if prev, ok := t.last[attemptID]; ok && now.Sub(prev) < minSnapshotGap {
		return false
	}
	t.last[attemptID] = now
	// Keep the map small: forget attempts idle for an hour.
	for id, at := range t.last {
		if now.Sub(at) > time.Hour {
			delete(t.last, id)
		}
	}
	return true
}

var jpegMagic = []byte{0xFF, 0xD8, 0xFF}

// SaveSnapshots stores the uploaded images under the attempt's organization,
// exam and attempt. Only a running attempt on its bound device may upload.
func (s *service) SaveSnapshots(ctx context.Context, sess *Session, c Client, shots []SnapshotUpload) (*SnapshotResult, error) {
	if s.opts.Snapshots == nil {
		return nil, errSnapshotsDisabled
	}
	for _, shot := range shots {
		if !SnapshotKinds[shot.Kind] || len(shot.Data) == 0 || len(shot.Data) > MaxSnapshotBytes || !bytes.HasPrefix(shot.Data, jpegMagic) {
			return nil, errSnapshotInvalid
		}
	}

	// Resolve and authorize inside the attempt lock, but write files after it.
	var (
		key storage.SnapshotKey
		now time.Time
	)
	err := s.withRuntime(ctx, sess, c, func(ctx context.Context, rt *runtime, t time.Time) error {
		a := rt.attempt()
		if !a.running() {
			return errAttemptEnded
		}
		now = t
		key = storage.SnapshotKey{OrgID: rt.bp.cand.OrgID, ExamID: rt.bp.cand.ExamID, AttemptID: a.ID, TakenAt: t}
		return nil
	})
	if err != nil {
		return nil, s.mapErr(err, "save snapshots")
	}
	if !s.snapshots.allow(key.AttemptID, now) {
		return nil, errSnapshotTooSoon
	}

	result := &SnapshotResult{Saved: []string{}, TakenAt: now}
	for _, shot := range shots {
		k := key
		k.Kind = shot.Kind
		if _, err := s.opts.Snapshots.Save(ctx, k, shot.Data); err != nil {
			s.logger.Error("attempts: store snapshot failed", "attempt_id", key.AttemptID, "kind", shot.Kind, "error", err)
			return nil, apperr.Internal(err, "store snapshot")
		}
		result.Saved = append(result.Saved, shot.Kind)
	}
	return result, nil
}
