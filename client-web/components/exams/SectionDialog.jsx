"use client";

import { useState } from "react";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import FormControlLabel from "@mui/material/FormControlLabel";
import MenuItem from "@mui/material/MenuItem";
import Stack from "@mui/material/Stack";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { FormErrors, dialogPaper } from "@/components/exams/common";
import { apiError } from "@/hooks/api/useExams";

// Section overrides are three-way: inherit the exam setting, or force on/off.
const toTri = (v) => (v === true ? "on" : v === false ? "off" : "inherit");
const fromTri = (v) => (v === "on" ? true : v === "off" ? false : null);
const numOrNull = (v) => (v === "" || v == null ? null : Number(v));
const str = (v) => (v == null ? "" : String(v));

export default function SectionDialog({ open, onClose, section, sections, config, onSave }) {
  const s = section ?? {};
  const [form, setForm] = useState({
    name: s.name ?? "",
    section_type: s.section_type ?? "objective",
    description: s.description ?? "",
    instructions: s.instructions ?? "",
    duration_minutes: str(s.duration_minutes),
    question_pick_count: str(s.question_pick_count),
    cutoff_marks: str(s.cutoff_marks),
    unlock_after_minutes: str(s.unlock_after_minutes),
    lock_on_complete: s.lock_on_complete ?? false,
    shuffle_questions: toTri(s.shuffle_questions),
    shuffle_options: toTri(s.shuffle_options),
    prerequisites: (s.prerequisites ?? []).map((p) => p.section_id),
  });
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const set = (key) => (e) => setForm((f) => ({ ...f, [key]: e.target.value }));

  const others = (sections ?? []).filter((x) => x.id !== s.id);
  const perSectionTiming = config?.schedule?.per_section_timing;

  const submit = async (e) => {
    e.preventDefault();
    setError(null);
    setSaving(true);
    // Keep any minimum-score gates already set on retained prerequisites.
    const existing = new Map((s.prerequisites ?? []).map((p) => [p.section_id, p]));
    const body = {
      name: form.name,
      description: form.description.trim() || null,
      instructions: form.instructions.trim() || null,
      duration_minutes: numOrNull(form.duration_minutes),
      question_pick_count: numOrNull(form.question_pick_count),
      cutoff_marks: numOrNull(form.cutoff_marks),
      unlock_after_minutes: numOrNull(form.unlock_after_minutes),
      lock_on_complete: form.lock_on_complete,
      shuffle_questions: fromTri(form.shuffle_questions),
      shuffle_options: fromTri(form.shuffle_options),
      prerequisites: form.prerequisites.map((id) => existing.get(id) ?? { section_id: id }),
    };
    if (!section) body.section_type = form.section_type;
    try {
      await onSave(body);
      onClose();
    } catch (err) {
      setError(apiError(err, "Could not save the section."));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onClose={() => !saving && onClose()} maxWidth="sm" fullWidth slotProps={dialogPaper}>
      <form onSubmit={submit}>
        <DialogTitle sx={{ fontSize: "1.125rem", fontWeight: 600 }}>{section ? "Edit section" : "New section"}</DialogTitle>
        <DialogContent>
          <FormErrors error={error} />
          <Stack spacing={2} sx={{ pt: 1 }}>
            <TextField label="Name" value={form.name} onChange={set("name")} required autoFocus />
            <TextField
              select
              label="Question type"
              value={form.section_type}
              onChange={set("section_type")}
              disabled={Boolean(section)}
              helperText={section ? "Cannot be changed after the section is created" : undefined}
            >
              <MenuItem value="objective">Objective — choice and fill-in questions, graded automatically</MenuItem>
              <MenuItem value="subjective">Subjective — written answers, graded manually</MenuItem>
            </TextField>
            <TextField label="Instructions" value={form.instructions} onChange={set("instructions")} multiline minRows={2} />

            <Typography sx={{ fontWeight: 600, fontSize: "0.875rem", pt: 1 }}>Timing and access</Typography>
            <Stack direction="row" spacing={2}>
              <TextField
                label="Time limit (minutes)"
                type="number"
                value={form.duration_minutes}
                onChange={set("duration_minutes")}
                fullWidth
                helperText={perSectionTiming ? "Required for every section" : "Only used when per-section timing is on"}
              />
              <TextField
                label="Unlocks after (minutes)"
                type="number"
                value={form.unlock_after_minutes}
                onChange={set("unlock_after_minutes")}
                fullWidth
                helperText="Minutes into the attempt"
              />
            </Stack>
            {others.length > 0 && (
              <TextField
                select
                label="Must finish first"
                value={form.prerequisites}
                onChange={(e) => setForm((f) => ({ ...f, prerequisites: e.target.value }))}
                slotProps={{ select: { multiple: true, renderValue: (ids) => others.filter((o) => ids.includes(o.id)).map((o) => o.name).join(", ") } }}
                helperText="This section stays locked until these sections are completed"
              >
                {others.map((o) => (
                  <MenuItem key={o.id} value={o.id}>
                    {o.name}
                  </MenuItem>
                ))}
              </TextField>
            )}
            <FormControlLabel
              control={<Switch size="small" checked={form.lock_on_complete} onChange={(e) => setForm((f) => ({ ...f, lock_on_complete: e.target.checked }))} />}
              label="Lock the section once the candidate leaves it"
            />

            <Typography sx={{ fontWeight: 600, fontSize: "0.875rem", pt: 1 }}>Questions</Typography>
            <Stack direction="row" spacing={2}>
              <TextField select label="Shuffle questions" value={form.shuffle_questions} onChange={set("shuffle_questions")} fullWidth>
                <MenuItem value="inherit">Exam default</MenuItem>
                <MenuItem value="on">Yes</MenuItem>
                <MenuItem value="off">No</MenuItem>
              </TextField>
              <TextField select label="Shuffle options" value={form.shuffle_options} onChange={set("shuffle_options")} fullWidth>
                <MenuItem value="inherit">Exam default</MenuItem>
                <MenuItem value="on">Yes</MenuItem>
                <MenuItem value="off">No</MenuItem>
              </TextField>
            </Stack>
            <Stack direction="row" spacing={2}>
              <TextField
                label="Questions per candidate"
                type="number"
                value={form.question_pick_count}
                onChange={set("question_pick_count")}
                fullWidth
                helperText="Draw this many at random; empty serves all"
              />
              <TextField label="Cutoff marks" type="number" value={form.cutoff_marks} onChange={set("cutoff_marks")} fullWidth helperText="Optional minimum for this section" />
            </Stack>
          </Stack>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="outlined" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="submit" variant="contained" disabled={saving}>
            {saving ? <CircularProgress size={18} color="inherit" /> : "Save section"}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
