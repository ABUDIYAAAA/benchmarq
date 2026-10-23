"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import FormControlLabel from "@mui/material/FormControlLabel";
import MenuItem from "@mui/material/MenuItem";
import Stack from "@mui/material/Stack";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { FormErrors, Panel, fromLocalInput, toLocalInput } from "@/components/exams/common";
import { apiError, useExamMutation } from "@/hooks/api/useExams";
import { useToast } from "@/context/ToastContext";
import examsApi from "@/lib/api/exams.api";

const options = (pairs) => pairs.map(([value, label]) => ({ value, label }));

// The configuration fields exposed in the UI, grouped as on the server.
// `show` hides fields that do not apply given the rest of the configuration.
const GROUPS = [
  {
    key: "schedule",
    title: "Schedule and timing",
    fields: [
      {
        name: "schedule_type",
        label: "Schedule",
        type: "select",
        options: options([
          ["open_ended", "Open ended"],
          ["flexible_window", "Flexible window"],
          ["fixed_window", "Fixed window"],
        ]),
      },
      { name: "start_time", label: "Opens", type: "datetime", show: (c) => c.schedule.schedule_type !== "open_ended" },
      { name: "end_time", label: "Closes", type: "datetime", show: (c) => c.schedule.schedule_type !== "open_ended" },
      { name: "total_duration_minutes", label: "Duration (minutes)", type: "number" },
      { name: "late_join_grace_minutes", label: "Late join allowed for (minutes)", type: "number", show: (c) => c.schedule.schedule_type === "fixed_window" },
      { name: "min_time_before_submission_minutes", label: "Earliest submission (minutes)", type: "number" },
      { name: "per_section_timing", label: "Separate time limit for each section", type: "switch" },
      { name: "auto_submit_on_expiry", label: "Submit automatically when time runs out", type: "switch" },
      { name: "allow_candidate_pause", label: "Allow candidates to pause", type: "switch" },
      { name: "max_pause_count", label: "Pauses allowed", type: "number", show: (c) => c.schedule.allow_candidate_pause },
      { name: "max_total_pause_minutes", label: "Total pause time (minutes)", type: "number", show: (c) => c.schedule.allow_candidate_pause },
    ],
  },
  {
    key: "navigation",
    title: "Navigation",
    fields: [
      {
        name: "section_progression_mode",
        label: "Sections",
        type: "select",
        options: options([
          ["free_navigation", "Any order"],
          ["linear_relaxed", "In order, may revisit"],
          ["linear_strict", "In order, locked once finished"],
          ["timed_sequential", "In order, each section timed separately"],
        ]),
      },
      {
        name: "question_navigation_mode",
        label: "Questions",
        type: "select",
        options: options([
          ["free_jump", "Jump to any question"],
          ["sequential_only", "One after another"],
          ["mandatory_answer", "Must answer before moving on"],
        ]),
      },
      { name: "allow_revisiting_questions", label: "Allow going back to earlier questions", type: "switch" },
      { name: "allow_bookmark_for_review", label: "Allow marking questions for review", type: "switch" },
      { name: "auto_advance_on_selection", label: "Move to the next question after choosing an option", type: "switch" },
    ],
  },
  {
    key: "randomization",
    title: "Randomization",
    fields: [
      { name: "shuffle_sections", label: "Shuffle section order", type: "switch" },
      { name: "shuffle_questions", label: "Shuffle questions", type: "switch" },
      { name: "shuffle_options", label: "Shuffle answer options", type: "switch" },
      { name: "enable_question_pool", label: "Draw a random subset of questions per section", type: "switch" },
      { name: "pool_sample_count", label: "Questions drawn per section", type: "number", nullable: true, show: (c) => c.randomization.enable_question_pool },
      { name: "deterministic_user_seed", label: "Keep the same order if a candidate reconnects", type: "switch" },
    ],
  },
  {
    key: "scoring",
    title: "Scoring",
    fields: [
      { name: "passing_percentage", label: "Passing percentage", type: "number" },
      {
        name: "grading_strategy",
        label: "Grading",
        type: "select",
        options: options([
          ["automated", "Automated"],
          ["hybrid", "Automated and manual"],
          ["manual", "Manual"],
        ]),
      },
      {
        name: "multi_select_scoring_mode",
        label: "Multi-select scoring",
        type: "select",
        options: options([
          ["all_or_nothing", "All or nothing"],
          ["partial_with_penalty", "Partial, wrong picks deduct"],
          ["partial_no_penalty", "Partial, no deduction"],
        ]),
      },
      { name: "negative_marking_enabled", label: "Negative marking", type: "switch" },
      { name: "negative_marking_penalty_percent", label: "Penalty (% of question marks)", type: "number", show: (c) => c.scoring.negative_marking_enabled },
    ],
  },
  {
    key: "proctoring",
    title: "Proctoring",
    fields: [
      { name: "enforce_fullscreen", label: "Require full screen", type: "switch" },
      { name: "max_fullscreen_exit_count", label: "Full-screen exits before the exam ends", type: "number", show: (c) => c.proctoring.enforce_fullscreen },
      { name: "track_tab_focus", label: "Record window switching", type: "switch" },
      { name: "max_tab_switch_count", label: "Window switches before the exam ends", type: "number", show: (c) => c.proctoring.track_tab_focus },
      { name: "block_copy_paste_clipboard", label: "Block copy and paste", type: "switch" },
    ],
  },
  {
    key: "attempts",
    title: "Attempts",
    fields: [
      { name: "max_attempts", label: "Attempts allowed", type: "number" },
      { name: "attempt_cooldown_hours", label: "Wait between attempts (hours)", type: "number", show: (c) => c.attempts.max_attempts > 1 },
      {
        name: "retake_score_rule",
        label: "Score that counts",
        type: "select",
        show: (c) => c.attempts.max_attempts > 1,
        options: options([
          ["best_attempt", "Best attempt"],
          ["latest_attempt", "Latest attempt"],
          ["average_attempt", "Average"],
        ]),
      },
      { name: "allow_resume_on_disconnect", label: "Allow resuming after a disconnect", type: "switch" },
    ],
  },
  {
    key: "results",
    title: "Results",
    fields: [
      {
        name: "results_release_mode",
        label: "Release results",
        type: "select",
        options: options([
          ["instant", "Immediately"],
          ["scheduled", "At a set time"],
          ["manual", "Manually"],
        ]),
      },
      { name: "results_scheduled_time", label: "Release at", type: "datetime", show: (c) => c.results.results_release_mode === "scheduled" },
      {
        name: "solution_visibility",
        label: "Show solutions",
        type: "select",
        options: options([
          ["never", "Never"],
          ["after_submission", "After submitting"],
          ["after_exam_closes", "After the exam closes"],
        ]),
      },
      { name: "show_question_breakdown", label: "Show per-question results", type: "switch" },
    ],
  },
];

