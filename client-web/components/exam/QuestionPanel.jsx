"use client";

import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import FormControlLabel from "@mui/material/FormControlLabel";
import Radio from "@mui/material/Radio";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

const TEXT_SAVE_DELAY_MS = 900;

const countWords = (s) => (s.trim() ? s.trim().split(/\s+/).length : 0);

/**
 * Renders one question and saves answers as the candidate works. The parent
 * keys it by question, so local state starts fresh for every question. Choice
 * answers save immediately; typed answers are debounced and flushed before
 * any navigation (the parent calls flush() through the ref).
 */
const QuestionPanel = forwardRef(function QuestionPanel({ question, rules, onSave, onAutoAdvance }, ref) {
  const [text, setText] = useState(question.response?.text ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const [response, setResponse] = useState(question.response ?? null);
  const timer = useRef(null);
  const pending = useRef(null);

  useEffect(() => () => clearTimeout(timer.current), []);

  const save = useCallback(
    async (input) => {
      setSaving(true);
      setError(null);
      try {
        const saved = await onSave({ question_id: question.question_id, ...input });
        setResponse(saved ?? null);
        return true;
      } catch (err) {
        setError(err.message);
        return false;
      } finally {
        setSaving(false);
      }
    },
    [onSave, question.question_id],
  );

  const flush = useCallback(async () => {
    clearTimeout(timer.current);
    if (pending.current === null) return true;
    const value = pending.current;
    pending.current = null;
    return save(value === "" ? { clear: true } : { text: value });
  }, [save]);

  useImperativeHandle(ref, () => ({ flush }), [flush]);

  const onText = (value) => {
    setText(value);
    pending.current = value;
    clearTimeout(timer.current);
    timer.current = setTimeout(flush, TEXT_SAVE_DELAY_MS);
  };

  const choose = async (input) => {
    const ok = await save(input);
    if (ok && rules?.auto_advance_on_selection && !question.is_last) onAutoAdvance?.();
  };

  const selected = new Set(response?.selected_option_ids ?? []);
  const maxChoices = question.multi_select?.max_choices;
  const limits = question.short_answer ?? question.essay ?? null;
  const words = countWords(text);
  const hasAnswer = Boolean(response && (response.selected_option_id || response.selected_option_ids?.length || response.text || response.boolean != null));

  const details = [`${question.points} ${question.points === 1 ? "mark" : "marks"}`];
  if (question.penalty > 0) details.push(`−${question.penalty} for a wrong answer`);
  if (question.type === "multi_select") details.push(`select ${question.multi_select.min_choices}${maxChoices ? `–${maxChoices}` : " or more"}`);

  return (
    <Box>
      <Stack direction="row" sx={{ justifyContent: "space-between", alignItems: "baseline" }}>
        <Box>
          <Typography variant="h3">Question {question.position + 1}</Typography>
          <Typography variant="body2" sx={{ mt: 0.5 }}>
            {details.join(" · ")}
          </Typography>
        </Box>
        <Typography variant="body2">
          {saving ? "Saving…" : response?.saved_at ? `Saved at ${new Date(response.saved_at).toLocaleTimeString()}` : "Not answered"}
        </Typography>
      </Stack>

      <Typography sx={{ mt: 3, fontSize: "1rem", lineHeight: 1.75, whiteSpace: "pre-wrap" }}>{question.prompt}</Typography>

      <Box sx={{ mt: 3 }}>
        {question.type === "multiple_choice" && (
          <Stack spacing={1}>
            {question.options.map((o) => (
              <OptionRow
                key={o.id}
                label={o.content}
                checked={response?.selected_option_id === o.id}
                control={<Radio size="small" />}
                onChange={() => choose({ selected_option_id: o.id })}
              />
            ))}
          </Stack>
        )}

        {question.type === "true_false" && (
          <Stack spacing={1} sx={{ maxWidth: 360 }}>
            {[true, false].map((v) => (
              <OptionRow key={String(v)} label={v ? "True" : "False"} checked={response?.boolean === v} control={<Radio size="small" />} onChange={() => choose({ boolean: v })} />
            ))}
          </Stack>
        )}

        {question.type === "multi_select" && (
          <Stack spacing={1}>
            {question.options.map((o) => {
              const active = selected.has(o.id);
              return (
                <OptionRow
                  key={o.id}
                  label={o.content}
                  checked={active}
                  disabled={Boolean(maxChoices) && selected.size >= maxChoices && !active}
                  control={<Checkbox size="small" />}
                  onChange={() => {
                    const next = new Set(selected);
                    if (active) next.delete(o.id);
                    else next.add(o.id);
                    save(next.size ? { selected_option_ids: [...next] } : { clear: true });
                  }}
                />
              );
            })}
          </Stack>
        )}

        {question.type === "blank_fill" && (
          <TextField
            placeholder="Type your answer"
            value={text}
            onChange={(e) => onText(e.target.value)}
            onBlur={flush}
            autoComplete="off"
            sx={{ width: "100%", maxWidth: 480 }}
            slotProps={{ htmlInput: { spellCheck: false } }}
          />
        )}

        {(question.type === "short_answer" || question.type === "essay") && (
          <Box>
            <TextField
              multiline
              fullWidth
              minRows={question.type === "essay" ? 12 : 5}
              placeholder="Write your answer"
              value={text}
              onChange={(e) => onText(e.target.value)}
              onBlur={flush}
              slotProps={{ htmlInput: { maxLength: limits?.max_length ?? undefined, spellCheck: false } }}
            />
            <Typography variant="body2" sx={{ mt: 1, textAlign: "right" }}>
              {words} words
              {limits?.min_words ? ` · minimum ${limits.min_words}` : ""}
              {limits?.max_words ? ` · maximum ${limits.max_words}` : ""}
              {limits?.max_length ? ` · ${text.length}/${limits.max_length} characters` : ""}
            </Typography>
          </Box>
        )}
      </Box>

      {error && (
        <Typography sx={{ mt: 2, fontWeight: 500 }} role="alert">
          {error}
        </Typography>
      )}

      <Stack direction="row" spacing={1} sx={{ mt: 3 }}>
        {rules?.allow_bookmark_for_review && (
          <Button variant="outlined" onClick={() => save({ flagged: !response?.flagged })}>
            {response?.flagged ? "Unmark review" : "Mark for review"}
          </Button>
        )}
        {hasAnswer && (
          <Button
            color="secondary"
            onClick={() => {
              pending.current = null;
              setText("");
              save({ clear: true });
            }}
          >
            Clear response
          </Button>
        )}
      </Stack>
    </Box>
  );
});

export default QuestionPanel;

function OptionRow({ label, checked, disabled, control, onChange }) {
  return (
    <FormControlLabel
      checked={checked}
      disabled={disabled}
      onChange={onChange}
      control={control}
      label={label}
      sx={{
        m: 0,
        px: 1.5,
        py: 0.75,
        border: "1px solid",
        borderColor: checked ? "text.primary" : "divider",
        borderRadius: 1.5,
        "& .MuiFormControlLabel-label": { whiteSpace: "pre-wrap", fontSize: "0.9375rem" },
        "&:hover": { borderColor: disabled ? "divider" : "text.primary" },
      }}
    />
  );
}
