package router

// End-to-end tests for organization and exam authoring against a real
// Postgres. They are skipped unless TEST_DATABASE_URL points at a disposable
// database, which is reset by running every migration down and up again:
//
//	TEST_DATABASE_URL=postgres://user:password@localhost:5433/benchmarq_test?sslmode=disable go test ./internal/router/

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/logger"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

type apiClient struct {
	t    *testing.T
	base string
	http *http.Client
}

type apiResponse struct {
	Status  int
	Message string            `json:"message"`
	Data    json.RawMessage   `json:"data"`
	Errors  map[string]string `json:"errors"`
}

func (c *apiClient) do(method, path string, body any) apiResponse {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()

	out := apiResponse{Status: resp.StatusCode}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		c.t.Fatalf("%s %s: decode response: %v", method, path, err)
	}
	return out
}

// must performs a request and fails the test unless the status matches.
func (c *apiClient) must(status int, method, path string, body any) apiResponse {
	c.t.Helper()
	r := c.do(method, path, body)
	if r.Status != status {
		c.t.Fatalf("%s %s: status %d, want %d (message=%q errors=%v)", method, path, r.Status, status, r.Message, r.Errors)
	}
	return r
}

func decode[T any](t *testing.T, r apiResponse) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.Data, &v); err != nil {
		t.Fatalf("decode data: %v\n%s", err, r.Data)
	}
	return v
}

type obj = map[string]any

func setupServer(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	_, file, _, _ := runtime.Caller(0)
	migrations := "file://" + filepath.Join(filepath.Dir(file), "..", "..", "migrations")
	m, err := migrate.New(migrations, dsn)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []func() error{m.Down, m.Up, m.Down, m.Up} {
		if err := step(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Fatalf("migrate: %v", err)
		}
	}

	pool, err := database.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	cfg := &config.Config{
		JWTSecret:              "integration_test_secret_0123456789",
		JWTAccessExpiryMinutes: 15,
		JWTRefreshExpiryDays:   7,
		JWTIssuer:              "benchmarq",
		CookieHTTPOnly:         true,
		CookieSameSite:         "lax",
		CookiePath:             "/",
		FrontendURL:            "http://localhost:3000",
		AllowedOrigins:         []string{"http://localhost:3000"},
		Environment:            "test",
	}
	srv := httptest.NewServer(NewRouter(config.NewAppConfig(cfg, logger.NewErrorLogger(), nil, pool)))
	t.Cleanup(srv.Close)
	return srv, pool
}

func signUp(t *testing.T, srv *httptest.Server, email string) (*apiClient, string) {
	jar, _ := cookiejar.New(nil)
	c := &apiClient{t: t, base: srv.URL + "/api/v1", http: &http.Client{Jar: jar}}
	r := c.must(http.StatusCreated, "POST", "/auth/signup", obj{
		"email": email, "password": "Password123!", "first_name": "Test", "last_name": "User",
	})
	user := decode[struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}](t, r)
	return c, user.User.ID
}

type examDetail struct {
	Exam struct {
		ID         string  `json:"id"`
		Status     string  `json:"status"`
		TotalMarks float64 `json:"total_marks"`
	} `json:"exam"`
	Config   map[string]any `json:"config"`
	Sections []sectionView  `json:"sections"`
}

type sectionView struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Position      int     `json:"position"`
	QuestionCount int     `json:"question_count"`
	MaxMarks      float64 `json:"max_marks"`
	Effective     struct {
		ShuffleQuestions  bool `json:"shuffle_questions"`
		ShuffleOptions    bool `json:"shuffle_options"`
		QuestionPickCount *int `json:"question_pick_count"`
		DurationMinutes   *int `json:"duration_minutes"`
		LockOnComplete    bool `json:"lock_on_complete"`
		LockedAtStart     bool `json:"locked_at_start"`
		Prerequisites     []struct {
			SectionID string `json:"section_id"`
		} `json:"prerequisites"`
	} `json:"effective"`
}

type question struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Position int    `json:"position"`
	Options  []struct {
		ID        string `json:"id"`
		IsCorrect bool   `json:"is_correct"`
		IsPinned  bool   `json:"is_pinned"`
	} `json:"options"`
}

type readiness struct {
	Ready      bool    `json:"ready"`
	TotalMarks float64 `json:"total_marks"`
	Errors     []struct {
		Code string `json:"code"`
		Path string `json:"path"`
	} `json:"errors"`
}

