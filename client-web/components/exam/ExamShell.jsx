"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import List from "@mui/material/List";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import Snackbar from "@mui/material/Snackbar";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import QuestionPanel from "@/components/exam/QuestionPanel";
import { formatDuration, syncServerTime, useRemaining } from "@/hooks/exam/useServerClock";
import { VIOLATION_TEXT } from "@/lib/exam/bridge";

const border = "1px solid #e4e4e7";

/**
 * The exam workspace: header with timers, section list, question, question
 * palette and navigation footer. Timing and locking are decided by the
 * server; this component renders what it is told.
 */
export default function ExamShell({ runtime }) {
  const { state, attempt, rules, actions, online, violation } = runtime;
  const [question, setQuestion] = useState(null);
  const [navigating, setNavigating] = useState(false);
  const [error, setError] = useState(null);
  const [confirm, setConfirm] = useState(null); // "submit" | "finish"
  const [warning, setWarning] = useState(null);
  const panel = useRef(null);

  const clock = attempt?.clock;
  const current = attempt?.current_section ?? null;
  const sectionId = current?.section_id;
  const currentSection = attempt?.sections?.find((s) => s.id === sectionId);
  const remaining = useRemaining(clock?.deadline_at, { frozen: clock?.paused });
  const sectionRemaining = useRemaining(clock?.section?.deadline_at, { frozen: clock?.paused });
  const canSubmitIn = useRemaining(clock?.can_submit_at);

  const run = useCallback(
    async (fn) => {
      setError(null);
      try {
        return await fn();
      } catch (err) {
        setError(err.message);
        if (err.status === 409) actions.refresh().catch(() => {});
        return null;
      }
    },
    [actions],
  );

  const showQuestion = useCallback(
    (q) => {
      if (!q) return;
      if (q.clock?.server_time) syncServerTime(q.clock.server_time);
      setQuestion(q);
      // Mirror the cursor locally; the next heartbeat brings the full picture.
      runtime.setAttempt((a) =>
        a?.current_section
          ? {
              ...a,
              current_section: {
                ...a.current_section,
                cursor: q.position,
                slots: a.current_section.slots.map((s) => (s.position === q.position ? { ...s, viewed: true } : s)),
              },
            }
          : a,
      );
    },
    [runtime],
  );

  // Load the question under the cursor whenever the open section changes.
  useEffect(() => {
    if (!sectionId) return;
    actions
      .question()
      .then(showQuestion)
      .catch((err) => setError(err.message));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sectionId]);
  const shown = question && question.section_id === sectionId ? question : null;
  const loading = navigating || Boolean(sectionId && !shown);

  // When a timer reaches zero, the server decides what happens next.
  useEffect(() => {
    if (remaining === 0 || sectionRemaining === 0) actions.refresh().catch(() => {});
  }, [remaining, sectionRemaining, actions]);

  const thresholds = useMemo(() => rules?.experience?.timer_warning_thresholds_minutes ?? [], [rules]);
  const warned = useRef(new Set());
  useEffect(() => {
    if (remaining == null) return;
    for (const m of thresholds) {
      if (remaining <= m * 60 && remaining > m * 60 - 2 && !warned.current.has(m)) {
        warned.current.add(m);
        setWarning(`${m} minute${m === 1 ? "" : "s"} remaining`);
      }
    }
  }, [remaining, thresholds]);

  const goTo = async (position) => {
    if (!(await panel.current?.flush?.())) return;
    setNavigating(true);
    showQuestion(await run(() => actions.navigate(position)));
    setNavigating(false);
  };

  const save = async (input) => {
    const saved = await actions.saveResponse(input);
    const answered = Boolean(saved && (saved.selected_option_id || saved.selected_option_ids?.length || saved.text || saved.boolean != null));
    runtime.setAttempt((a) =>
      a?.current_section
        ? {
            ...a,
            current_section: {
              ...a.current_section,
              slots: a.current_section.slots.map((s) => (s.position === shown.position ? { ...s, answered, flagged: Boolean(saved?.flagged) } : s)),
            },
          }
        : a,
    );
    return saved;
  };

  const finishSection = async () => {
    await panel.current?.flush?.();
    await run(() => actions.finishSection(sectionId));
    setConfirm(null);
  };

  const submit = async () => {
    await panel.current?.flush?.();
    await run(actions.submit);
    setConfirm(null);
  };

  const slots = current?.slots ?? [];
  const answeredHere = slots.filter((s) => s.answered).length;
  const flagged = slots.filter((s) => s.flagged).length;
  const unanswered = (attempt?.total ?? 0) - (attempt?.answered ?? 0);
  const mustAnswer = rules?.question_navigation_mode === "mandatory_answer";
  const currentAnswered = slots.find((s) => s.position === shown?.position)?.answered;
  const canGoBack = rules?.allow_revisiting_questions && shown?.position > 0;
  const lowTime = remaining != null && remaining <= 300;
  const linearLock = rules?.section_progression_mode === "linear_strict" || rules?.section_progression_mode === "timed_sequential";

  return (
    <Box sx={{ height: "100vh", display: "flex", flexDirection: "column", bgcolor: "#ffffff", userSelect: "none" }}>
      {/* Header */}
      <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between", px: 3, py: 1.5, borderBottom: border, gap: 3 }}>
        <Box sx={{ minWidth: 0 }}>
          <Typography noWrap sx={{ fontWeight: 600 }}>
            {state?.exam?.title}
          </Typography>
          <Typography variant="body2">
            {attempt?.answered ?? 0} of {attempt?.total ?? 0} answered
            {!online && " · Reconnecting…"}
          </Typography>
        </Box>
        <Stack direction="row" spacing={3} sx={{ alignItems: "center" }}>
          {clock?.section && <Timer label="Section" seconds={sectionRemaining} />}
          {rules?.experience?.show_countdown_timer !== false && <Timer label="Time left" seconds={remaining} strong={lowTime} />}
          {rules?.allow_pause && clock?.pauses_left > 0 && (
            <Button variant="outlined" onClick={() => run(actions.pause)}>
              Pause
            </Button>
          )}
          <Button variant="contained" onClick={() => setConfirm("submit")} disabled={canSubmitIn > 0}>
            {canSubmitIn > 0 ? `Submit in ${formatDuration(canSubmitIn)}` : "Submit exam"}
          </Button>
        </Stack>
      </Stack>

      <Box sx={{ flex: 1, minHeight: 0, display: "flex" }}>
        {/* Sections */}
        <Box sx={{ width: 260, flexShrink: 0, borderRight: border, overflowY: "auto", py: 2 }}>
          <Typography variant="body2" sx={{ px: 2.5, mb: 1, fontWeight: 600, color: "text.primary" }}>
            Sections
          </Typography>
          <List disablePadding>
            {attempt?.sections?.map((s) => {
              const open = s.id === sectionId;
              return (
                <ListItemButton
                  key={s.id}
                  selected={open}
                  disabled={!open && !s.can_enter}
                  onClick={() => !open && run(() => actions.enterSection(s.id))}
                  sx={{ px: 2.5, borderLeft: open ? "2px solid #09090b" : "2px solid transparent" }}
                >
                  <ListItemText
                    primary={s.name}
                    secondary={`${sectionStatus(s, open)} · ${s.answered_count}/${s.question_count} answered`}
                    slotProps={{ primary: { sx: { fontSize: "0.875rem", fontWeight: open ? 600 : 500 } }, secondary: { sx: { fontSize: "0.75rem" } } }}
                  />
                </ListItemButton>
              );
            })}
          </List>
        </Box>

        {/* Question */}
        <Box sx={{ flex: 1, minWidth: 0, overflowY: "auto", px: 6, py: 4, position: "relative" }}>
          {loading && <CircularProgress size={18} sx={{ position: "absolute", top: 24, right: 24 }} />}
          {!current && (
            <Box sx={{ maxWidth: 420, mx: "auto", mt: 10, textAlign: "center" }}>
              <Typography variant="h3">Choose a section</Typography>
              <Typography variant="body2" sx={{ mt: 1 }}>
                Select an unlocked section from the list to continue.
              </Typography>
            </Box>
          )}
          {currentSection?.instructions && shown?.position === 0 && (
            <Box sx={{ mb: 4, p: 2, border, borderRadius: 1.5 }}>
              <Typography sx={{ whiteSpace: "pre-wrap" }}>{currentSection.instructions}</Typography>
            </Box>
          )}
          {shown && (
            <QuestionPanel
              key={`${shown.question_id}:${shown.response?.saved_at ?? ""}`}
              ref={panel}
              question={shown}
              rules={rules}
              onSave={save}
              onAutoAdvance={() => goTo(shown.position + 1)}
            />
          )}
          {error && (
            <Typography sx={{ mt: 2, fontWeight: 500 }} role="alert">
              {error}
            </Typography>
          )}
        </Box>

        {/* Palette */}
        {current && (
          <Box sx={{ width: 264, flexShrink: 0, borderLeft: border, overflowY: "auto", p: 2.5 }}>
            <Stack direction="row" sx={{ justifyContent: "space-between" }}>
              <Typography variant="body2" sx={{ fontWeight: 600, color: "text.primary" }}>
                {currentSection?.name}
              </Typography>
              <Typography variant="body2">
                {answeredHere}/{slots.length}
              </Typography>
            </Stack>
            <Box sx={{ mt: 2, display: "grid", gridTemplateColumns: "repeat(5, 1fr)", gap: 1 }}>
              {slots.map((s) => (
                <PaletteButton key={s.position} slot={s} active={s.position === shown?.position} onClick={() => goTo(s.position)} />
              ))}
            </Box>
            <Stack spacing={1} sx={{ mt: 3 }}>
              <Legend sample={{ answered: true }} label="Answered" />
              <Legend sample={{ viewed: true }} label="Visited, not answered" />
              <Legend sample={{}} label="Not visited" />
              <Legend sample={{ flagged: true }} label="Marked for review" />
            </Stack>
            {flagged > 0 && (
              <Typography variant="body2" sx={{ mt: 2 }}>
                {flagged} marked for review
              </Typography>
            )}
          </Box>
        )}
      </Box>

      {/* Footer */}
      {shown && (
        <Stack direction="row" sx={{ justifyContent: "space-between", px: 3, py: 1.5, borderTop: border }}>
          <Button variant="outlined" onClick={() => goTo(shown.position - 1)} disabled={!canGoBack}>
            Previous
          </Button>
          {shown.is_last ? (
            <Button variant="outlined" onClick={() => setConfirm("finish")}>
              Finish section
            </Button>
          ) : (
            <Button variant="contained" onClick={() => goTo(shown.position + 1)} disabled={mustAnswer && !currentAnswered}>
              Save and next
            </Button>
          )}
        </Stack>
      )}

      <Dialog open={confirm === "submit"} onClose={() => setConfirm(null)} maxWidth="xs" fullWidth>
        <DialogTitle>Submit exam?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            You answered {attempt?.answered ?? 0} of {attempt?.total ?? 0} questions
            {unanswered > 0 ? `; ${unanswered} unanswered` : ""}. You cannot change your answers after submitting.
          </DialogContentText>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="outlined" onClick={() => setConfirm(null)}>
            Keep working
          </Button>
          <Button variant="contained" onClick={submit}>
            Submit
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={confirm === "finish"} onClose={() => setConfirm(null)} maxWidth="xs" fullWidth>
        <DialogTitle>Finish {currentSection?.name ?? "section"}?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {answeredHere} of {slots.length} answered in this section.
            {linearLock ? " You will not be able to return to it." : " Depending on the exam rules, you may not be able to return to it."}
          </DialogContentText>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="outlined" onClick={() => setConfirm(null)}>
            Stay
          </Button>
          <Button variant="contained" onClick={finishSection}>
            Finish section
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={Boolean(clock?.paused)} maxWidth="xs" fullWidth>
        <DialogTitle>Exam paused</DialogTitle>
        <DialogContent>
          <DialogContentText component="div">{clock?.paused && <PauseCountdown clock={clock} />}</DialogContentText>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="contained" onClick={() => run(actions.resume)}>
            Resume exam
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        open={Boolean(warning)}
        autoHideDuration={8000}
        onClose={() => setWarning(null)}
        message={warning}
        anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      />
      <Snackbar
        open={Boolean(violation)}
        message={violation ? (VIOLATION_TEXT[violation.type] ?? "A proctoring event was recorded.") : ""}
        anchorOrigin={{ vertical: "top", horizontal: "center" }}
      />
    </Box>
  );
}

