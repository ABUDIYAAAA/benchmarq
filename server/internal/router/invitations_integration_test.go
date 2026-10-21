package router

import (
	"context"
	"net/http"
	"testing"
)

type invitationView struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

func TestEmailVerification(t *testing.T) {
	srv, pool, mail := setupServerWithMail(t)
	ctx := context.Background()

	alice, _ := signUp(t, srv, "verify@benchmarq.io")
	first := mail.token(t, "welcome", "verify@benchmarq.io")

	alice.must(http.StatusOK, "POST", "/auth/verify-email", obj{"token": first})
	alice.must(http.StatusBadRequest, "POST", "/auth/verify-email", obj{"token": first})
	alice.must(http.StatusBadRequest, "POST", "/auth/verify-email", obj{"token": "not-a-token"})
	me := decode[struct {
		IsEmailVerified bool `json:"is_email_verified"`
	}](t, alice.must(http.StatusOK, "GET", "/auth/me", nil))
	if !me.IsEmailVerified {
		t.Fatal("user should be verified")
	}
	alice.must(http.StatusConflict, "POST", "/auth/resend-verification", nil)

	// Resending is rate limited, and a resent link replaces the previous one.
	bob, bobID := signUp(t, srv, "resend@benchmarq.io")
	original := mail.token(t, "welcome", "resend@benchmarq.io")
	bob.must(http.StatusTooManyRequests, "POST", "/auth/resend-verification", nil)
	if _, err := pool.Exec(ctx, `UPDATE email_verification_tokens SET created_at = now() - interval '2 minutes' WHERE user_id = $1`, bobID); err != nil {
		t.Fatal(err)
	}
	bob.must(http.StatusOK, "POST", "/auth/resend-verification", nil)
	bob.must(http.StatusBadRequest, "POST", "/auth/verify-email", obj{"token": original})
	bob.must(http.StatusOK, "POST", "/auth/verify-email", obj{"token": mail.token(t, "verify", "resend@benchmarq.io")})

	// Expired links are rejected.
	carol, carolID := signUp(t, srv, "expired@benchmarq.io")
	if _, err := pool.Exec(ctx, `UPDATE email_verification_tokens SET expires_at = now() - interval '1 second' WHERE user_id = $1`, carolID); err != nil {
		t.Fatal(err)
	}
	carol.must(http.StatusBadRequest, "POST", "/auth/verify-email", obj{"token": mail.token(t, "welcome", "expired@benchmarq.io")})

	// Unauthenticated callers cannot request a resend.
	anon := &apiClient{t: t, base: srv.URL + "/api/v1", http: http.DefaultClient}
	anon.must(http.StatusUnauthorized, "POST", "/auth/resend-verification", nil)
}