func mcq(prompt string, points float64) obj {
	return obj{"type": "multiple_choice", "prompt": prompt, "points": points, "options": []obj{
		{"content": "1"}, {"content": "2", "is_correct": true}, {"content": "3"}, {"content": "None of the above", "is_pinned": true},
	}}
}

func TestAuthoringEndToEnd(t *testing.T) {
	srv, pool := setupServer(t)
	ctx := context.Background()

	alice, _ := signUp(t, srv, "alice@benchmarq.io")
	bob, bobID := signUp(t, srv, "bob@benchmarq.io")

	// ----- Organizations --------------------------------------------------
	org := decode[struct {
		ID     string `json:"id"`
		Slug   string `json:"slug"`
		MyRole string `json:"my_role"`
	}](t, alice.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Acme Labs"}))
	if org.Slug != "acme-labs" || org.MyRole != "owner" {
		t.Fatalf("unexpected org %+v", org)
	}
	dup := decode[struct {
		Slug string `json:"slug"`
	}](t, alice.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Acme Labs"}))
	if dup.Slug == "acme-labs" {
		t.Fatal("generated slug collision should be disambiguated")
	}
	alice.must(http.StatusConflict, "POST", "/orgs", obj{"name": "Other", "slug": "acme-labs"})

	orgPath := "/orgs/" + org.ID
	bob.must(http.StatusNotFound, "GET", orgPath, nil)

	if _, err := pool.Exec(ctx, `INSERT INTO organization_members (org_id, user_id, role) VALUES ($1, $2, 'instructor')`, org.ID, bobID); err != nil {
		t.Fatal(err)
	}
	bob.must(http.StatusOK, "GET", orgPath, nil)
	bob.must(http.StatusForbidden, "PATCH", orgPath, obj{"name": "Hijacked"})
	alice.must(http.StatusOK, "PATCH", orgPath, obj{"website": "https://acme.example"})
	alice.must(http.StatusUnprocessableEntity, "PATCH", orgPath, obj{"slug": "Not A Slug"})

	// ----- Exam creation with inline config -------------------------------
	examsPath := orgPath + "/exams"
	r := alice.must(http.StatusUnprocessableEntity, "POST", examsPath, obj{
		"title": "Bad", "config": obj{"schedule": obj{
			"schedule_type": "fixed_window", "start_time": "2026-12-01T10:00:00Z", "end_time": "2026-12-01T09:00:00Z",
		}},
	})
	if r.Errors["config.schedule.end_time"] == "" {
		t.Fatalf("expected inverted window error, got %v", r.Errors)
	}
	detail := decode[examDetail](t, alice.must(http.StatusCreated, "POST", examsPath, obj{
		"title": "Placement Mock 1",
		"code":  "PM-1",
		"config": obj{
			"schedule":      obj{"schedule_type": "open_ended", "total_duration_minutes": 90, "per_section_timing": true},
			"navigation":    obj{"section_progression_mode": "linear_strict"},
			"randomization": obj{"shuffle_questions": true, "shuffle_options": true},
		},
	}))
	setting := func(group, key string) any { return detail.Config[group].(map[string]any)[key] }
	if setting("schedule", "total_duration_minutes") != 90.0 || setting("attempts", "max_attempts") != 1.0 ||
		setting("navigation", "section_progression_mode") != "linear_strict" {
		t.Fatalf("config patch or defaults not applied: %v", detail.Config)
	}
	alice.must(http.StatusConflict, "POST", examsPath, obj{"title": "Dup code", "code": "PM-1"})
	examPath := examsPath + "/" + detail.Exam.ID

	// Config PATCH validation surfaces rule paths relative to the body.
	r = alice.must(http.StatusUnprocessableEntity, "PATCH", examPath+"/config", obj{"proctoring": obj{"webcam_snapshots_enabled": true}})
	if _, ok := r.Errors["proctoring.webcam_snapshots_enabled"]; !ok {
		t.Fatalf("expected proctoring error, got %v", r.Errors)
	}
	alice.must(http.StatusUnprocessableEntity, "PATCH", examPath+"/config", obj{"unknown_field": true})

	// ----- Sections -------------------------------------------------------
	sectionsPath := examPath + "/sections"
	quant := decode[sectionView](t, alice.must(http.StatusCreated, "POST", sectionsPath, obj{
		"name": "Quant", "section_type": "objective", "duration_minutes": 40, "question_pick_count": 2,
	}))
	essay := decode[sectionView](t, alice.must(http.StatusCreated, "POST", sectionsPath, obj{
		"name": "Writing", "section_type": "subjective", "duration_minutes": 20,
		"prerequisites": []obj{{"section_id": quant.ID, "min_score_percent": 40}},
	}))
	verbal := decode[sectionView](t, alice.must(http.StatusCreated, "POST", sectionsPath, obj{
		"name": "Verbal", "section_type": "objective", "duration_minutes": 30, "shuffle_options": false, "position": 1,
	}))
	if verbal.Position != 1 {
		t.Fatalf("section should be inserted at position 1, got %d", verbal.Position)
	}

	// Score gates need auto-gradable prerequisites; cycles are rejected.
	r = alice.must(http.StatusUnprocessableEntity, "PATCH", sectionsPath+"/"+verbal.ID, obj{
		"prerequisites": []obj{{"section_id": essay.ID, "min_score_percent": 50}},
	})
	r = alice.must(http.StatusUnprocessableEntity, "PATCH", sectionsPath+"/"+quant.ID, obj{
		"prerequisites": []obj{{"section_id": essay.ID}},
	})
	if r.Errors["prerequisites"] == "" {
		t.Fatalf("expected cycle error on prerequisites, got %v", r.Errors)
	}
	alice.must(http.StatusUnprocessableEntity, "PATCH", sectionsPath+"/"+quant.ID, obj{"duration_minutes": 500})
	alice.must(http.StatusUnprocessableEntity, "PATCH", sectionsPath+"/"+quant.ID, obj{"section_type": "coding"})
	// Shrinking the exam below a section's own timer is refused.
	alice.must(http.StatusUnprocessableEntity, "PATCH", examPath+"/config", obj{"schedule": obj{"total_duration_minutes": 35}})

	// ----- Questions ------------------------------------------------------
	qPath := func(sectionID string) string { return sectionsPath + "/" + sectionID + "/questions" }

	var quantQs []question
	for i := range 3 {
		quantQs = append(quantQs, decode[question](t, alice.must(http.StatusCreated, "POST", qPath(quant.ID), mcq(fmt.Sprintf("Q%d", i), 2))))
	}
	if !quantQs[0].Options[3].IsPinned || !quantQs[0].Options[1].IsCorrect {
		t.Fatalf("options not persisted correctly: %+v", quantQs[0].Options)
	}

	alice.must(http.StatusCreated, "POST", qPath(verbal.ID), obj{
		"type": "multi_select", "prompt": "Pick synonyms of fast", "points": 3,
		"options":      []obj{{"content": "quick", "is_correct": true}, {"content": "rapid", "is_correct": true}, {"content": "slow"}},
		"multi_select": obj{"min_choices_required": 1, "max_choices_allowed": 2},
	})
	alice.must(http.StatusCreated, "POST", qPath(verbal.ID), obj{
		"type": "true_false", "prompt": "Go has generics", "points": 1, "true_false": obj{"correct_answer": true},
	})
	alice.must(http.StatusCreated, "POST", qPath(verbal.ID), obj{
		"type": "blank_fill", "prompt": "Capital of France", "points": 1,
		"blank_fill": obj{"correct_answer": "Paris", "alternate_answers": []string{"paris city"}},
	})
	alice.must(http.StatusCreated, "POST", qPath(essay.ID), obj{
		"type": "essay", "prompt": "Describe a hard bug you fixed", "points": 10, "essay": obj{"min_words": 100, "max_words": 500},
	})
	r = alice.must(http.StatusUnprocessableEntity, "POST", qPath(essay.ID), mcq("wrong family", 1))
	if r.Errors["type"] == "" {
		t.Fatalf("expected family error, got %v", r.Errors)
	}

	// Reorder and replace.
	reordered := decode[[]question](t, alice.must(http.StatusOK, "PUT", qPath(quant.ID)+"/order", obj{
		"question_ids": []string{quantQs[2].ID, quantQs[1].ID, quantQs[0].ID},
	}))
	if reordered[0].ID != quantQs[2].ID || reordered[2].Position != 2 {
		t.Fatalf("reorder failed: %+v", reordered)
	}
	alice.must(http.StatusUnprocessableEntity, "PUT", qPath(quant.ID)+"/order", obj{"question_ids": []string{quantQs[0].ID}})
	replaced := decode[question](t, alice.must(http.StatusOK, "PUT", qPath(quant.ID)+"/"+quantQs[1].ID, obj{
		"type": "multi_select", "prompt": "Q1 now multi", "points": 2,
		"options": []obj{{"content": "a", "is_correct": true}, {"content": "b"}},
	}))
	if replaced.Type != "multi_select" || replaced.Position != 1 || replaced.ID != quantQs[1].ID {
		t.Fatalf("replace failed: %+v", replaced)
	}

	// Deleting a section full of answer-keyed questions cascades cleanly.
	scratch := decode[sectionView](t, alice.must(http.StatusCreated, "POST", sectionsPath, obj{"name": "Scratch", "section_type": "objective", "position": 0}))
	alice.must(http.StatusCreated, "POST", qPath(scratch.ID), mcq("tmp", 1))
	alice.must(http.StatusCreated, "POST", qPath(scratch.ID), obj{
		"type": "multi_select", "prompt": "tmp", "points": 1, "options": []obj{{"content": "a", "is_correct": true}, {"content": "b"}},
	})
	alice.must(http.StatusOK, "DELETE", sectionsPath+"/"+scratch.ID, nil)

	// ----- Readiness and lifecycle -----------------------------------------
	ready := decode[readiness](t, alice.must(http.StatusOK, "GET", examPath+"/readiness", nil))
	if ready.Ready || len(ready.Errors) != 1 || ready.Errors[0].Code != "subjective_needs_manual" {
		t.Fatalf("expected only the grading error, got %+v", ready)
	}
	r = alice.must(http.StatusUnprocessableEntity, "POST", examPath+"/publish", nil)
	if r.Errors["config.scoring.grading_strategy"] == "" {
		t.Fatalf("publish should report readiness errors, got %v", r.Errors)
	}
	alice.must(http.StatusOK, "PATCH", examPath+"/config", obj{"scoring": obj{"grading_strategy": "hybrid"}})

	full := decode[examDetail](t, alice.must(http.StatusOK, "GET", examPath, nil))
	if len(full.Sections) != 3 || full.Sections[0].Name != "Quant" || full.Sections[1].Name != "Verbal" {
		t.Fatalf("unexpected section order: %+v", full.Sections)
	}
	q, v, w := full.Sections[0], full.Sections[1], full.Sections[2]
	if q.MaxMarks != 4 || *q.Effective.QuestionPickCount != 2 || !q.Effective.ShuffleQuestions {
		t.Fatalf("quant effective rules wrong: %+v", q)
	}
	if v.Effective.ShuffleOptions || len(v.Effective.Prerequisites) != 1 || v.Effective.Prerequisites[0].SectionID != quant.ID || !v.Effective.LockOnComplete {
		t.Fatalf("verbal effective rules wrong: %+v", v.Effective)
	}
	if len(w.Effective.Prerequisites) != 2 || *w.Effective.DurationMinutes != 20 {
		t.Fatalf("writing should depend on verbal (implied) and quant (explicit): %+v", w.Effective)
	}
	if full.Exam.TotalMarks != 4+5+10 {
		t.Fatalf("draft total marks = %v, want 19", full.Exam.TotalMarks)
	}

	published := decode[struct {
		Status      string  `json:"status"`
		TotalMarks  float64 `json:"total_marks"`
		PublishedAt *string `json:"published_at"`
	}](t, alice.must(http.StatusOK, "POST", examPath+"/publish", nil))
	if published.Status != "published" || published.TotalMarks != 19 || published.PublishedAt == nil {
		t.Fatalf("unexpected published exam %+v", published)
	}

	// Published exams are immutable.
	alice.must(http.StatusConflict, "PATCH", examPath, obj{"title": "Renamed"})
	alice.must(http.StatusConflict, "PATCH", examPath+"/config", obj{"attempts": obj{"max_attempts": 3}})
	alice.must(http.StatusConflict, "POST", qPath(quant.ID), mcq("late", 2))
	alice.must(http.StatusConflict, "POST", examPath+"/publish", nil)

	alice.must(http.StatusOK, "POST", examPath+"/unpublish", nil)
	alice.must(http.StatusOK, "POST", examPath+"/publish", nil)

	// Once a candidate has attempted the exam it can no longer return to draft or be deleted.
	if _, err := pool.Exec(ctx, `INSERT INTO exam_attempts (exam_id, user_id) VALUES ($1, $2)`, detail.Exam.ID, bobID); err != nil {
		t.Fatal(err)
	}
	alice.must(http.StatusConflict, "POST", examPath+"/unpublish", nil)
	alice.must(http.StatusConflict, "DELETE", examPath, nil)
	alice.must(http.StatusOK, "POST", examPath+"/archive", nil)
	alice.must(http.StatusConflict, "POST", examPath+"/restore", nil)

	// ----- RBAC on exams --------------------------------------------------
	bob.must(http.StatusCreated, "POST", examsPath, obj{"title": "Bob's quiz"})
	bob.must(http.StatusForbidden, "DELETE", examPath, nil) // instructors cannot delete
	alice.must(http.StatusOK, "PATCH", orgPath+"/members/"+bobID, obj{"role": "proctor"})
	bob.must(http.StatusForbidden, "POST", examsPath, obj{"title": "Not allowed"})
	bob.must(http.StatusForbidden, "GET", examsPath, nil) // proctors cannot see answer keys

	// ----- Listing with keyset pagination ----------------------------------
	alice.must(http.StatusCreated, "POST", examsPath, obj{"title": "Third"})
	page1 := decode[struct {
		Items      []struct{ Title string } `json:"items"`
		NextCursor string                   `json:"next_cursor"`
	}](t, alice.must(http.StatusOK, "GET", examsPath+"?limit=2", nil))
	if len(page1.Items) != 2 || page1.NextCursor == "" || page1.Items[0].Title != "Third" {
		t.Fatalf("unexpected first page %+v", page1)
	}
	page2 := decode[struct {
		Items      []struct{ Title string } `json:"items"`
		NextCursor string                   `json:"next_cursor"`
	}](t, alice.must(http.StatusOK, "GET", examsPath+"?limit=2&cursor="+page1.NextCursor, nil))
	if len(page2.Items) != 1 || page2.NextCursor != "" || page2.Items[0].Title != "Placement Mock 1" {
		t.Fatalf("unexpected second page %+v", page2)
	}
	archived := decode[struct {
		Items []struct{ Title string } `json:"items"`
	}](t, alice.must(http.StatusOK, "GET", examsPath+"?status=archived&q=mock", nil))
	if len(archived.Items) != 1 {
		t.Fatalf("status/search filter failed: %+v", archived)
	}
}

func TestMembershipRules(t *testing.T) {
	srv, pool := setupServer(t)
	ctx := context.Background()

	owner, ownerID := signUp(t, srv, "owner@benchmarq.io")
	admin, adminID := signUp(t, srv, "admin@benchmarq.io")
	_, instructorID := signUp(t, srv, "instructor@benchmarq.io")

	org := decode[struct {
		ID string `json:"id"`
	}](t, owner.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Members Inc"}))
	for id, role := range map[string]string{adminID: "admin", instructorID: "instructor"} {
		if _, err := pool.Exec(ctx, `INSERT INTO organization_members (org_id, user_id, role) VALUES ($1, $2, $3)`, org.ID, id, role); err != nil {
			t.Fatal(err)
		}
	}
	members := "/orgs/" + org.ID + "/members/"

	admin.must(http.StatusForbidden, "PATCH", members+instructorID, obj{"role": "admin"}) // cannot grant own rank
	admin.must(http.StatusOK, "PATCH", members+instructorID, obj{"role": "proctor"})
	admin.must(http.StatusForbidden, "PATCH", members+ownerID, obj{"role": "candidate"}) // cannot touch owner
	admin.must(http.StatusForbidden, "PATCH", members+adminID, obj{"role": "owner"})     // no self-promotion
	owner.must(http.StatusConflict, "DELETE", members+ownerID, nil)                      // last owner cannot leave

	owner.must(http.StatusOK, "PATCH", members+adminID, obj{"role": "owner"})
	owner.must(http.StatusOK, "DELETE", members+ownerID, nil) // another owner exists now
	owner.must(http.StatusNotFound, "GET", "/orgs/"+org.ID, nil)

	list := decode[struct {
		Items []struct {
			Role string `json:"role"`
		} `json:"items"`
	}](t, admin.must(http.StatusOK, "GET", members, nil))
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 remaining members, got %+v", list.Items)
	}
	admin.must(http.StatusOK, "DELETE", "/orgs/"+org.ID, nil)
	admin.must(http.StatusNotFound, "GET", "/orgs/"+org.ID, nil)
}

func TestCodingQuestionsAndIsolation(t *testing.T) {
	srv, _ := setupServer(t)

	alice, _ := signUp(t, srv, "alice2@benchmarq.io")
	mallory, _ := signUp(t, srv, "mallory@benchmarq.io")

	orgA := decode[struct {
		ID string `json:"id"`
	}](t, alice.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Alice Org"}))
	orgM := decode[struct {
		ID string `json:"id"`
	}](t, mallory.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Mallory Org"}))

	examsPath := "/orgs/" + orgA.ID + "/exams"
	exam := decode[examDetail](t, alice.must(http.StatusCreated, "POST", examsPath, obj{"title": "DSA Round"}))
	sectionsPath := examsPath + "/" + exam.Exam.ID + "/sections"
	coding := decode[sectionView](t, alice.must(http.StatusCreated, "POST", sectionsPath, obj{"name": "Coding", "section_type": "coding"}))
	writing := decode[sectionView](t, alice.must(http.StatusCreated, "POST", sectionsPath, obj{"name": "Short", "section_type": "subjective"}))
	qPath := sectionsPath + "/" + coding.ID + "/questions"

	created := decode[struct {
		ID     string `json:"id"`
		Coding struct {
			TimeLimitMs int `json:"time_limit_ms"`
			TestCases   []struct {
				IsHidden bool    `json:"is_hidden"`
				Points   float64 `json:"points"`
			} `json:"test_cases"`
		} `json:"coding"`
	}](t, alice.must(http.StatusCreated, "POST", qPath, obj{
		"type": "coding", "prompt": "Reverse a linked list", "points": 20,
		"coding": obj{
			"starter_code": "func reverse(head *Node) *Node {}",
			"test_cases": []obj{
				{"input": "1 2 3", "expected_output": "3 2 1", "is_hidden": false},
				{"input": "", "expected_output": "", "points": 2},
			},
		},
	}))
	if created.Coding.TimeLimitMs != 2000 || len(created.Coding.TestCases) != 2 ||
		created.Coding.TestCases[0].IsHidden || !created.Coding.TestCases[1].IsHidden || created.Coding.TestCases[1].Points != 2 {
		t.Fatalf("coding question not persisted as expected: %+v", created.Coding)
	}
	alice.must(http.StatusUnprocessableEntity, "POST", qPath, obj{
		"type": "coding", "prompt": "x", "points": 1,
		"coding": obj{"default_language_id": "00000000-0000-0000-0000-000000000001", "test_cases": []obj{{"input": "1", "expected_output": "1"}}},
	})
	alice.must(http.StatusUnprocessableEntity, "POST", qPath, obj{"type": "coding", "prompt": "x", "points": 1, "coding": obj{"test_cases": []obj{}}})

	shortPath := sectionsPath + "/" + writing.ID + "/questions"
	var shortIDs []string
	for i := range 3 {
		q := decode[question](t, alice.must(http.StatusCreated, "POST", shortPath, obj{
			"type": "short_answer", "prompt": fmt.Sprintf("Define term %d", i), "points": 2,
			"short_answer": obj{"max_words": 50, "rubric_keywords": []string{"latency"}},
		}))
		shortIDs = append(shortIDs, q.ID)
	}
	alice.must(http.StatusOK, "DELETE", shortPath+"/"+shortIDs[0], nil)
	remaining := decode[[]question](t, alice.must(http.StatusOK, "GET", shortPath, nil))
	if len(remaining) != 2 || remaining[0].ID != shortIDs[1] || remaining[0].Position != 0 || remaining[1].Position != 1 {
		t.Fatalf("positions not compacted after delete: %+v", remaining)
	}

	// Tenant isolation: nothing in Alice's org is reachable from Mallory's,
	// even when Mallory knows the IDs.
	mallory.must(http.StatusNotFound, "GET", examsPath+"/"+exam.Exam.ID, nil)
	foreign := "/orgs/" + orgM.ID + "/exams/" + exam.Exam.ID
	mallory.must(http.StatusNotFound, "GET", foreign, nil)
	mallory.must(http.StatusNotFound, "GET", foreign+"/sections/"+coding.ID+"/questions", nil)
	mallory.must(http.StatusNotFound, "POST", foreign+"/sections/"+coding.ID+"/questions", obj{
		"type": "coding", "prompt": "pwn", "points": 1, "coding": obj{"test_cases": []obj{{"input": "", "expected_output": ""}}},
	})
	mallory.must(http.StatusNotFound, "DELETE", foreign, nil)
}
