"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import FormControlLabel from "@mui/material/FormControlLabel";
import MenuItem from "@mui/material/MenuItem";
import Radio from "@mui/material/Radio";
import Stack from "@mui/material/Stack";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { FormErrors, dialogPaper } from "@/components/exams/common";
import { apiError } from "@/hooks/api/useExams";

export const QUESTION_TYPES = {
  multiple_choice: "Multiple choice",
  multi_select: "Multiple select",
  true_false: "True / false",
  blank_fill: "Fill in the blank",
  short_answer: "Short answer",
  essay: "Essay",
};

const TYPES_BY_SECTION = {
  objective: ["multiple_choice", "multi_select", "true_false", "blank_fill"],
  subjective: ["short_answer", "essay"],
};

const lines = (text) =>
  text
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
const numOrNull = (v) => (v === "" || v == null ? null : Number(v));
const str = (v) => (v == null ? "" : String(v));

/** Form state from an existing question (or defaults for a new one). */
function initialForm(question, sectionType) {
  const q = question ?? {};
  return {
    type: q.type ?? TYPES_BY_SECTION[sectionType][0],
    prompt: q.prompt ?? "",
    explanation: q.explanation ?? "",
    points: str(q.points ?? 1),
    negative_points: str(q.negative_points ?? 0),
    options: q.options?.map((o) => ({ content: o.content, is_correct: o.is_correct, is_pinned: o.is_pinned })) ?? [
      { content: "", is_correct: true, is_pinned: false },
      { content: "", is_correct: false, is_pinned: false },
      { content: "", is_correct: false, is_pinned: false },
      { content: "", is_correct: false, is_pinned: false },
    ],
    min_choices: str(q.multi_select?.min_choices_required ?? 1),
    max_choices: str(q.multi_select?.max_choices_allowed),
    tf_answer: q.true_false ? String(q.true_false.correct_answer) : "true",
    blank_answer: q.blank_fill?.correct_answer ?? "",
    blank_case: q.blank_fill?.case_sensitive ?? false,
    blank_alternates: (q.blank_fill?.alternate_answers ?? []).join("\n"),
    sample_answer: q.short_answer?.sample_answer ?? "",
    max_words: str(q.short_answer?.max_words ?? q.essay?.max_words),
    rubric_keywords: (q.short_answer?.rubric_keywords ?? []).join(", "),
    min_words: str(q.essay?.min_words),
    rubric_guidelines: q.essay?.rubric_guidelines ?? "",
  };
}

/** Builds the API payload: only the block that matches the type is sent. */
function toPayload(f) {
  const body = {
    type: f.type,
    prompt: f.prompt,
    explanation: f.explanation.trim() || null,
    points: Number(f.points),
    negative_points: Number(f.negative_points || 0),
  };
  switch (f.type) {
    case "multiple_choice":
    case "multi_select":
      body.options = f.options.map((o) => ({ content: o.content, is_correct: o.is_correct, is_pinned: o.is_pinned }));
      if (f.type === "multi_select") {
        body.multi_select = { min_choices_required: numOrNull(f.min_choices) ?? 1, max_choices_allowed: numOrNull(f.max_choices) };
      }
      break;
    case "true_false":
      body.true_false = { correct_answer: f.tf_answer === "true" };
      break;
    case "blank_fill":
      body.blank_fill = { correct_answer: f.blank_answer, case_sensitive: f.blank_case, alternate_answers: lines(f.blank_alternates) };
      break;
    case "short_answer":
      body.short_answer = {
        sample_answer: f.sample_answer.trim() || null,
        max_words: numOrNull(f.max_words),
        rubric_keywords: f.rubric_keywords
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
      };
      break;
    case "essay":
      body.essay = { min_words: numOrNull(f.min_words), max_words: numOrNull(f.max_words), rubric_guidelines: f.rubric_guidelines.trim() || null };
      break;
  }
  return body;
}

