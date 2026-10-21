package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ABUDIYAAAA/benchmarq/internal/config"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type sectionSpec struct {
	body      obj
	questions []obj
}

// publishExam creates, fills and publishes an exam and assigns the candidate.
func publishExam(t *testing.T, owner *apiClient, orgID, candidateID string, config obj, sections ...sectionSpec) (string, []string) {
	t.Helper()
	examsPath := "/orgs/" + orgID + "/exams"
	exam := decode[examDetail](t, owner.must(http.StatusCreated, "POST", examsPath, obj{"title": "Runtime exam", "config": config}))
	examPath := examsPath + "/" + exam.Exam.ID
	var ids []string
	for _, spec := range sections {
		sec := decode[sectionView](t, owner.must(http.StatusCreated, "POST", examPath+"/sections", spec.body))
		for _, q := range spec.questions {
			owner.must(http.StatusCreated, "POST", examPath+"/sections/"+sec.ID+"/questions", q)
		}
		ids = append(ids, sec.ID)
	}
	owner.must(http.StatusOK, "POST", examPath+"/publish", nil)
	owner.must(http.StatusOK, "POST", examPath+"/assignments", obj{"user_ids": []string{candidateID}})
	return exam.Exam.ID, ids
}

// answerKey maps question IDs to their correct option IDs, read through the
// authoring API.
func answerKey(t *testing.T, owner *apiClient, orgID, examID, sectionID string) map[string][]string {
	t.Helper()
	qs := decode[[]question](t, owner.must(http.StatusOK, "GET", "/orgs/"+orgID+"/exams/"+examID+"/sections/"+sectionID+"/questions", nil))
	key := map[string][]string{}
	for _, q := range qs {
		for _, o := range q.Options {
			if o.IsCorrect {
				key[q.ID] = append(key[q.ID], o.ID)
			}
		}
	}
	return key
}

// desktop is an exam desktop app: no cookies, a device ID and, optionally,
// a forwarded client address as a reverse proxy would send it.
func desktop(t *testing.T, srv *httptest.Server, deviceID, forwardedFor string) *apiClient {
	h := map[string]string{"X-Device-ID": deviceID}
	if forwardedFor != "" {
		h["X-Forwarded-For"] = forwardedFor
	}
	return &apiClient{t: t, base: srv.URL + "/api/v1", http: &http.Client{}, headers: h}
}

func launchCode(t *testing.T, candidate *apiClient, examID string) string {
	t.Helper()
	grant := decode[struct {
		Code      string `json:"code"`
		LaunchURL string `json:"launch_url"`
	}](t, candidate.must(http.StatusCreated, "POST", "/me/exams/"+examID+"/launch", nil))
	if !strings.HasPrefix(grant.LaunchURL, "benchmarq://launch?code=") {
		t.Fatalf("unexpected launch url %q", grant.LaunchURL)
	}
	return grant.Code
}

// openSession redeems a launch code and keeps the session token.
func (c *apiClient) openSession(code string) apiResponse {
	c.t.Helper()
	r := c.do("POST", "/runtime/sessions", obj{"launch_code": code, "device_id": c.headers["X-Device-ID"]})
	if r.Status == http.StatusCreated {
		var grant struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(r.Data, &grant); err != nil {
			c.t.Fatal(err)
		}
		c.headers["Authorization"] = "Bearer " + grant.Token
	}
	return r
}

type runtimeState struct {
	Availability *struct {
		State     string `json:"state"`
		CanStart  bool   `json:"can_start"`
		CanLaunch bool   `json:"can_launch"`
	} `json:"availability"`
	Attempt *attemptState `json:"attempt"`
}

type attemptState struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	EndedReason *string `json:"ended_reason"`
	Clock       *struct {
		RemainingSeconds int  `json:"remaining_seconds"`
		Paused           bool `json:"paused"`
		Section          *struct {
			SectionID        string `json:"section_id"`
			RemainingSeconds int    `json:"remaining_seconds"`
		} `json:"section"`
	} `json:"clock"`
	Sections []struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		IsOpen     bool   `json:"is_open"`
		CanEnter   bool   `json:"can_enter"`
		LockReason string `json:"lock_reason"`
	} `json:"sections"`
	Current *struct {
		SectionID string `json:"section_id"`
		Cursor    int    `json:"cursor"`
	} `json:"current_section"`
	Violations map[string]int `json:"violations"`
	Answered   int            `json:"answered"`
	Total      int            `json:"total"`
}

