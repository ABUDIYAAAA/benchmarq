package attempts

import (
	"context"
	"errors"
	"fmt"
	"github.com/ABUDIYAAAA/benchmarq/internal/storage"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ABUDIYAAAA/benchmarq/internal/authz"
	"github.com/ABUDIYAAAA/benchmarq/internal/database"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/exams"
	"github.com/ABUDIYAAAA/benchmarq/internal/modules/questions"
	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/pagination"
	"github.com/ABUDIYAAAA/benchmarq/pkg/utils/token"
	"github.com/google/uuid"
)

const (
	launchCodeTTL = 2 * time.Minute
	sessionTTL    = 36 * time.Hour
)

var (
	errExamNotFound    = apperr.NotFound("Exam not found")
	errAttemptNotFound = apperr.NotFound("Attempt not found")
	errLaunchInvalid   = apperr.Unauthorized("This launch link is invalid or has expired; launch the exam again from the portal")
	errSessionInvalid  = apperr.Unauthorized("Your exam session is no longer valid; launch the exam again from the portal")
	errOtherDevice     = apperr.Conflict("This exam is already in progress on another device")
	errNetworkBlocked  = apperr.Forbidden("Your network is not permitted for this exam")
	errQuestionMoved   = apperr.Conflict("That question is no longer on screen; reload the current question")
)

// Options configures the attempt runtime.
type Options struct {
	// DesktopLaunchURL is the deep link that opens the desktop app; the
	// launch code is appended as ?code=.
	DesktopLaunchURL string
	// Snapshots stores proctoring images; nil disables snapshot uploads.
	Snapshots storage.SnapshotStore
}

type Service interface {
	// Web portal
	ListMyExams(ctx context.Context, userID uuid.UUID) ([]CandidateExam, error)
	Launch(ctx context.Context, userID, examID uuid.UUID) (*LaunchGrant, error)

	// Desktop app
	OpenSession(ctx context.Context, req OpenSessionRequest, c Client) (*SessionGrant, error)
	Authenticate(ctx context.Context, rawToken, deviceID string) (*Session, error)
	State(ctx context.Context, sess *Session, c Client) (*RuntimeState, error)
	Start(ctx context.Context, sess *Session, c Client) (*RuntimeState, error)
	Heartbeat(ctx context.Context, sess *Session, c Client) (*AttemptView, error)
	EnterSection(ctx context.Context, sess *Session, c Client, sectionID uuid.UUID) (*AttemptView, error)
	FinishSection(ctx context.Context, sess *Session, c Client, sectionID uuid.UUID) (*AttemptView, error)
	CurrentQuestion(ctx context.Context, sess *Session, c Client) (*QuestionView, error)
	Navigate(ctx context.Context, sess *Session, c Client, position int) (*QuestionView, error)
	SaveResponse(ctx context.Context, sess *Session, c Client, req SaveResponseRequest) (*ResponseView, error)
	Pause(ctx context.Context, sess *Session, c Client) (*AttemptView, error)
	Resume(ctx context.Context, sess *Session, c Client) (*AttemptView, error)
	ReportEvent(ctx context.Context, sess *Session, c Client, req EventRequest) (*EventResult, error)
	SaveSnapshots(ctx context.Context, sess *Session, c Client, shots []SnapshotUpload) (*SnapshotResult, error)
	Submit(ctx context.Context, sess *Session, c Client) (*AttemptView, error)

	// Staff
	ListAttempts(ctx context.Context, m authz.Membership, examID uuid.UUID, p pagination.Params) (pagination.Page[AttemptSummary], error)
	ReleaseDevice(ctx context.Context, m authz.Membership, examID, attemptID uuid.UUID) error

	// Sweep settles running attempts that need the server to act on its own.
	Sweep(ctx context.Context) (int, error)
}

// Client identifies where a request came from.
type Client struct {
	IP        string
	UserAgent string
}

type service struct {
	repo      Repository
	exams     exams.Repository
	questions questions.Repository
	tx        database.Transactor
	logger    *slog.Logger
	opts      Options
	now       func() time.Time
	snapshots snapshotThrottle
}