function Timer({ label, seconds, strong }) {
  return (
    <Box sx={{ textAlign: "right" }}>
      <Typography variant="body2" sx={{ fontSize: "0.6875rem", textTransform: "uppercase", letterSpacing: "0.04em" }}>
        {label}
      </Typography>
      <Typography sx={{ fontSize: "1.25rem", fontWeight: strong ? 700 : 600, fontVariantNumeric: "tabular-nums", textDecoration: strong ? "underline" : "none" }}>
        {formatDuration(seconds)}
      </Typography>
    </Box>
  );
}

function PauseCountdown({ clock }) {
  // pause_seconds_left is a budget measured at server_time.
  const deadline = new Date(Date.parse(clock.server_time) + clock.pause_seconds_left * 1000).toISOString();
  const left = useRemaining(deadline);
  return <>The timer is stopped. Pause time left: {formatDuration(left)}. The exam resumes automatically when it runs out.</>;
}

function sectionStatus(section, open) {
  if (open) return "Open";
  if (section.status === "completed") return "Completed";
  if (section.status === "skipped") return "Skipped";
  if (!section.can_enter) {
    if (section.unlocks_at) return `Unlocks at ${new Date(section.unlocks_at).toLocaleTimeString()}`;
    return "Locked";
  }
  return "Available";
}