type questionState struct {
	SectionID  string `json:"section_id"`
	Position   int    `json:"position"`
	QuestionID string `json:"question_id"`
	Type       string `json:"type"`
	Options    []struct {
		ID string `json:"id"`
	} `json:"options"`
	Penalty *float64 `json:"penalty"`
}

func (c *apiClient) attempt(status int) *attemptState {
	c.t.Helper()
	return decode[runtimeState](c.t, c.must(status, "GET", "/runtime/state", nil)).Attempt
}

// rewind moves an attempt's history into the past, as if that much time had
// passed since it was last touched.
func rewind(t *testing.T, pool *pgxpool.Pool, attemptID, by string) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		`UPDATE exam_attempts SET started_at = started_at - $2::interval, timer_origin_at = timer_origin_at - $2::interval,
		        deadline_at = deadline_at - $2::interval, last_heartbeat_at = last_heartbeat_at - $2::interval,
		        paused_at = paused_at - $2::interval WHERE id = $1`,
		`UPDATE attempt_section_states SET entered_at = entered_at - $2::interval, unlocked_at = unlocked_at - $2::interval,
		        completed_at = completed_at - $2::interval WHERE attempt_id = $1`,
		`UPDATE attempt_questions SET first_viewed_at = first_viewed_at - $2::interval WHERE attempt_id = $1`,
	} {
		if _, err := pool.Exec(ctx, q, attemptID, by); err != nil {
			t.Fatal(err)
		}
	}
}