func NewService(repo Repository, examRepo exams.Repository, questionRepo questions.Repository, tx database.Transactor, logger *slog.Logger, opts Options) Service {
	if opts.DesktopLaunchURL == "" {
		opts.DesktopLaunchURL = "benchmarq://launch"
	}
	return &service{
		repo: repo, exams: examRepo, questions: questionRepo, tx: tx, logger: logger, opts: opts,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// ---------------------------------------------------------------------------
// Blueprint: the authored exam, loaded per request
// ---------------------------------------------------------------------------

type blueprint struct {
	cand     *candidateExam
	cfg      *exams.ExamConfig
	sections []exams.Section // authored order
	infos    map[uuid.UUID]sectionInfo
}

func (s *service) loadBlueprint(ctx context.Context, cand *candidateExam) (*blueprint, error) {
	cfg, err := s.exams.GetConfig(ctx, cand.ExamID)
	if err != nil {
		return nil, err
	}
	sections, err := s.exams.ListSections(ctx, cand.ExamID)
	if err != nil {
		return nil, err
	}
	rules := exams.ResolveRules(cfg, sections)
	infos := make(map[uuid.UUID]sectionInfo, len(sections))
	for _, sec := range sections {
		infos[sec.ID] = sectionInfo{Section: sec, Rules: rules[sec.ID]}
	}
	return &blueprint{cand: cand, cfg: cfg, sections: sections, infos: infos}, nil
}

func (b *blueprint) candidacy() *candidacy {
	c := b.cand
	return &candidacy{
		ExamStatus: c.Status, Config: b.cfg, StartOverride: c.StartOverride, EndOverride: c.EndOverride,
		TimeMultiplier: c.Multiplier, AttemptsUsed: c.AttemptsUsed, LastEndedAt: c.LastEndedAt, InProgressID: c.InProgressID,
	}
}

// ---------------------------------------------------------------------------
// Web portal
// ---------------------------------------------------------------------------

func (s *service) ListMyExams(ctx context.Context, userID uuid.UUID) ([]CandidateExam, error) {
	cands, err := s.repo.ListCandidateExams(ctx, userID)
	if err != nil {
		return nil, s.mapErr(err, "list candidate exams")
	}
	now := s.now()
	out := make([]CandidateExam, 0, len(cands))
	for i := range cands {
		c := &cands[i]
		cfg, err := s.exams.GetConfig(ctx, c.ExamID)
		if err != nil {
			return nil, s.mapErr(err, "list candidate exams")
		}
		b := &blueprint{cand: c, cfg: cfg}
		out = append(out, CandidateExam{
			ExamID: c.ExamID, OrgID: c.OrgID, OrgName: c.OrgName, Title: c.Title, Description: c.Description,
			Availability: availability(b.candidacy(), now),
		})
	}
	return out, nil
}

// Launch issues a single-use code for the desktop app.
func (s *service) Launch(ctx context.Context, userID, examID uuid.UUID) (*LaunchGrant, error) {
	cand, err := s.repo.GetCandidateExam(ctx, userID, examID)
	if err != nil {
		return nil, s.mapErr(err, "launch exam")
	}
	cfg, err := s.exams.GetConfig(ctx, examID)
	if err != nil {
		return nil, s.mapErr(err, "launch exam")
	}
	b := &blueprint{cand: cand, cfg: cfg}
	if av := availability(b.candidacy(), s.now()); !av.CanLaunch {
		return nil, apperr.Conflict(unavailableMessage(av))
	}

	code, codeHash, err := token.GenerateOpaqueToken()
	if err != nil {
		return nil, apperr.Internal(err, "launch exam")
	}
	expires := s.now().Add(launchCodeTTL)
	if err := s.repo.CreateLaunchCode(ctx, userID, examID, codeHash, expires); err != nil {
		return nil, s.mapErr(err, "launch exam")
	}
	return &LaunchGrant{Code: code, LaunchURL: s.opts.DesktopLaunchURL + "?code=" + code, ExpiresAt: expires}, nil
}

func unavailableMessage(av Availability) string {
	switch av.Reason {
	case "not_open_yet":
		return "This exam has not opened yet"
	case "late_join_closed":
		return "The time to join this exam has passed"
	case "window_closed":
		return "This exam's window has closed"
	case "no_attempts_left":
		return "You have used all of your attempts"
	case "cooldown":
		return "You must wait before attempting this exam again"
	case "not_published", "not_scheduled":
		return "This exam is not available"
	default:
		return "This exam cannot be started right now"
	}
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

// OpenSession redeems a launch code on a device. A running attempt can only
// be continued from the device it is bound to.
func (s *service) OpenSession(ctx context.Context, req OpenSessionRequest, c Client) (*SessionGrant, error) {
	raw, tokenHash, err := token.GenerateOpaqueToken()
	if err != nil {
		return nil, apperr.Internal(err, "open session")
	}

	var sess *Session
	var singleSession bool
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		userID, examID, err := s.repo.ConsumeLaunchCode(ctx, token.HashToken(req.LaunchCode))
		if err != nil {
			return err
		}
		cand, err := s.repo.GetCandidateExam(ctx, userID, examID)
		if err != nil {
			return err
		}
		cfg, err := s.exams.GetConfig(ctx, examID)
		if err != nil {
			return err
		}
		if !ipAllowed(cfg, c.IP) {
			return errNetworkBlocked
		}
		singleSession = cfg.Proctoring.SingleSessionLock

		sess = &Session{UserID: userID, ExamID: examID, DeviceID: req.DeviceID, ExpiresAt: s.now().Add(sessionTTL)}
		if cand.InProgressID != nil {
			a, err := s.repo.LockAttempt(ctx, *cand.InProgressID, false)
			if err != nil {
				return err
			}
			switch {
			case a.DeviceID == nil:
				// A proctor released the binding; this device takes over.
				a.DeviceID = &req.DeviceID
				if err := s.repo.SaveAttempt(ctx, a); err != nil {
					return err
				}
			case *a.DeviceID != req.DeviceID:
				return errOtherDevice
			}
			sess.AttemptID = &a.ID
		} else {
			b := &blueprint{cand: cand, cfg: cfg}
			if av := availability(b.candidacy(), s.now()); !av.CanLaunch {
				return apperr.Conflict(unavailableMessage(av))
			}
		}

		if err := s.repo.CreateSession(ctx, sess, tokenHash, c.IP, c.UserAgent); err != nil {
			return err
		}
		if singleSession {
			return s.repo.RevokeSessions(ctx, userID, examID, sess.ID)
		}
		return nil
	})
	if err != nil {
		return nil, s.mapErr(err, "open session")
	}

	state, err := s.State(ctx, sess, c)
	if err != nil {
		return nil, err
	}
	return &SessionGrant{Token: raw, ExpiresAt: sess.ExpiresAt, State: state}, nil
}

func (s *service) Authenticate(ctx context.Context, rawToken, deviceID string) (*Session, error) {
	if rawToken == "" || deviceID == "" {
		return nil, errSessionInvalid
	}
	sess, err := s.repo.GetSession(ctx, token.HashToken(rawToken))
	if err != nil {
		return nil, s.mapErr(err, "authenticate session")
	}
	if sess.DeviceID != deviceID {
		return nil, errSessionInvalid
	}
	return sess, nil
}

// ---------------------------------------------------------------------------
// Starting
// ---------------------------------------------------------------------------

func (s *service) State(ctx context.Context, sess *Session, c Client) (*RuntimeState, error) {
	if sess.AttemptID == nil {
		return s.preStartState(ctx, sess)
	}
	var state *RuntimeState
	err := s.withRuntime(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error {
		var err error
		state, err = s.runtimeState(ctx, rt, now)
		return err
	})
	return state, err
}

func (s *service) preStartState(ctx context.Context, sess *Session) (*RuntimeState, error) {
	cand, err := s.repo.GetCandidateExam(ctx, sess.UserID, sess.ExamID)
	if err != nil {
		return nil, s.mapErr(err, "load exam")
	}
	b, err := s.loadBlueprint(ctx, cand)
	if err != nil {
		return nil, s.mapErr(err, "load exam")
	}
	now := s.now()
	av := availability(b.candidacy(), now)
	brief, err := s.examBrief(ctx, b)
	if err != nil {
		return nil, s.mapErr(err, "load exam")
	}
	return &RuntimeState{ServerTime: now, Exam: *brief, Rules: rulesFrom(b.cfg), Availability: &av}, nil
}

func (s *service) examBrief(ctx context.Context, b *blueprint) (*ExamBrief, error) {
	stats, err := s.exams.SectionStats(ctx, b.cand.ExamID)
	if err != nil {
		return nil, err
	}
	cd := b.candidacy()
	brief := &ExamBrief{
		ID: b.cand.ExamID, Title: b.cand.Title, Description: b.cand.Description, Instructions: b.cand.Instructions,
		DurationMinutes: cd.limitSeconds() / 60, Sections: make([]SectionBrief, 0, len(b.sections)),
	}
	for _, sec := range b.sections {
		r := b.infos[sec.ID].Rules
		n := stats[sec.ID].QuestionCount
		if r.QuestionPickCount != nil {
			n = min(n, *r.QuestionPickCount)
		}
		sb := SectionBrief{Name: sec.Name, Type: sec.SectionType, QuestionCount: n}
		if r.DurationMinutes != nil {
			sb.TimeLimitSeconds = ptrTo(cd.scaled(*r.DurationMinutes))
		}
		brief.Sections = append(brief.Sections, sb)
	}
	return brief, nil
}

// Start begins the attempt on the session's device: the paper is drawn, the
// clock is fixed and, in linear modes, the first section opens.
func (s *service) Start(ctx context.Context, sess *Session, c Client) (*RuntimeState, error) {
	if sess.AttemptID != nil {
		return nil, apperr.Conflict("The exam has already started")
	}
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		cand, err := s.repo.GetCandidateExam(ctx, sess.UserID, sess.ExamID)
		if err != nil {
			return err
		}
		if cand.InProgressID != nil {
			return errOtherDevice
		}
		b, err := s.loadBlueprint(ctx, cand)
		if err != nil {
			return err
		}
		if !ipAllowed(b.cfg, c.IP) {
			return errNetworkBlocked
		}
		now := s.now()
		cd := b.candidacy()
		if av := availability(cd, now); !av.CanStart {
			return apperr.Conflict(unavailableMessage(av))
		}

		src := make([]paperSection, 0, len(b.sections))
		for _, sec := range b.sections {
			qs, err := s.questions.List(ctx, sec.ID)
			if err != nil {
				return err
			}
			r := b.infos[sec.ID].Rules
			ps := paperSection{ID: sec.ID, Rules: r, Questions: make([]paperSource, len(qs))}
			if r.DurationMinutes != nil {
				ps.TimeLimitSeconds = ptrTo(cd.scaled(*r.DurationMinutes))
			}
			for i, q := range qs {
				src := paperSource{ID: q.ID, Points: q.Points}
				for _, o := range q.Options {
					src.Options = append(src.Options, optionSource{ID: o.ID, Pinned: o.IsPinned})
				}
				ps.Questions[i] = src
			}
			src = append(src, ps)
		}

		seed := attemptSeed(b.cfg.Randomization.DeterministicUserSeed, cand.ExamID, sess.UserID)
		states, paper := buildPaper(seed, src, b.cfg.Randomization.ShuffleSections)
		plan := planTimer(cd, now)
		a := &Attempt{
			ID: uuid.New(), ExamID: cand.ExamID, UserID: sess.UserID, AssignmentID: &cand.AssignmentID,
			Status: StatusInProgress, DeviceID: &sess.DeviceID, Seed: seed, StartedAt: now,
			TimerOriginAt: plan.Origin, LimitSeconds: plan.LimitSeconds, WindowEndAt: plan.WindowEnd,
			TimeMultiplier: cand.Multiplier, LastHeartbeatAt: &now, ClientIP: nonEmpty(c.IP),
		}
		if computeClock(a, 0, now).expired() {
			return apperr.Conflict("There is no time left to take this exam")
		}
		eng := newEngine(b.cfg, a, b.infos, states, paper, nil, nil)
		a.DeadlineAt = ptrTo(latestDeadline(a, eng.pauseBudget()))
		if err := s.repo.CreateAttempt(ctx, a, states, paper, c.UserAgent); err != nil {
			return err
		}
		if err := s.repo.BindSession(ctx, sess.ID, a.ID); err != nil {
			return err
		}
		sess.AttemptID = &a.ID

		if eng.linear() && len(eng.states) > 0 {
			if eng.access(eng.states[0], eng.clockAt(now)).Open {
				eng.open(eng.states[0], now)
				if err := s.repo.SaveSectionStates(ctx, a.ID, eng.states); err != nil {
					return err
				}
				return s.repo.SaveAttempt(ctx, a)
			}
		}
		return nil
	})
	if err != nil {
		sess.AttemptID = nil
		return nil, s.mapErr(err, "start attempt")
	}
	return s.State(ctx, sess, c)
}

