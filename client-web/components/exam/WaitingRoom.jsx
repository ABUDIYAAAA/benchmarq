"use client";

import { useEffect, useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import CircularProgress from "@mui/material/CircularProgress";
import Divider from "@mui/material/Divider";
import FormControlLabel from "@mui/material/FormControlLabel";
import Paper from "@mui/material/Paper";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import Typography from "@mui/material/Typography";
import { formatDuration, useRemaining } from "@/hooks/exam/useServerClock";

const POLL_MS = 15_000;

const STATUS_TEXT = {
  upcoming: "Not open yet",
  open: "Open",
  closed: "Closed",
};

/**
 * Waiting area shown from launch until the candidate starts. Candidates may
 * open the app before the exam opens; the server decides when starting is
 * allowed and the countdown follows the server's clock.
 */
export default function WaitingRoom({ state, attempt, actions }) {
  const [agreed, setAgreed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const availability = state?.availability;
  const exam = state?.exam;
  const rules = state?.rules;
  const resuming = attempt?.status === "in_progress";
  const opensIn = useRemaining(availability?.state === "upcoming" ? availability?.opens_at : null);
  const canStart = resuming || availability?.can_start;
  const totalQuestions = exam?.sections?.reduce((n, s) => n + s.question_count, 0) ?? 0;

  // Poll while waiting, and refresh right when the countdown reaches zero.
  useEffect(() => {
    if (resuming) return undefined;
    const id = setInterval(() => actions.refresh().catch(() => {}), POLL_MS);
    return () => clearInterval(id);
  }, [actions, resuming]);

  useEffect(() => {
    if (opensIn === 0) actions.refresh().catch(() => {});
  }, [opensIn, actions]);

  const begin = async () => {
    setBusy(true);
    setError(null);
    try {
      if (resuming) await actions.resumeAttempt();
      else await actions.start();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Box sx={{ minHeight: "100vh", bgcolor: "#fafafa", py: 6, px: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: "auto", display: "grid", gap: 3, gridTemplateColumns: { xs: "1fr", md: "1fr 320px" }, alignItems: "start" }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <Box>
            <Typography variant="body2">Waiting room</Typography>
            <Typography variant="h1" sx={{ mt: 0.5 }}>
              {exam?.title}
            </Typography>
            {exam?.description && (
              <Typography variant="body2" sx={{ mt: 1 }}>
                {exam.description}
              </Typography>
            )}
          </Box>

          <Paper elevation={0} sx={{ borderRadius: 2 }}>
            <Box sx={{ px: 3, pt: 2.5, pb: 1 }}>
              <Typography variant="h3">Exam structure</Typography>
              <Typography variant="body2" sx={{ mt: 0.5 }}>
                {exam?.duration_minutes} minutes · {totalQuestions} questions
              </Typography>
            </Box>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Section</TableCell>
                  <TableCell align="right">Questions</TableCell>
                  <TableCell align="right">Time limit</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {exam?.sections?.map((s, i) => (
                  <TableRow key={i}>
                    <TableCell>{s.name}</TableCell>
                    <TableCell align="right">{s.question_count}</TableCell>
                    <TableCell align="right">{s.time_limit_seconds ? `${Math.round(s.time_limit_seconds / 60)} min` : "—"}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Paper>

          {exam?.instructions && (
            <Paper elevation={0} sx={{ p: 3, borderRadius: 2 }}>
              <Typography variant="h3">Instructions</Typography>
              <Typography sx={{ mt: 1.5, whiteSpace: "pre-wrap", lineHeight: 1.7 }}>{exam.instructions}</Typography>
            </Paper>
          )}

          <Paper elevation={0} sx={{ p: 3, borderRadius: 2 }}>
            <Typography variant="h3">Rules</Typography>
            <Box component="ul" sx={{ mt: 1.5, mb: 0, pl: 2.5, "& li": { fontSize: "0.875rem", lineHeight: 1.8 } }}>
              <li>{progressionText(rules?.section_progression_mode)}</li>
              {!rules?.allow_revisiting_questions && <li>You cannot go back to a question once you move on.</li>}
              {rules?.question_navigation_mode === "mandatory_answer" && <li>Every question must be answered before moving on.</li>}
              {rules?.min_time_before_submission_minutes > 0 && <li>You can submit after {rules.min_time_before_submission_minutes} minutes.</li>}
              <li>The exam runs in full screen. Switching windows or leaving full screen is recorded.</li>
              <li>Applications opened during the exam are closed automatically.</li>
              {rules?.allow_pause && (
                <li>
                  You may pause up to {rules.max_pause_count} time(s), {rules.max_total_pause_minutes} minutes in total.
                </li>
              )}
            </Box>
          </Paper>
        </Box>

        <Paper elevation={0} sx={{ p: 3, borderRadius: 2, position: { md: "sticky" }, top: 24 }}>
          <Typography variant="body2">Status</Typography>
          <Typography sx={{ fontWeight: 600, mt: 0.5 }}>{resuming ? "In progress" : STATUS_TEXT[availability?.state] ?? "Loading"}</Typography>
          {availability?.reason && !canStart && (
            <Typography variant="body2" sx={{ mt: 0.5 }}>
              {availability.reason.replaceAll("_", " ")}
            </Typography>
          )}

          {opensIn != null && opensIn > 0 && (
            <Box sx={{ mt: 2.5, py: 2, textAlign: "center", border: "1px solid", borderColor: "divider", borderRadius: 1.5 }}>
              <Typography variant="body2">Opens in</Typography>
              <Typography sx={{ mt: 0.5, fontSize: "2rem", fontWeight: 600, fontVariantNumeric: "tabular-nums" }}>{formatDuration(opensIn)}</Typography>
            </Box>
          )}

          {availability?.max_attempts > 0 && (
            <Typography variant="body2" sx={{ mt: 2 }}>
              Attempt {Math.min(availability.attempts_used + (resuming ? 0 : 1), availability.max_attempts)} of {availability.max_attempts}
            </Typography>
          )}

          <Divider sx={{ my: 2.5 }} />

          {!resuming && (
            <FormControlLabel
              sx={{ alignItems: "flex-start", m: 0, mb: 2, "& .MuiFormControlLabel-label": { fontSize: "0.8125rem", pt: 1 } }}
              control={<Checkbox size="small" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} />}
              label="I have read the instructions and agree to the exam rules."
            />
          )}

          {error && (
            <Typography variant="body2" sx={{ mb: 2, color: "text.primary", fontWeight: 500 }}>
              {error}
            </Typography>
          )}

          <Button fullWidth variant="contained" size="large" disabled={!canStart || (!resuming && !agreed) || busy} onClick={begin}>
            {busy ? <CircularProgress size={20} color="inherit" /> : resuming ? "Resume exam" : "Start exam"}
          </Button>
        </Paper>
      </Box>
    </Box>
  );
}

function progressionText(mode) {
  switch (mode) {
    case "linear_strict":
      return "Sections are taken in order and lock once you finish them.";
    case "linear_relaxed":
      return "Sections are taken in order; you may revisit finished sections unless they lock.";
    case "timed_sequential":
      return "Each section has its own timer and moves on automatically when it runs out.";
    default:
      return "You may move between unlocked sections in any order.";
  }
}