func TestOrganizationInvitations(t *testing.T) {
	srv, pool, mail := setupServerWithMail(t)
	ctx := context.Background()

	owner, _ := signUp(t, srv, "boss@benchmarq.io")
	org := decode[struct {
		ID string `json:"id"`
	}](t, owner.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Invite Co"}))
	orgPath := "/orgs/" + org.ID
	invites := orgPath + "/invitations"

	// ----- Create ---------------------------------------------------------
	carolInv := decode[invitationView](t, owner.must(http.StatusCreated, "POST", invites, obj{"email": "Carol@Benchmarq.io", "role": "instructor"}))
	if carolInv.Email != "carol@benchmarq.io" || carolInv.Status != "pending" || carolInv.Role != "instructor" {
		t.Fatalf("unexpected invitation %+v", carolInv)
	}
	if mail.count("org_invite", "carol@benchmarq.io") != 1 {
		t.Fatal("invitation email not sent")
	}
	owner.must(http.StatusConflict, "POST", invites, obj{"email": "CAROL@benchmarq.io", "role": "admin"})
	owner.must(http.StatusConflict, "POST", invites, obj{"email": "boss@benchmarq.io", "role": "admin"}) // already a member
	owner.must(http.StatusUnprocessableEntity, "POST", invites, obj{"email": "not-an-email", "role": "admin"})
	owner.must(http.StatusUnprocessableEntity, "POST", invites, obj{"email": "x@benchmarq.io", "role": "superuser"})

	// ----- Claim on email verification ------------------------------------
	// Signing up with an invited address is not enough: the invitation is
	// only claimed once the address is verified.
	carol, _ := signUp(t, srv, "carol@benchmarq.io")
	carol.must(http.StatusNotFound, "GET", orgPath, nil)
	carol.must(http.StatusOK, "POST", "/auth/verify-email", obj{"token": mail.token(t, "welcome", "carol@benchmarq.io")})
	joined := decode[struct {
		MyRole string `json:"my_role"`
	}](t, carol.must(http.StatusOK, "GET", orgPath, nil))
	if joined.MyRole != "instructor" {
		t.Fatalf("carol should have joined as instructor, got %q", joined.MyRole)
	}

	// Instructors can see invitations but not manage them.
	carol.must(http.StatusOK, "GET", invites, nil)
	carol.must(http.StatusForbidden, "POST", invites, obj{"email": "x@benchmarq.io", "role": "candidate"})

	// ----- Accept by link, resend ------------------------------------------
	daveInv := decode[invitationView](t, owner.must(http.StatusCreated, "POST", invites, obj{"email": "dave@benchmarq.io", "role": "admin"}))
	staleToken := mail.token(t, "org_invite", "dave@benchmarq.io")
	owner.must(http.StatusOK, "POST", invites+"/"+daveInv.ID+"/resend", nil)
	freshToken := mail.token(t, "org_invite", "dave@benchmarq.io")
	if staleToken == freshToken {
		t.Fatal("resend should issue a new link")
	}

	dave, _ := signUp(t, srv, "dave@benchmarq.io")
	eve, _ := signUp(t, srv, "eve@benchmarq.io")
	eve.must(http.StatusForbidden, "POST", "/invitations/accept", obj{"token": freshToken}) // someone else's invitation
	dave.must(http.StatusNotFound, "POST", "/invitations/accept", obj{"token": staleToken})
	accepted := decode[struct {
		ID     string `json:"id"`
		MyRole string `json:"my_role"`
	}](t, dave.must(http.StatusOK, "POST", "/invitations/accept", obj{"token": freshToken}))
	if accepted.ID != org.ID || accepted.MyRole != "admin" {
		t.Fatalf("unexpected accept result %+v", accepted)
	}
	dave.must(http.StatusNotFound, "POST", "/invitations/accept", obj{"token": freshToken})
	owner.must(http.StatusConflict, "POST", invites+"/"+daveInv.ID+"/resend", nil)

	// Admins cannot invite at or above their own rank.
	dave.must(http.StatusForbidden, "POST", invites, obj{"email": "frank@benchmarq.io", "role": "admin"})
	frankInv := decode[invitationView](t, dave.must(http.StatusCreated, "POST", invites, obj{"email": "frank@benchmarq.io", "role": "proctor"}))

	// ----- Revoke ----------------------------------------------------------
	dave.must(http.StatusOK, "DELETE", invites+"/"+frankInv.ID, nil)
	dave.must(http.StatusConflict, "DELETE", invites+"/"+frankInv.ID, nil)
	frank, _ := signUp(t, srv, "frank@benchmarq.io")
	frank.must(http.StatusNotFound, "POST", "/invitations/accept", obj{"token": mail.token(t, "org_invite", "frank@benchmarq.io")})
	frank.must(http.StatusOK, "POST", "/auth/verify-email", obj{"token": mail.token(t, "welcome", "frank@benchmarq.io")})
	frank.must(http.StatusNotFound, "GET", orgPath, nil) // revoked invitations are not claimed either

	// ----- Expiry ------------------------------------------------------------
	ginaInv := decode[invitationView](t, owner.must(http.StatusCreated, "POST", invites, obj{"email": "gina@benchmarq.io", "role": "candidate"}))
	if _, err := pool.Exec(ctx, `UPDATE organization_invitations SET expires_at = now() - interval '1 second' WHERE id = $1`, ginaInv.ID); err != nil {
		t.Fatal(err)
	}
	gina, _ := signUp(t, srv, "gina@benchmarq.io")
	gina.must(http.StatusConflict, "POST", "/invitations/accept", obj{"token": mail.token(t, "org_invite", "gina@benchmarq.io")})
	expired := decode[struct {
		Items []invitationView `json:"items"`
	}](t, owner.must(http.StatusOK, "GET", invites+"?status=expired", nil))
	if len(expired.Items) != 1 || expired.Items[0].ID != ginaInv.ID || expired.Items[0].Status != "expired" {
		t.Fatalf("expected gina's invitation to be listed as expired, got %+v", expired.Items)
	}
	// A lapsed invitation does not block a new one for the same address.
	owner.must(http.StatusCreated, "POST", invites, obj{"email": "gina@benchmarq.io", "role": "candidate"})
	gina.must(http.StatusOK, "POST", "/invitations/accept", obj{"token": mail.token(t, "org_invite", "gina@benchmarq.io")})

	// ----- Listing -----------------------------------------------------------
	all := decode[struct {
		Items []invitationView `json:"items"`
	}](t, owner.must(http.StatusOK, "GET", invites, nil))
	statuses := map[string]int{}
	for _, inv := range all.Items {
		statuses[inv.Status]++
	}
	if statuses["accepted"] != 3 || statuses["revoked"] != 1 || statuses["expired"] != 1 || len(all.Items) != 5 {
		t.Fatalf("unexpected invitation statuses %v", statuses)
	}
	owner.must(http.StatusUnprocessableEntity, "GET", invites+"?status=bogus", nil)
}

func TestExamAssignments(t *testing.T) {
	srv, pool, mail := setupServerWithMail(t)
	ctx := context.Background()

	owner, _ := signUp(t, srv, "examiner@benchmarq.io")
	_, c1 := signUp(t, srv, "cand1@benchmarq.io")
	_, c2 := signUp(t, srv, "cand2@benchmarq.io")
	proctor, proctorID := signUp(t, srv, "proctor@benchmarq.io")
	_, outsider := signUp(t, srv, "outsider@benchmarq.io")
	mallory, _ := signUp(t, srv, "mallory2@benchmarq.io")

	org := decode[struct {
		ID string `json:"id"`
	}](t, owner.must(http.StatusCreated, "POST", "/orgs", obj{"name": "Assign Co"}))
	for id, role := range map[string]string{c1: "candidate", c2: "candidate", proctorID: "proctor"} {
		if _, err := pool.Exec(ctx, `INSERT INTO organization_members (org_id, user_id, role) VALUES ($1, $2, $3)`, org.ID, id, role); err != nil {
			t.Fatal(err)
		}
	}

	examsPath := "/orgs/" + org.ID + "/exams"
	exam := decode[examDetail](t, owner.must(http.StatusCreated, "POST", examsPath, obj{
		"title": "Aptitude", "config": obj{"schedule": obj{"schedule_type": "open_ended", "total_duration_minutes": 45}},
	}))
	examPath := examsPath + "/" + exam.Exam.ID
	section := decode[sectionView](t, owner.must(http.StatusCreated, "POST", examPath+"/sections", obj{"name": "Quant", "section_type": "objective"}))
	owner.must(http.StatusCreated, "POST", examPath+"/sections/"+section.ID+"/questions", mcq("2 + 0", 1))
	assignPath := examPath + "/assignments"

	type assignResult struct {
		Assignments []struct {
			ID                string  `json:"id"`
			UserID            string  `json:"user_id"`
			Email             string  `json:"email"`
			InviteEmailSentAt *string `json:"invite_email_sent_at"`
		} `json:"assignments"`
		Created int `json:"created"`
		Emailed int `json:"emailed"`
	}

	// Drafts accept assignments but cannot email invitations yet.
	draft := decode[assignResult](t, owner.must(http.StatusOK, "POST", assignPath, obj{"user_ids": []string{c1}}))
	if draft.Created != 1 || draft.Emailed != 0 || len(draft.Assignments) != 1 || draft.Assignments[0].Email != "cand1@benchmarq.io" {
		t.Fatalf("unexpected draft assignment %+v", draft)
	}
	r := owner.must(http.StatusUnprocessableEntity, "POST", assignPath, obj{"user_ids": []string{c2}, "send_email": true})
	if r.Errors["send_email"] == "" {
		t.Fatalf("expected send_email error, got %v", r.Errors)
	}
	r = owner.must(http.StatusUnprocessableEntity, "POST", assignPath, obj{"user_ids": []string{c2, outsider}})
	if r.Errors["user_ids"] == "" {
		t.Fatalf("expected non-member error, got %v", r.Errors)
	}
	owner.must(http.StatusUnprocessableEntity, "POST", assignPath, obj{"user_ids": []string{}})
	owner.must(http.StatusUnprocessableEntity, "POST", assignPath, obj{"user_ids": []string{c1, c1}})
	if mail.count("exam_invite", "cand1@benchmarq.io")+mail.count("exam_invite", "cand2@benchmarq.io") != 0 {
		t.Fatal("no invitation should have been emailed yet")
	}

	owner.must(http.StatusOK, "POST", examPath+"/publish", nil)

	// Emailing is optional and covers already-assigned users too.
	published := decode[assignResult](t, owner.must(http.StatusOK, "POST", assignPath, obj{"user_ids": []string{c1, c2}, "send_email": true}))
	if published.Created != 1 || published.Emailed != 2 || len(published.Assignments) != 2 {
		t.Fatalf("unexpected assignment result %+v", published)
	}
	for _, a := range published.Assignments {
		if a.InviteEmailSentAt == nil {
			t.Fatalf("invite_email_sent_at not recorded for %s", a.Email)
		}
	}
	if mail.count("exam_invite", "cand1@benchmarq.io") != 1 || mail.count("exam_invite", "cand2@benchmarq.io") != 1 {
		t.Fatal("each candidate should get one invitation email")
	}
	again := decode[assignResult](t, owner.must(http.StatusOK, "POST", assignPath, obj{"user_ids": []string{c2}}))
	if again.Created != 0 || again.Emailed != 0 || mail.count("exam_invite", "cand2@benchmarq.io") != 1 {
		t.Fatalf("re-assigning without send_email should change nothing: %+v", again)
	}

	list := decode[struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}](t, owner.must(http.StatusOK, "GET", assignPath, nil))
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 assignments, got %+v", list.Items)
	}

	// RBAC and tenant isolation.
	proctor.must(http.StatusForbidden, "GET", assignPath, nil)
	proctor.must(http.StatusForbidden, "POST", assignPath, obj{"user_ids": []string{c1}})
	mallory.must(http.StatusNotFound, "GET", assignPath, nil)

	owner.must(http.StatusOK, "DELETE", assignPath+"/"+list.Items[0].ID, nil)
	owner.must(http.StatusNotFound, "DELETE", assignPath+"/"+list.Items[0].ID, nil)

	owner.must(http.StatusOK, "POST", examPath+"/archive", nil)
	owner.must(http.StatusConflict, "POST", assignPath, obj{"user_ids": []string{c1}})
}