// ---------------------------------------------------------------------------
// Runtime
// ---------------------------------------------------------------------------

// runtime is one attempt loaded for a request.
type runtime struct {
	bp        *blueprint
	eng       *engine
	questions map[uuid.UUID][]questions.Question // by section, loaded on demand
}

func (rt *runtime) attempt() *Attempt { return rt.eng.attempt }

// sectionQuestions loads a section's questions, with answer keys, once per request.
func (s *service) sectionQuestions(ctx context.Context, rt *runtime, sectionID uuid.UUID) ([]questions.Question, error) {
	if qs, ok := rt.questions[sectionID]; ok {
		return qs, nil
	}
	qs, err := s.questions.List(ctx, sectionID)
	if err != nil {
		return nil, err
	}
	rt.questions[sectionID] = qs
	return qs, nil
}

func (s *service) question(ctx context.Context, rt *runtime, pq *PaperQuestion) (*questions.Question, error) {
	qs, err := s.sectionQuestions(ctx, rt, pq.SectionID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(qs, func(q questions.Question) bool { return q.ID == pq.QuestionID })
	if i < 0 {
		return nil, fmt.Errorf("question %s missing from section %s", pq.QuestionID, pq.SectionID)
	}
	return &qs[i], nil
}

// load reads an attempt and everything its rules depend on.
func (s *service) load(ctx context.Context, a *Attempt) (*runtime, error) {
	cand, err := s.repo.GetCandidateExam(ctx, a.UserID, a.ExamID)
	if errors.Is(err, ErrExamNotFound) {
		// The exam, assignment or membership is gone: the attempt still
		// loads so it can be closed.
		cand = &candidateExam{ExamID: a.ExamID, Status: exams.StatusArchived, Multiplier: a.TimeMultiplier}
	} else if err != nil {
		return nil, err
	}
	b, err := s.loadBlueprint(ctx, cand)
	if err != nil {
		return nil, err
	}
	states, err := s.repo.LoadSectionStates(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	paper, err := s.repo.LoadPaper(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	responses, err := s.repo.LoadResponses(ctx, a.ID)
	if err != nil {
		return nil, err
	}

	rt := &runtime{bp: b, questions: map[uuid.UUID][]questions.Question{}}
	rt.eng = newEngine(b.cfg, a, b.infos, states, paper, responses, func(sectionID uuid.UUID) (*float64, float64, error) {
		return s.scoreSection(ctx, rt, sectionID)
	})
	rt.eng.onEnd = func() error { return s.finalize(ctx, rt) }
	return rt, nil
}

// scoreSection grades the questions served in one section. Subjective
// sections report their maximum but no score.
func (s *service) scoreSection(ctx context.Context, rt *runtime, sectionID uuid.UUID) (*float64, float64, error) {
	served := rt.eng.paper[sectionID]
	maxScore := 0.0
	for _, pq := range served {
		maxScore += pq.Points
	}
	if !rt.bp.infos[sectionID].Section.SectionType.AutoGradable() {
		return nil, round2(maxScore), nil
	}
	policy := policyFrom(rt.bp.cfg)
	total := 0.0
	for _, pq := range served {
		q, err := s.question(ctx, rt, pq)
		if err != nil {
			return nil, 0, err
		}
		total += grade(q, pq.Points, rt.eng.responses[pq.QuestionID], policy).Score
	}
	total = round2(total)
	return &total, round2(maxScore), nil
}

// withRuntime runs fn against the session's attempt under its row lock,
// after applying the passage of time, and persists the result. If fn fails
// nothing is saved; time-driven changes are recomputed identically next time.
// fn must do all I/O through the ctx it is given, which carries the
// transaction: a query on another connection would wait on the row lock.
func (s *service) withRuntime(ctx context.Context, sess *Session, c Client, fn func(ctx context.Context, rt *runtime, now time.Time) error) error {
	if sess.AttemptID == nil {
		return apperr.Conflict("The exam has not started yet")
	}
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		a, err := s.repo.LockAttempt(ctx, *sess.AttemptID, false)
		if err != nil {
			return err
		}
		if a.UserID != sess.UserID {
			return ErrAttemptNotFound
		}
		if a.running() && (a.DeviceID == nil || *a.DeviceID != sess.DeviceID) {
			return errSessionInvalid
		}
		rt, err := s.load(ctx, a)
		if err != nil {
			return err
		}
		now := s.now()
		if err := s.settle(ctx, rt, now); err != nil {
			return err
		}
		if a.running() {
			if err := s.guardNetwork(ctx, rt, c, now); err != nil {
				return err
			}
			rt.eng.touch(now)
		}
		if err := fn(ctx, rt, now); err != nil {
			return err
		}
		return s.persist(ctx, rt)
	})
	return s.mapErr(err, "attempt")
}

// settle applies time-driven transitions and closes attempts whose exam is
// no longer live.
func (s *service) settle(ctx context.Context, rt *runtime, now time.Time) error {
	if err := rt.eng.reconcile(now); err != nil {
		return err
	}
	if rt.attempt().running() && rt.bp.cand.Status != exams.StatusPublished {
		return rt.eng.end(now, EndExamClosed, StatusAutoSubmitted)
	}
	return nil
}

// guardNetwork enforces the IP allowlist and records network changes.
func (s *service) guardNetwork(ctx context.Context, rt *runtime, c Client, now time.Time) error {
	if !ipAllowed(rt.bp.cfg, c.IP) {
		return errNetworkBlocked
	}
	a := rt.attempt()
	if c.IP == "" || (a.ClientIP != nil && *a.ClientIP == c.IP) {
		return nil
	}
	if a.ClientIP != nil {
		err := s.repo.AddViolation(ctx, a.ID, violationRecord{
			Type: "ip_changed", Counted: true,
			Metadata: map[string]any{"from": *a.ClientIP, "to": c.IP, "at": now},
		})
		if err != nil {
			return err
		}
	}
	a.ClientIP = &c.IP
	return nil
}

func (s *service) persist(ctx context.Context, rt *runtime) error {
	a := rt.attempt()
	if a.running() {
		a.DeadlineAt = ptrTo(latestDeadline(a, rt.eng.pauseBudget()))
	}
	if err := s.repo.SaveSectionStates(ctx, a.ID, rt.eng.states); err != nil {
		return err
	}
	return s.repo.SaveAttempt(ctx, a)
}

// finalize grades an attempt the moment it ends. Attempts with subjective
// questions keep a provisional objective score until graded by hand.
func (s *service) finalize(ctx context.Context, rt *runtime) error {
	a := rt.attempt()
	maxScore, total := 0.0, 0.0
	manual := false
	policy := policyFrom(rt.bp.cfg)
	var grades []questionGrade

	for _, st := range rt.eng.states {
		for _, pq := range rt.eng.paper[st.SectionID] {
			maxScore += pq.Points
			if a.Status == StatusAbandoned {
				continue
			}
			q, err := s.question(ctx, rt, pq)
			if err != nil {
				return err
			}
			resp := rt.eng.responses[pq.QuestionID]
			g := grade(q, pq.Points, resp, policy)
			if !g.Auto {
				manual = manual || resp.answered()
				continue
			}
			total += g.Score
			if resp != nil {
				grades = append(grades, questionGrade{QuestionID: pq.QuestionID, Score: g.Score, Correct: g.Correct})
			}
		}
	}

	a.MaxScore = ptrTo(round2(maxScore))
	if a.Status == StatusAbandoned {
		return nil
	}
	if err := s.repo.SaveGrades(ctx, a.ID, grades); err != nil {
		return err
	}
	a.TotalScore = ptrTo(round2(total))
	if !manual {
		threshold := rt.bp.cfg.Scoring.PassingPercentage / 100 * maxScore
		if rt.bp.cand.PassingMarks != nil {
			threshold = *rt.bp.cand.PassingMarks
		}
		a.Status = StatusEvaluated
		a.Passed = ptrTo(total >= threshold)
	}
	return nil
}

func (s *service) Heartbeat(ctx context.Context, sess *Session, c Client) (*AttemptView, error) {
	return s.attemptAction(ctx, sess, c, func(context.Context, *runtime, time.Time) error { return nil })
}

func (s *service) EnterSection(ctx context.Context, sess *Session, c Client, sectionID uuid.UUID) (*AttemptView, error) {
	return s.attemptAction(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error {
		return rt.eng.enterSection(sectionID, now)
	})
}

func (s *service) FinishSection(ctx context.Context, sess *Session, c Client, sectionID uuid.UUID) (*AttemptView, error) {
	return s.attemptAction(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error {
		return rt.eng.finishSection(sectionID, now)
	})
}

func (s *service) Pause(ctx context.Context, sess *Session, c Client) (*AttemptView, error) {
	return s.attemptAction(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error { return rt.eng.pause(now) })
}

func (s *service) Resume(ctx context.Context, sess *Session, c Client) (*AttemptView, error) {
	return s.attemptAction(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error { return rt.eng.resume(now) })
}

// Submit ends the attempt at the candidate's request.
func (s *service) Submit(ctx context.Context, sess *Session, c Client) (*AttemptView, error) {
	return s.attemptAction(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error {
		if err := rt.eng.requireActive(now); err != nil {
			return err
		}
		wait := time.Duration(rt.bp.cfg.Schedule.MinTimeBeforeSubmissionMinutes)*time.Minute - rt.eng.clockAt(now).activeElapsed(rt.attempt())
		if wait > 0 {
			return apperr.Conflict(fmt.Sprintf("You can submit in %d minute(s)", int((wait+time.Minute-time.Second)/time.Minute)))
		}
		return rt.eng.end(now, EndSubmitted, StatusSubmitted)
	})
}

// attemptAction runs an action and returns the resulting attempt view.
func (s *service) attemptAction(ctx context.Context, sess *Session, c Client, fn func(ctx context.Context, rt *runtime, now time.Time) error) (*AttemptView, error) {
	var view *AttemptView
	err := s.withRuntime(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error {
		if err := fn(ctx, rt, now); err != nil {
			return err
		}
		var err error
		view, err = s.attemptView(ctx, rt, now)
		return err
	})
	return view, err
}

// CurrentQuestion returns the question under the cursor of the open section.
func (s *service) CurrentQuestion(ctx context.Context, sess *Session, c Client) (*QuestionView, error) {
	var view *QuestionView
	err := s.withRuntime(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error {
		var err error
		view, err = s.showCurrent(ctx, rt, now)
		return err
	})
	return view, err
}

func (s *service) Navigate(ctx context.Context, sess *Session, c Client, position int) (*QuestionView, error) {
	var view *QuestionView
	err := s.withRuntime(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error {
		complete := true
		if rt.bp.cfg.Navigation.QuestionNavigationMode == exams.NavigationMandatoryAnswer {
			pq, err := rt.eng.currentQuestion(now)
			if err != nil {
				return err
			}
			q, err := s.question(ctx, rt, pq)
			if err != nil {
				return err
			}
			complete = answerComplete(q, rt.eng.responses[pq.QuestionID])
		}
		if err := rt.eng.navigate(position, complete, now); err != nil {
			return err
		}
		var err error
		view, err = s.showCurrent(ctx, rt, now)
		return err
	})
	return view, err
}

// showCurrent renders the question under the cursor and records that the
// candidate has seen it.
func (s *service) showCurrent(ctx context.Context, rt *runtime, now time.Time) (*QuestionView, error) {
	pq, err := rt.eng.currentQuestion(now)
	if err != nil {
		return nil, err
	}
	q, err := s.question(ctx, rt, pq)
	if err != nil {
		return nil, err
	}
	if pq.FirstViewedAt == nil {
		if err := s.repo.MarkViewed(ctx, rt.attempt().ID, pq.QuestionID, now); err != nil {
			return nil, err
		}
		pq.FirstViewedAt = &now
	}
	view := s.questionView(rt, pq, q, now)
	return view, nil
}

// SaveResponse answers the question under the cursor, and only that one.
func (s *service) SaveResponse(ctx context.Context, sess *Session, c Client, req SaveResponseRequest) (*ResponseView, error) {
	var view *ResponseView
	err := s.withRuntime(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error {
		pq, err := rt.eng.currentQuestion(now)
		if err != nil {
			return err
		}
		if pq.QuestionID != req.QuestionID {
			return errQuestionMoved
		}
		q, err := s.question(ctx, rt, pq)
		if err != nil {
			return err
		}
		resp, err := buildResponse(rt.bp.cfg, q, pq, rt.eng.responses[pq.QuestionID], req)
		if err != nil {
			return err
		}
		if err := s.repo.SaveResponse(ctx, rt.attempt().ID, resp); err != nil {
			return err
		}
		rt.eng.responses[pq.QuestionID] = resp
		view = responseView(resp)
		return nil
	})
	return view, err
}

// ReportEvent records a proctoring event from the desktop app and ends the
// attempt when a configured limit is exceeded.
func (s *service) ReportEvent(ctx context.Context, sess *Session, c Client, req EventRequest) (*EventResult, error) {
	var result *EventResult
	err := s.withRuntime(ctx, sess, c, func(ctx context.Context, rt *runtime, now time.Time) error {
		a := rt.attempt()
		if !a.running() {
			return errAttemptEnded
		}
		pr := rt.bp.cfg.Proctoring
		counted := true
		limit := 0
		switch req.Type {
		case "tab_switch":
			counted = pr.TrackTabFocus && (req.DurationMs == nil || *req.DurationMs >= pr.TabSwitchGraceSeconds*1000)
			limit = pr.MaxTabSwitchCount
		case "fullscreen_exit":
			counted = pr.EnforceFullscreen
			limit = pr.MaxFullscreenExitCount
		}

		meta := map[string]any{"received_at": now}
		if req.DurationMs != nil {
			meta["duration_ms"] = *req.DurationMs
		}
		if req.ClientTime != nil {
			meta["client_time"] = *req.ClientTime
		}
		if len(req.Details) > 0 {
			meta["details"] = req.Details
		}
		if err := s.repo.AddViolation(ctx, a.ID, violationRecord{Type: req.Type, Counted: counted, Metadata: meta}); err != nil {
			return err
		}
		counts, err := s.repo.CountViolations(ctx, a.ID)
		if err != nil {
			return err
		}

		result = &EventResult{Counted: counted, Violations: counts}
		// A limit of 0 means the event is logged but never ends the attempt.
		if counted && limit > 0 && counts[req.Type] > limit {
			if err := rt.eng.end(now, EndViolationLimit, StatusAutoSubmitted); err != nil {
				return err
			}
			result.Ended = true
		}
		return nil
	})
	return result, err
}

// ---------------------------------------------------------------------------
// Staff and background
// ---------------------------------------------------------------------------

func (s *service) ListAttempts(ctx context.Context, m authz.Membership, examID uuid.UUID, p pagination.Params) (pagination.Page[AttemptSummary], error) {
	if ok, err := s.repo.ExamInOrg(ctx, m.OrgID, examID); err != nil || !ok {
		if err == nil {
			err = ErrExamNotFound
		}
		return pagination.Page[AttemptSummary]{}, s.mapErr(err, "list attempts")
	}
	rows, err := s.repo.ListAttempts(ctx, examID, p)
	if err != nil {
		return pagination.Page[AttemptSummary]{}, s.mapErr(err, "list attempts")
	}
	return pagination.NewPage(rows, p.Limit, func(a AttemptSummary) pagination.Cursor {
		return pagination.Cursor{Time: a.StartedAt, ID: a.ID}
	}), nil
}

// ReleaseDevice unbinds a running attempt from its device, so the candidate
// can continue on another one. Every session for the attempt is revoked.
func (s *service) ReleaseDevice(ctx context.Context, m authz.Membership, examID, attemptID uuid.UUID) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		ok, err := s.repo.ExamInOrg(ctx, m.OrgID, examID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrExamNotFound
		}
		if _, err := s.repo.LockAttempt(ctx, attemptID, false); err != nil {
			return err
		}
		return s.repo.ReleaseDevice(ctx, examID, attemptID)
	})
	if err == nil {
		s.logger.Info("attempts: device released", "attempt_id", attemptID, "by", m.UserID)
	}
	return s.mapErr(err, "release device")
}