export default function QuestionDialog({ open, onClose, sectionType, question, onSave }) {
  const [form, setForm] = useState(() => initialForm(question, sectionType));
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const set = (key) => (e) => setForm((f) => ({ ...f, [key]: e.target.value }));
  const setOption = (i, patch) => setForm((f) => ({ ...f, options: f.options.map((o, j) => (j === i ? { ...o, ...patch } : o)) }));

  const markCorrect = (i, checked) =>
    setForm((f) => ({
      ...f,
      // Multiple choice has exactly one correct option.
      options: f.options.map((o, j) => (f.type === "multiple_choice" ? { ...o, is_correct: j === i } : j === i ? { ...o, is_correct: checked } : o)),
    }));

  const submit = async (e) => {
    e.preventDefault();
    setError(null);
    setSaving(true);
    try {
      await onSave(toPayload(form));
      onClose();
    } catch (err) {
      setError(apiError(err, "Could not save the question."));
    } finally {
      setSaving(false);
    }
  };

  const usesOptions = form.type === "multiple_choice" || form.type === "multi_select";

  return (
    <Dialog open={open} onClose={() => !saving && onClose()} maxWidth="md" fullWidth slotProps={dialogPaper}>
      <form onSubmit={submit}>
        <DialogTitle sx={{ fontSize: "1.125rem", fontWeight: 600 }}>{question ? "Edit question" : "New question"}</DialogTitle>
        <DialogContent>
          <FormErrors error={error} />
          <Stack spacing={2} sx={{ pt: 1 }}>
            <Stack direction={{ xs: "column", sm: "row" }} spacing={2}>
              <TextField select label="Type" value={form.type} onChange={set("type")} sx={{ minWidth: 220 }}>
                {TYPES_BY_SECTION[sectionType].map((t) => (
                  <MenuItem key={t} value={t}>
                    {QUESTION_TYPES[t]}
                  </MenuItem>
                ))}
              </TextField>
              <TextField label="Marks" type="number" value={form.points} onChange={set("points")} required slotProps={{ htmlInput: { min: 0.25, step: 0.25 } }} />
              {sectionType === "objective" && (
                <TextField
                  label="Negative marks"
                  type="number"
                  value={form.negative_points}
                  onChange={set("negative_points")}
                  slotProps={{ htmlInput: { min: 0, step: 0.25 } }}
                  helperText="Applies when negative marking is on"
                />
              )}
            </Stack>

            <TextField label="Question" value={form.prompt} onChange={set("prompt")} required multiline minRows={3} />

            {usesOptions && (
              <Box>
                <Typography sx={{ fontWeight: 600, fontSize: "0.875rem", mb: 1 }}>
                  Options{" "}
                  <Typography component="span" variant="body2">
                    — mark {form.type === "multiple_choice" ? "the correct answer" : "every correct answer"}; pinned options keep their place when options are shuffled
                  </Typography>
                </Typography>
                <Stack spacing={1}>
                  {form.options.map((o, i) => (
                    <Stack key={i} direction="row" spacing={1} sx={{ alignItems: "center" }}>
                      {form.type === "multiple_choice" ? (
                        <Radio size="small" checked={o.is_correct} onChange={() => markCorrect(i, true)} slotProps={{ input: { "aria-label": `Option ${i + 1} is correct` } }} />
                      ) : (
                        <Checkbox size="small" checked={o.is_correct} onChange={(e) => markCorrect(i, e.target.checked)} slotProps={{ input: { "aria-label": `Option ${i + 1} is correct` } }} />
                      )}
                      <TextField size="small" fullWidth placeholder={`Option ${i + 1}`} value={o.content} onChange={(e) => setOption(i, { content: e.target.value })} required />
                      <FormControlLabel
                        control={<Checkbox size="small" checked={o.is_pinned} onChange={(e) => setOption(i, { is_pinned: e.target.checked })} />}
                        label="Pinned"
                        sx={{ m: 0, "& .MuiFormControlLabel-label": { fontSize: "0.8125rem" } }}
                      />
                      <Button
                        size="small"
                        color="secondary"
                        disabled={form.options.length <= 2}
                        onClick={() => setForm((f) => ({ ...f, options: f.options.filter((_, j) => j !== i) }))}
                      >
                        Remove
                      </Button>
                    </Stack>
                  ))}
                </Stack>
                <Button
                  size="small"
                  variant="outlined"
                  sx={{ mt: 1 }}
                  disabled={form.options.length >= 26}
                  onClick={() => setForm((f) => ({ ...f, options: [...f.options, { content: "", is_correct: false, is_pinned: false }] }))}
                >
                  Add option
                </Button>
                {form.type === "multi_select" && (
                  <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
                    <TextField size="small" label="Minimum selections" type="number" value={form.min_choices} onChange={set("min_choices")} />
                    <TextField size="small" label="Maximum selections" type="number" value={form.max_choices} onChange={set("max_choices")} helperText="Leave empty for no limit" />
                  </Stack>
                )}
              </Box>
            )}

            {form.type === "true_false" && (
              <TextField select label="Correct answer" value={form.tf_answer} onChange={set("tf_answer")} sx={{ maxWidth: 220 }}>
                <MenuItem value="true">True</MenuItem>
                <MenuItem value="false">False</MenuItem>
              </TextField>
            )}

            {form.type === "blank_fill" && (
              <>
                <TextField label="Correct answer" value={form.blank_answer} onChange={set("blank_answer")} required />
                <TextField label="Also accept" value={form.blank_alternates} onChange={set("blank_alternates")} multiline minRows={2} helperText="One accepted answer per line" />
                <FormControlLabel
                  control={<Switch size="small" checked={form.blank_case} onChange={(e) => setForm((f) => ({ ...f, blank_case: e.target.checked }))} />}
                  label="Case sensitive"
                />
              </>
            )}

            {form.type === "short_answer" && (
              <>
                <TextField label="Sample answer (for graders)" value={form.sample_answer} onChange={set("sample_answer")} multiline minRows={2} />
                <Stack direction="row" spacing={2}>
                  <TextField label="Word limit" type="number" value={form.max_words} onChange={set("max_words")} />
                  <TextField label="Rubric keywords" value={form.rubric_keywords} onChange={set("rubric_keywords")} fullWidth helperText="Comma separated" />
                </Stack>
              </>
            )}

            {form.type === "essay" && (
              <>
                <Stack direction="row" spacing={2}>
                  <TextField label="Minimum words" type="number" value={form.min_words} onChange={set("min_words")} />
                  <TextField label="Maximum words" type="number" value={form.max_words} onChange={set("max_words")} />
                </Stack>
                <TextField label="Grading guidelines" value={form.rubric_guidelines} onChange={set("rubric_guidelines")} multiline minRows={3} />
              </>
            )}

            <TextField label="Explanation (shown with solutions)" value={form.explanation} onChange={set("explanation")} multiline minRows={2} />
          </Stack>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="outlined" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" variant="contained" disabled={saving}>
            {saving ? <CircularProgress size={18} color="inherit" /> : "Save question"}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