func addMember(t *testing.T, pool *pgxpool.Pool, orgID, userID, role string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO organization_members (org_id, user_id, role) VALUES ($1, $2, $3)`, orgID, userID, role); err != nil {
		t.Fatal(err)
	}
}

func mcqQ(prompt string) obj {
	return obj{"type": "multiple_choice", "prompt": prompt, "points": 4, "options": []obj{
		{"content": "right", "is_correct": true}, {"content": "wrong"}, {"content": "also wrong"}, {"content": "None of these", "is_pinned": true},
	}}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRuntimeEndToEnd(t *testing.T) {
	srv, pool := setupServer(t)
	ctx := context.Background()

	owner, _ := signUp(t, srv, "rt-owner@benchmarq.io")
	cand, candID := signUp(t, srv, "rt-cand@benchmarq.io")
	proctor, proctorID := signUp(t, srv, "rt-proctor@benchmarq.io")
	org := decode[struct {
		ID string `json:"id"`
	}](t, owner.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Runtime Org"}))
	addMember(t, pool, org.ID, candID, "candidate")
	addMember(t, pool, org.ID, proctorID, "proctor")

	examID, sections := publishExam(t, owner, org.ID, candID, obj{
		"schedule":   obj{"schedule_type": "open_ended", "total_duration_minutes": 30, "min_time_before_submission_minutes": 0},
		"navigation": obj{"section_progression_mode": "linear_strict"},
		"scoring":    obj{"grading_strategy": "hybrid", "negative_marking_enabled": true, "negative_marking_penalty_percent": 25},
		"proctoring": obj{"track_tab_focus": true, "max_tab_switch_count": 2, "tab_switch_grace_seconds": 3},
	},
		sectionSpec{obj{"name": "Quant", "section_type": "objective"}, []obj{mcqQ("Q1"), mcqQ("Q2"), {
			"type": "multi_select", "prompt": "Pick", "points": 2,
			"options": []obj{{"content": "a", "is_correct": true}, {"content": "b", "is_correct": true}, {"content": "c"}},
		}}},
		sectionSpec{obj{"name": "Writing", "section_type": "subjective"}, []obj{{
			"type": "essay", "prompt": "Explain", "points": 10, "essay": obj{"max_words": 5},
		}}},
	)
	quant, writing := sections[0], sections[1]
	key := answerKey(t, owner, org.ID, examID, quant)

	// ----- Portal ------------------------------------------------------------
	mine := decode[[]struct {
		ExamID       string `json:"exam_id"`
		Availability struct {
			State     string `json:"state"`
			CanLaunch bool   `json:"can_launch"`
		} `json:"availability"`
	}](t, cand.must(http.StatusOK, "GET", "/me/exams", nil))
	if len(mine) != 1 || mine[0].ExamID != examID || mine[0].Availability.State != "open" || !mine[0].Availability.CanLaunch {
		t.Fatalf("unexpected candidate exams %+v", mine)
	}
	owner.must(http.StatusNotFound, "POST", "/me/exams/"+examID+"/launch", nil) // not assigned

	// ----- Device binding ------------------------------------------------------
	devA := desktop(t, srv, "device-A-0000000000", "")
	devB := desktop(t, srv, "device-B-0000000000", "")
	codeA := launchCode(t, cand, examID)
	devA.openSession(codeA)
	if r := devB.do("POST", "/runtime/sessions", obj{"launch_code": codeA, "device_id": "device-B-0000000000"}); r.Status != http.StatusUnauthorized {
		t.Fatalf("a launch code is single-use, got %d", r.Status)
	}
	pre := decode[runtimeState](t, devA.must(http.StatusOK, "GET", "/runtime/state", nil))
	if pre.Attempt != nil || pre.Availability == nil || !pre.Availability.CanStart {
		t.Fatalf("expected a pre-start state, got %+v", pre)
	}

	// The latest launch wins before the exam starts; the older session dies.
	devB.openSession(launchCode(t, cand, examID))
	devA.must(http.StatusUnauthorized, "GET", "/runtime/state", nil)
	started := decode[runtimeState](t, devB.must(http.StatusOK, "POST", "/runtime/start", nil)).Attempt
	if started.Status != "in_progress" || started.Current == nil || started.Current.SectionID != quant {
		t.Fatalf("linear exams open the first section on start: %+v", started)
	}
	devB.must(http.StatusConflict, "POST", "/runtime/start", nil)

	// Once started, only device B may continue.
	if r := devA.openSession(launchCode(t, cand, examID)); r.Status != http.StatusConflict {
		t.Fatalf("another device must be refused, got %d", r.Status)
	}
	wrongDevice := *devB
	wrongDevice.headers = map[string]string{"Authorization": devB.headers["Authorization"], "X-Device-ID": "device-C-0000000000"}
	wrongDevice.must(http.StatusUnauthorized, "GET", "/runtime/state", nil)
	cand.must(http.StatusUnauthorized, "GET", "/runtime/state", nil) // portal cookies do not work here

	// ----- Questions -----------------------------------------------------------
	r := devB.must(http.StatusOK, "GET", "/runtime/question", nil)
	for _, leak := range []string{"is_correct", "correct_answer", "explanation", "weight", "rubric", "sample_answer"} {
		if strings.Contains(string(r.Data), leak) {
			t.Fatalf("question view leaks %q: %s", leak, r.Data)
		}
	}
	q0 := decode[questionState](t, r)
	if q0.Position != 0 || len(q0.Options) != 4 || q0.Penalty == nil || *q0.Penalty != 1 {
		t.Fatalf("unexpected first question %+v", q0)
	}
	devB.must(http.StatusConflict, "PUT", "/runtime/question/response", obj{"question_id": quant, "selected_option_id": key[q0.QuestionID][0]})
	devB.must(http.StatusUnprocessableEntity, "PUT", "/runtime/question/response", obj{"question_id": q0.QuestionID, "boolean": true})
	devB.must(http.StatusUnprocessableEntity, "PUT", "/runtime/question/response", obj{"question_id": q0.QuestionID, "selected_option_id": q0.QuestionID})
	devB.must(http.StatusOK, "PUT", "/runtime/question/response", obj{"question_id": q0.QuestionID, "selected_option_id": key[q0.QuestionID][0], "flagged": true})

	// Second MCQ answered wrong (-1), multi-select fully right (+2).
	q1 := decode[questionState](t, devB.must(http.StatusOK, "POST", "/runtime/question/navigate", obj{"position": 1}))
	for _, o := range q1.Options {
		if o.ID != key[q1.QuestionID][0] {
			devB.must(http.StatusOK, "PUT", "/runtime/question/response", obj{"question_id": q1.QuestionID, "selected_option_id": o.ID})
			break
		}
	}
	q2 := decode[questionState](t, devB.must(http.StatusOK, "POST", "/runtime/question/navigate", obj{"position": 2}))
	devB.must(http.StatusOK, "PUT", "/runtime/question/response", obj{"question_id": q2.QuestionID, "selected_option_ids": key[q2.QuestionID]})
	devB.must(http.StatusUnprocessableEntity, "POST", "/runtime/question/navigate", obj{"position": 3})

	// ----- Sections ------------------------------------------------------------
	devB.must(http.StatusConflict, "POST", "/runtime/sections/"+writing+"/enter", nil)
	after := decode[attemptState](t, devB.must(http.StatusOK, "POST", "/runtime/sections/"+quant+"/finish", nil))
	if after.Current == nil || after.Current.SectionID != writing || after.Answered != 3 {
		t.Fatalf("finishing Quant should open Writing: %+v", after)
	}
	devB.must(http.StatusConflict, "POST", "/runtime/sections/"+quant+"/enter", nil) // sealed

	essay := decode[questionState](t, devB.must(http.StatusOK, "GET", "/runtime/question", nil))
	devB.must(http.StatusUnprocessableEntity, "PUT", "/runtime/question/response", obj{"question_id": essay.QuestionID, "text": "one two three four five six"})
	devB.must(http.StatusOK, "PUT", "/runtime/question/response", obj{"question_id": essay.QuestionID, "text": "short and sweet"})

	// ----- Proctoring events ---------------------------------------------------
	ev := decode[struct {
		Counted bool `json:"counted"`
		Ended   bool `json:"ended"`
	}](t, devB.must(http.StatusOK, "POST", "/runtime/events", obj{"type": "tab_switch", "duration_ms": 1000}))
	if ev.Counted {
		t.Fatal("tab switches under the grace period are not counted")
	}
	devB.must(http.StatusOK, "POST", "/runtime/events", obj{"type": "tab_switch", "duration_ms": 5000})
	devB.must(http.StatusUnprocessableEntity, "POST", "/runtime/events", obj{"type": "ip_changed"}) // server-only

	// ----- Staff view ----------------------------------------------------------
	staffPath := "/orgs/" + org.ID + "/exams/" + examID + "/attempts"
	list := decode[struct {
		Items []struct {
			Status      string `json:"status"`
			DeviceBound bool   `json:"device_bound"`
		} `json:"items"`
	}](t, proctor.must(http.StatusOK, "GET", staffPath, nil))
	if len(list.Items) != 1 || list.Items[0].Status != "in_progress" || !list.Items[0].DeviceBound {
		t.Fatalf("unexpected staff list %+v", list.Items)
	}
	cand.must(http.StatusForbidden, "GET", staffPath, nil)

	// ----- Submit ----------------------------------------------------------------
	done := decode[attemptState](t, devB.must(http.StatusOK, "POST", "/runtime/submit", nil))
	if done.Status != "submitted" || done.Clock != nil || done.Violations["tab_switch"] != 1 {
		t.Fatalf("unexpected submitted attempt %+v", done)
	}
	devB.must(http.StatusConflict, "GET", "/runtime/question", nil)
	devB.must(http.StatusConflict, "POST", "/runtime/submit", nil)
	if a := devB.attempt(http.StatusOK); a.Status != "submitted" {
		t.Fatalf("state should still be readable after submission: %+v", a)
	}

	// Objective score is 4 - 1 + 2 = 5 of 20; the essay awaits manual grading.
	var total, maxScore float64
	var passed *bool
	if err := pool.QueryRow(ctx, `SELECT total_score::float8, max_score::float8, passed FROM exam_attempts WHERE id = $1`, done.ID).
		Scan(&total, &maxScore, &passed); err != nil {
		t.Fatal(err)
	}
	if total != 5 || maxScore != 20 || passed != nil {
		t.Fatalf("score %v / %v passed=%v", total, maxScore, passed)
	}
	again := decode[[]struct {
		Availability struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		} `json:"availability"`
	}](t, cand.must(http.StatusOK, "GET", "/me/exams", nil))
	if again[0].Availability.Reason != "no_attempts_left" {
		t.Fatalf("single-attempt exam should be closed now: %+v", again)
	}
	cand.must(http.StatusConflict, "POST", "/me/exams/"+examID+"/launch", nil)
}

func TestRuntimeTimingAndRecovery(t *testing.T) {
	srv, pool := setupServer(t)
	ctx := context.Background()
	svc := NewAttemptService(config.NewAppConfig(&config.Config{}, logger.NewErrorLogger(), nil, pool))

	owner, _ := signUp(t, srv, "tm-owner@benchmarq.io")
	cand, candID := signUp(t, srv, "tm-cand@benchmarq.io")
	proctor, proctorID := signUp(t, srv, "tm-proctor@benchmarq.io")
	org := decode[struct {
		ID string `json:"id"`
	}](t, owner.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Timing Org"}))
	addMember(t, pool, org.ID, candID, "candidate")
	addMember(t, pool, org.ID, proctorID, "proctor")

	// ----- Timed sections catch up after silence and the exam expires ---------
	timedID, timed := publishExam(t, owner, org.ID, candID, obj{
		"schedule": obj{"schedule_type": "open_ended", "total_duration_minutes": 15, "per_section_timing": true,
			"min_time_before_submission_minutes": 0, "allow_candidate_pause": true, "max_pause_count": 1, "max_total_pause_minutes": 5},
		"navigation": obj{"section_progression_mode": "timed_sequential"},
	},
		sectionSpec{obj{"name": "A", "section_type": "objective", "duration_minutes": 5}, []obj{mcqQ("A1")}},
		sectionSpec{obj{"name": "B", "section_type": "objective", "duration_minutes": 5}, []obj{mcqQ("B1")}},
	)
	key := answerKey(t, owner, org.ID, timedID, timed[0])
	dev := desktop(t, srv, "device-T-0000000000", "")
	dev.openSession(launchCode(t, cand, timedID))
	a := decode[runtimeState](t, dev.must(http.StatusOK, "POST", "/runtime/start", nil)).Attempt
	q := decode[questionState](t, dev.must(http.StatusOK, "GET", "/runtime/question", nil))
	dev.must(http.StatusOK, "PUT", "/runtime/question/response", obj{"question_id": q.QuestionID, "selected_option_id": key[q.QuestionID][0]})

	// Pausing freezes content and is limited.
	dev.must(http.StatusOK, "POST", "/runtime/pause", nil)
	dev.must(http.StatusConflict, "GET", "/runtime/question", nil)
	if s := dev.attempt(http.StatusOK); s.Clock == nil || !s.Clock.Paused {
		t.Fatalf("expected a paused clock: %+v", s.Clock)
	}
	dev.must(http.StatusOK, "POST", "/runtime/resume", nil)
	dev.must(http.StatusConflict, "POST", "/runtime/pause", nil)

	rewind(t, pool, a.ID, "7 minutes")
	s := dev.attempt(http.StatusOK)
	if s.Sections[0].Status != "completed" || s.Current == nil || s.Current.SectionID != timed[1] ||
		s.Clock.Section == nil || s.Clock.Section.RemainingSeconds < 170 || s.Clock.Section.RemainingSeconds > 181 {
		t.Fatalf("section A should have closed and B run for 2 minutes: %+v %+v", s.Sections, s.Clock)
	}
	dev.must(http.StatusConflict, "POST", "/runtime/sections/"+timed[0]+"/enter", nil)

	rewind(t, pool, a.ID, "30 minutes")
	s = dev.attempt(http.StatusOK)
	if s.Status != "evaluated" || s.EndedReason == nil || *s.EndedReason != "time_expired" {
		t.Fatalf("expected auto-submitted and graded on expiry: %+v", s)
	}
	var total float64
	var passed bool
	if err := pool.QueryRow(ctx, `SELECT total_score::float8, passed FROM exam_attempts WHERE id = $1`, a.ID).Scan(&total, &passed); err != nil {
		t.Fatal(err)
	}
	if total != 4 || !passed {
		t.Fatalf("expected 4/8 and a pass at 50%%, got %v passed=%v", total, passed)
	}

	// ----- The sweeper ends attempts nobody polls ------------------------------
	sweptID, _ := publishExam(t, owner, org.ID, candID, obj{
		"schedule": obj{"schedule_type": "open_ended", "total_duration_minutes": 10, "min_time_before_submission_minutes": 0,
			"reconnection_grace_seconds": 60},
		"attempts": obj{"allow_resume_on_disconnect": false},
	}, sectionSpec{obj{"name": "S", "section_type": "objective"}, []obj{mcqQ("S1")}})
	silent := desktop(t, srv, "device-S-0000000000", "")
	silent.openSession(launchCode(t, cand, sweptID))
	sa := decode[runtimeState](t, silent.must(http.StatusOK, "POST", "/runtime/start", nil)).Attempt
	if _, err := pool.Exec(ctx, `UPDATE exam_attempts SET last_heartbeat_at = now() - interval '5 minutes' WHERE id = $1`, sa.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.Sweep(ctx); err != nil || n != 1 {
		t.Fatalf("sweep settled %d attempts, err %v", n, err)
	}
	if s := silent.attempt(http.StatusOK); s.Status != "evaluated" || *s.EndedReason != "disconnected" {
		t.Fatalf("expected a disconnect submission: %+v", s)
	}

	// ----- Releasing a device lets the candidate move ----------------------------
	movedID, _ := publishExam(t, owner, org.ID, candID, obj{
		"schedule":   obj{"schedule_type": "open_ended", "total_duration_minutes": 10, "min_time_before_submission_minutes": 0},
		"proctoring": obj{"ip_whitelist_cidr": []string{"10.9.0.0/16"}},
	}, sectionSpec{obj{"name": "M", "section_type": "objective"}, []obj{mcqQ("M1")}})

	blocked := desktop(t, srv, "device-X-0000000000", "")
	if r := blocked.openSession(launchCode(t, cand, movedID)); r.Status != http.StatusForbidden {
		t.Fatalf("the IP allowlist should block 127.0.0.1, got %d", r.Status)
	}
	old := desktop(t, srv, "device-O-0000000000", "10.9.1.1")
	old.openSession(launchCode(t, cand, movedID))
	ma := decode[runtimeState](t, old.must(http.StatusOK, "POST", "/runtime/start", nil)).Attempt
	old.headers["X-Forwarded-For"] = "10.9.2.2"
	if s := old.attempt(http.StatusOK); s.Violations["ip_changed"] != 1 {
		t.Fatalf("a network change should be logged: %+v", s.Violations)
	}

	staff := "/orgs/" + org.ID + "/exams/" + movedID + "/attempts/" + ma.ID + "/release-device"
	cand.must(http.StatusForbidden, "POST", staff, nil)
	proctor.must(http.StatusOK, "POST", staff, nil)
	old.must(http.StatusUnauthorized, "GET", "/runtime/state", nil)

	moved := desktop(t, srv, "device-N-0000000000", "10.9.3.3")
	moved.openSession(launchCode(t, cand, movedID))
	if s := moved.attempt(http.StatusOK); s.ID != ma.ID || s.Status != "in_progress" {
		t.Fatalf("the new device should continue the same attempt: %+v", s)
	}
	if r := old.openSession(launchCode(t, cand, movedID)); r.Status != http.StatusConflict {
		t.Fatalf("the old device is no longer bound, got %d", r.Status)
	}
}