// Sweep settles attempts nobody is polling: expired ones, silent ones that
// may not resume, and ones whose exam was taken down. Attempts locked by a
// live request are skipped; that request settles them.
func (s *service) Sweep(ctx context.Context) (int, error) {
	ids, err := s.repo.DueAttempts(ctx, 200)
	if err != nil {
		return 0, err
	}
	settled := 0
	for _, id := range ids {
		err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			a, err := s.repo.LockAttempt(ctx, id, true)
			if errors.Is(err, ErrAttemptNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			rt, err := s.load(ctx, a)
			if err != nil {
				return err
			}
			if err := s.settle(ctx, rt, s.now()); err != nil {
				return err
			}
			if !a.running() {
				settled++
			}
			return s.persist(ctx, rt)
		})
		if err != nil {
			s.logger.Error("attempts: sweep failed", "attempt_id", id, "error", err)
		}
	}
	return settled, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// ipAllowed applies the exam's IP allowlist. An empty list allows all.
func ipAllowed(cfg *exams.ExamConfig, ip string) bool {
	list := cfg.Proctoring.IPWhitelistCIDR
	if len(list) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, entry := range list {
		if p, err := netip.ParsePrefix(entry); err == nil && p.Contains(addr) {
			return true
		}
		if a, err := netip.ParseAddr(entry); err == nil && a.Unmap() == addr {
			return true
		}
	}
	return false
}

