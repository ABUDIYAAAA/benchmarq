"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import MenuItem from "@mui/material/MenuItem";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import { FormErrors, dialogPaper, fromLocalInput } from "@/components/exams/common";
import { apiError, useCreateExam } from "@/hooks/api/useExams";

const EMPTY = { title: "", code: "", description: "", schedule_type: "open_ended", start_time: "", end_time: "", duration: 60 };

export default function CreateExamDialog({ open, onClose, orgId }) {
  const router = useRouter();
  const [form, setForm] = useState(EMPTY);
  const [error, setError] = useState(null);
  const create = useCreateExam(orgId);
  const windowed = form.schedule_type !== "open_ended";

  const set = (key) => (e) => setForm((f) => ({ ...f, [key]: e.target.value }));

  const submit = async (e) => {
    e.preventDefault();
    setError(null);
    try {
      const detail = await create.mutateAsync({
        title: form.title.trim(),
        code: form.code.trim() || null,
        description: form.description.trim() || null,
        config: {
          schedule: {
            schedule_type: form.schedule_type,
            total_duration_minutes: Number(form.duration),
            start_time: windowed ? fromLocalInput(form.start_time) : null,
            end_time: windowed ? fromLocalInput(form.end_time) : null,
          },
        },
      });
      setForm(EMPTY);
      onClose();
      router.push(`/dashboard/exams/${detail.exam.id}`);
    } catch (err) {
      setError(apiError(err, "Could not create the exam."));
    }
  };

  return (
    <Dialog open={open} onClose={() => !create.isPending && onClose()} maxWidth="sm" fullWidth slotProps={dialogPaper}>
      <form onSubmit={submit}>
        <DialogTitle sx={{ fontSize: "1.125rem", fontWeight: 600 }}>New exam</DialogTitle>
        <DialogContent>
          <FormErrors error={error} />
          <Stack spacing={2} sx={{ pt: 1 }}>
            <TextField label="Title" value={form.title} onChange={set("title")} required autoFocus />
            <TextField label="Code" value={form.code} onChange={set("code")} helperText="Optional short reference, e.g. PLACEMENT-2026" />
            <TextField label="Description" value={form.description} onChange={set("description")} multiline minRows={2} />
            <TextField select label="Schedule" value={form.schedule_type} onChange={set("schedule_type")}>
              <MenuItem value="open_ended">Open ended — candidates start any time</MenuItem>
              <MenuItem value="flexible_window">Flexible window — start any time within a window</MenuItem>
              <MenuItem value="fixed_window">Fixed window — everyone starts together</MenuItem>
            </TextField>
            {windowed && (
              <Stack direction="row" spacing={2}>
                <TextField label="Opens" type="datetime-local" value={form.start_time} onChange={set("start_time")} fullWidth slotProps={{ inputLabel: { shrink: true } }} />
                <TextField label="Closes" type="datetime-local" value={form.end_time} onChange={set("end_time")} fullWidth slotProps={{ inputLabel: { shrink: true } }} />
              </Stack>
            )}
            <TextField label="Duration (minutes)" type="number" value={form.duration} onChange={set("duration")} required slotProps={{ htmlInput: { min: 1, max: 1440 } }} />
          </Stack>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button variant="outlined" onClick={onClose} disabled={create.isPending}>
            Cancel
          </Button>
          <Button type="submit" variant="contained" disabled={create.isPending || !form.title.trim()}>
            {create.isPending ? <CircularProgress size={18} color="inherit" /> : "Create exam"}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