function toPatch(config) {
  const patch = {};
  for (const group of GROUPS) {
    patch[group.key] = {};
    for (const f of group.fields) {
      let v = config[group.key][f.name];
      if (f.type === "datetime") v = fromLocalInput(v);
      if (f.type === "number") v = v === "" || v == null ? (f.nullable ? null : 0) : Number(v);
      patch[group.key][f.name] = v;
    }
  }
  return patch;
}

function toForm(config) {
  const form = structuredClone(config);
  for (const group of GROUPS) {
    for (const f of group.fields) {
      if (f.type === "datetime") form[group.key][f.name] = toLocalInput(config[group.key][f.name]);
      if (f.type === "number" && form[group.key][f.name] == null) form[group.key][f.name] = "";
    }
  }
  return form;
}

export default function ExamSettings({ orgId, exam, config, editable }) {
  return (
    <Stack spacing={3}>
      <DetailsForm orgId={orgId} exam={exam} editable={editable} />
      <RulesForm orgId={orgId} examId={exam.id} config={config} editable={editable} />
    </Stack>
  );
}

function DetailsForm({ orgId, exam, editable }) {
  const { showSuccess } = useToast();
  const [form, setForm] = useState({
    title: exam.title ?? "",
    code: exam.code ?? "",
    description: exam.description ?? "",
    instructions: exam.instructions ?? "",
    passing_marks: exam.passing_marks ?? "",
  });
  const [error, setError] = useState(null);
  const save = useExamMutation(orgId, exam.id, (patch) => examsApi.update(orgId, exam.id, patch));
  const set = (key) => (e) => setForm((f) => ({ ...f, [key]: e.target.value }));

  const submit = async (e) => {
    e.preventDefault();
    setError(null);
    try {
      await save.mutateAsync({
        title: form.title,
        code: form.code.trim() || null,
        description: form.description.trim() || null,
        instructions: form.instructions.trim() || null,
        passing_marks: form.passing_marks === "" ? null : Number(form.passing_marks),
      });
      showSuccess("Exam details saved.");
    } catch (err) {
      setError(apiError(err, "Could not save the exam details."));
    }
  };

  return (
    <Panel title="Details">
      <Box component="form" onSubmit={submit} sx={{ p: 3 }}>
        <FormErrors error={error} />
        <Stack spacing={2}>
          <Stack direction={{ xs: "column", sm: "row" }} spacing={2}>
            <TextField label="Title" value={form.title} onChange={set("title")} required fullWidth disabled={!editable} error={Boolean(error?.fields?.title)} />
            <TextField label="Code" value={form.code} onChange={set("code")} sx={{ minWidth: 220 }} disabled={!editable} error={Boolean(error?.fields?.code)} />
          </Stack>
          <TextField label="Description" value={form.description} onChange={set("description")} multiline minRows={2} disabled={!editable} />
          <TextField label="Instructions shown to candidates" value={form.instructions} onChange={set("instructions")} multiline minRows={4} disabled={!editable} />
          <TextField
            label="Passing marks"
            type="number"
            value={form.passing_marks}
            onChange={set("passing_marks")}
            sx={{ maxWidth: 220 }}
            disabled={!editable}
            helperText="Optional. Otherwise the passing percentage applies."
          />
        </Stack>
        {editable && (
          <Box sx={{ mt: 3 }}>
            <Button type="submit" variant="contained" disabled={save.isPending}>
              {save.isPending ? <CircularProgress size={18} color="inherit" /> : "Save details"}
            </Button>
          </Box>
        )}
      </Box>
    </Panel>
  );
}