// answerComplete reports whether a response satisfies mandatory_answer
// navigation. Multi-select answers must meet the minimum number of choices.
func answerComplete(q *questions.Question, r *Response) bool {
	if !r.answered() {
		return false
	}
	if q.Type == questions.TypeMultiSelect && q.MultiSelect != nil {
		return len(r.SelectedOptionIDs) >= q.MultiSelect.MinChoicesRequired
	}
	if r.TextResponse != nil {
		return strings.TrimSpace(*r.TextResponse) != ""
	}
	return true
}

// buildResponse validates an answer against the question and returns the
// response to store. The existing response supplies the review flag when
// the request does not change it.
func buildResponse(cfg *exams.ExamConfig, q *questions.Question, pq *PaperQuestion, prev *Response, req SaveResponseRequest) (*Response, error) {
	out := &Response{QuestionID: q.ID}
	if prev != nil {
		out.Flagged = prev.Flagged
	}
	if req.Flagged != nil {
		if *req.Flagged && !cfg.Navigation.AllowBookmarkForReview {
			return nil, apperr.Forbidden("Marking questions for review is not allowed in this exam")
		}
		out.Flagged = *req.Flagged
	}

	given := 0
	for _, set := range []bool{req.SelectedOptionID != nil, req.SelectedOptionIDs != nil, req.Text != nil, req.Boolean != nil} {
		if set {
			given++
		}
	}
	invalid := func(field, msg string) error { return apperr.Validation(map[string]string{field: msg}) }
	switch {
	case req.Clear && given > 0:
		return nil, invalid("clear", "clear cannot be combined with an answer")
	case req.Clear:
		return out, nil
	case given == 0:
		if req.Flagged == nil {
			return nil, invalid("body", "send an answer, clear or flagged")
		}
		if prev != nil {
			out.SelectedOptionID, out.SelectedOptionIDs = prev.SelectedOptionID, prev.SelectedOptionIDs
			out.TextResponse, out.BooleanResponse = prev.TextResponse, prev.BooleanResponse
		}
		return out, nil
	case given > 1:
		return nil, invalid("body", "send exactly one answer field")
	}

	field := map[questions.QuestionType]string{
		questions.TypeMultipleChoice: "selected_option_id",
		questions.TypeMultiSelect:    "selected_option_ids",
		questions.TypeTrueFalse:      "boolean",
		questions.TypeBlankFill:      "text",
		questions.TypeShortAnswer:    "text",
		questions.TypeEssay:          "text",
	}[q.Type]
	wrongField := func() error { return invalid("body", fmt.Sprintf("%s questions are answered with %s", q.Type, field)) }

	switch q.Type {
	case questions.TypeMultipleChoice:
		if req.SelectedOptionID == nil {
			return nil, wrongField()
		}
		if !slices.Contains(pq.OptionOrder, *req.SelectedOptionID) {
			return nil, invalid("selected_option_id", "not an option of this question")
		}
		out.SelectedOptionID = req.SelectedOptionID

	case questions.TypeMultiSelect:
		if req.SelectedOptionIDs == nil {
			return nil, wrongField()
		}
		for _, id := range req.SelectedOptionIDs {
			if !slices.Contains(pq.OptionOrder, id) {
				return nil, invalid("selected_option_ids", "contains an option that is not part of this question")
			}
		}
		if ms := q.MultiSelect; ms != nil && ms.MaxChoicesAllowed != nil && len(req.SelectedOptionIDs) > *ms.MaxChoicesAllowed {
			return nil, invalid("selected_option_ids", fmt.Sprintf("select at most %d options", *ms.MaxChoicesAllowed))
		}
		if len(req.SelectedOptionIDs) > 0 {
			out.SelectedOptionIDs = req.SelectedOptionIDs
		}

	case questions.TypeTrueFalse:
		if req.Boolean == nil {
			return nil, wrongField()
		}
		out.BooleanResponse = req.Boolean

	case questions.TypeBlankFill, questions.TypeShortAnswer, questions.TypeEssay:
		if req.Text == nil {
			return nil, wrongField()
		}
		text := *req.Text
		maxLen, maxWords := textLimits(q)
		if maxLen != nil && utf8.RuneCountInString(text) > *maxLen {
			return nil, invalid("text", fmt.Sprintf("answer cannot exceed %d characters", *maxLen))
		}
		if maxWords != nil && len(strings.Fields(text)) > *maxWords {
			return nil, invalid("text", fmt.Sprintf("answer cannot exceed %d words", *maxWords))
		}
		if strings.TrimSpace(text) != "" {
			out.TextResponse = &text
		}
	}
	return out, nil
}

// blankFillMaxLength bounds blank-fill answers, which have no authored limit.
const blankFillMaxLength = 1000

func textLimits(q *questions.Question) (maxLen, maxWords *int) {
	switch {
	case q.ShortAnswer != nil:
		return q.ShortAnswer.MaxLength, q.ShortAnswer.MaxWords
	case q.Essay != nil:
		return q.Essay.MaxLength, q.Essay.MaxWords
	case q.Type == questions.TypeBlankFill:
		return ptrTo(blankFillMaxLength), nil
	}
	return nil, nil
}

func (s *service) mapErr(err error, op string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrExamNotFound), errors.Is(err, exams.ErrExamNotFound):
		return errExamNotFound
	case errors.Is(err, ErrAttemptNotFound):
		return errAttemptNotFound
	case errors.Is(err, ErrAttemptRunning):
		return errOtherDevice
	case errors.Is(err, ErrSessionNotFound):
		return errSessionInvalid
	case errors.Is(err, ErrLaunchCodeUnused):
		return errLaunchInvalid
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	s.logger.Error("attempts: "+op+" failed", "error", err)
	return apperr.Internal(err, op)
}

func ptrTo[T any](v T) *T { return &v }

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