/** Answered: filled black. Visited: grey. Flagged: corner marker. Current: thick outline. */
function slotSx(slot, active) {
  return {
    position: "relative",
    height: 36,
    borderRadius: 1,
    fontSize: "0.8125rem",
    fontWeight: 500,
    cursor: slot.reachable === false ? "default" : "pointer",
    border: active ? "2px solid #09090b" : "1px solid #e4e4e7",
    bgcolor: slot.answered ? "#09090b" : slot.viewed ? "#f4f4f5" : "#ffffff",
    color: slot.answered ? "#ffffff" : slot.reachable === false ? "#a1a1aa" : "#09090b",
    "&::after": slot.flagged
      ? { content: '""', position: "absolute", top: 0, right: 0, borderTop: "8px solid #71717a", borderLeft: "8px solid transparent" }
      : undefined,
  };
}

function PaletteButton({ slot, active, onClick }) {
  return (
    <Box component="button" type="button" disabled={!slot.reachable || active} onClick={onClick} sx={slotSx(slot, active)}>
      {slot.position + 1}
    </Box>
  );
}

function Legend({ sample, label }) {
  return (
    <Stack direction="row" spacing={1} sx={{ alignItems: "center" }}>
      <Box sx={{ ...slotSx({ reachable: true, ...sample }, false), width: 18, height: 18 }} />
      <Typography variant="body2">{label}</Typography>
    </Stack>
  );
}