function RulesForm({ orgId, examId, config, editable }) {
  const { showSuccess } = useToast();
  const [form, setForm] = useState(() => toForm(config));
  const [error, setError] = useState(null);
  const [warnings, setWarnings] = useState([]);
  const save = useExamMutation(orgId, examId, (patch) => examsApi.updateConfig(orgId, examId, patch));

  const setField = (group, name, value) => setForm((f) => ({ ...f, [group]: { ...f[group], [name]: value } }));

  const submit = async (e) => {
    e.preventDefault();
    setError(null);
    try {
      const res = await save.mutateAsync(toPatch(form));
      setWarnings(res?.warnings ?? []);
      showSuccess("Exam rules saved.");
    } catch (err) {
      setError(apiError(err, "Could not save the exam rules."));
    }
  };

  return (
    <Panel title="Rules" subtitle="How the exam runs for candidates">
      <Box component="form" onSubmit={submit} sx={{ p: 3 }}>
        <FormErrors error={error} />
        {warnings.length > 0 && (
          <Box sx={{ mb: 2, p: 1.5, border: "1px solid #e4e4e7", borderRadius: 1.5 }}>
            <Typography sx={{ fontWeight: 600, fontSize: "0.8125rem" }}>Saved with warnings</Typography>
            <Box component="ul" sx={{ m: 0, mt: 0.5, pl: 2.5 }}>
              {warnings.map((w) => (
                <Typography component="li" variant="body2" key={w.path + w.code}>
                  {w.message}
                </Typography>
              ))}
            </Box>
          </Box>
        )}

        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" }, gap: 4 }}>
          {GROUPS.map((group) => (
            <Box key={group.key}>
              <Typography sx={{ fontWeight: 600, mb: 1.5 }}>{group.title}</Typography>
              <Stack spacing={1.5}>
                {group.fields
                  .filter((f) => !f.show || f.show(form))
                  .map((f) => (
                    <ConfigField
                      key={f.name}
                      field={f}
                      value={form[group.key][f.name]}
                      error={error?.fields?.[`${group.key}.${f.name}`]}
                      disabled={!editable}
                      onChange={(v) => setField(group.key, f.name, v)}
                    />
                  ))}
              </Stack>
            </Box>
          ))}
        </Box>

        {editable && (
          <Box sx={{ mt: 3 }}>
            <Button type="submit" variant="contained" disabled={save.isPending}>
              {save.isPending ? <CircularProgress size={18} color="inherit" /> : "Save rules"}
            </Button>
          </Box>
        )}
      </Box>
    </Panel>
  );
}

function ConfigField({ field, value, error, disabled, onChange }) {
  if (field.type === "switch") {
    return (
      <Box>
        <FormControlLabel
          control={<Switch size="small" checked={Boolean(value)} onChange={(e) => onChange(e.target.checked)} disabled={disabled} />}
          label={field.label}
          sx={{ "& .MuiFormControlLabel-label": { fontSize: "0.875rem" } }}
        />
        {error && (
          <Typography variant="body2" sx={{ color: "text.primary", fontWeight: 500 }}>
            {error}
          </Typography>
        )}
      </Box>
    );
  }
  const common = { label: field.label, size: "small", fullWidth: true, disabled, error: Boolean(error), helperText: error };
  if (field.type === "select") {
    return (
      <TextField select value={value ?? ""} onChange={(e) => onChange(e.target.value)} {...common}>
        {field.options.map((o) => (
          <MenuItem key={o.value} value={o.value}>
            {o.label}
          </MenuItem>
        ))}
      </TextField>
    );
  }
  if (field.type === "datetime") {
    return <TextField type="datetime-local" value={value ?? ""} onChange={(e) => onChange(e.target.value)} slotProps={{ inputLabel: { shrink: true } }} {...common} />;
  }
  return <TextField type="number" value={value ?? ""} onChange={(e) => onChange(e.target.value)} slotProps={{ htmlInput: { min: 0 } }} {...common} />;
}
