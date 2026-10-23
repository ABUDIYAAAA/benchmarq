"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import List from "@mui/material/List";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import Typography from "@mui/material/Typography";
import QuestionDialog, { QUESTION_TYPES } from "@/components/exams/QuestionDialog";
import SectionDialog from "@/components/exams/SectionDialog";
import { ConfirmDialog, EmptyState, Loading, Panel } from "@/components/exams/common";
import { useToast } from "@/context/ToastContext";
import { apiError, useExamMutation, useQuestions } from "@/hooks/api/useExams";
import examsApi from "@/lib/api/exams.api";

const move = (list, from, to) => {
  const next = [...list];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next;
};

export default function SectionsEditor({ orgId, exam, config, sections, editable }) {
  const { showSuccess, showError } = useToast();
  const [selectedId, setSelectedId] = useState(sections[0]?.id ?? null);
  const [sectionDialog, setSectionDialog] = useState(null); // { section? }
  const [deleting, setDeleting] = useState(null);

  const examId = exam.id;
  const selected = sections.find((s) => s.id === selectedId) ?? sections[0] ?? null;

  const createSection = useExamMutation(orgId, examId, (body) => examsApi.createSection(orgId, examId, body));
  const updateSection = useExamMutation(orgId, examId, ({ id, body }) => examsApi.updateSection(orgId, examId, id, body));
  const deleteSection = useExamMutation(orgId, examId, (id) => examsApi.deleteSection(orgId, examId, id));
  const reorder = useExamMutation(orgId, examId, (ids) => examsApi.reorderSections(orgId, examId, ids));

  const saveSection = async (body) => {
    if (sectionDialog?.section) {
      await updateSection.mutateAsync({ id: sectionDialog.section.id, body });
    } else {
      const created = await createSection.mutateAsync(body);
      setSelectedId(created.id);
    }
    showSuccess("Section saved.");
  };

  const moveSection = (index, delta) =>
    reorder.mutateAsync(move(sections, index, index + delta).map((s) => s.id)).catch((err) => showError(apiError(err).message));

  return (
    <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "300px 1fr" }, gap: 3, alignItems: "start" }}>
      <Panel
        title="Sections"
        actions={
          editable && (
            <Button size="small" variant="outlined" onClick={() => setSectionDialog({})}>
              Add
            </Button>
          )
        }
      >
        {sections.length === 0 ? (
          <EmptyState title="No sections">Sections group questions and carry their own timing and unlock rules.</EmptyState>
        ) : (
          <List disablePadding>
            {sections.map((s, i) => (
              <ListItemButton
                key={s.id}
                selected={s.id === selected?.id}
                onClick={() => setSelectedId(s.id)}
                sx={{ borderLeft: s.id === selected?.id ? "2px solid #09090b" : "2px solid transparent", alignItems: "flex-start" }}
              >
                <ListItemText
                  primary={`${i + 1}. ${s.name}`}
                  secondary={`${s.section_type === "objective" ? "Objective" : "Subjective"} · ${s.question_count} questions · ${s.max_marks} marks`}
                  slotProps={{ primary: { sx: { fontSize: "0.875rem", fontWeight: 500 } }, secondary: { sx: { fontSize: "0.75rem" } } }}
                />
                {editable && (
                  <Box sx={{ display: "flex", flexDirection: "column" }} onClick={(e) => e.stopPropagation()}>
                    <Button size="small" sx={{ minWidth: 0, px: 1, py: 0, fontSize: "0.75rem" }} disabled={i === 0} onClick={() => moveSection(i, -1)}>
                      Up
                    </Button>
                    <Button size="small" sx={{ minWidth: 0, px: 1, py: 0, fontSize: "0.75rem" }} disabled={i === sections.length - 1} onClick={() => moveSection(i, 1)}>
                      Down
                    </Button>
                  </Box>
                )}
              </ListItemButton>
            ))}
          </List>
        )}
      </Panel>

      {selected ? (
        <SectionQuestions
          key={selected.id}
          orgId={orgId}
          examId={examId}
          section={selected}
          editable={editable}
          onEdit={() => setSectionDialog({ section: selected })}
          onDelete={() => setDeleting(selected)}
        />
      ) : (
        <Panel>
          <EmptyState title="Add a section to start adding questions" />
        </Panel>
      )}

      {sectionDialog && (
        <SectionDialog
          open
          section={sectionDialog.section}
          sections={sections}
          config={config}
          onSave={saveSection}
          onClose={() => setSectionDialog(null)}
        />
      )}
      <ConfirmDialog
        open={Boolean(deleting)}
        title="Delete section?"
        confirmLabel="Delete"
        busy={deleteSection.isPending}
        onClose={() => setDeleting(null)}
        onConfirm={async () => {
          try {
            await deleteSection.mutateAsync(deleting.id);
            showSuccess("Section deleted.");
            setSelectedId(null);
          } catch (err) {
            showError(apiError(err).message);
          }
          setDeleting(null);
        }}
      >
        {deleting?.name} and its {deleting?.question_count} question(s) will be removed.
      </ConfirmDialog>
    </Box>
  );
}

function SectionQuestions({ orgId, examId, section, editable, onEdit, onDelete }) {
  const { showSuccess, showError } = useToast();
  const { data: questions = [], isLoading } = useQuestions(orgId, examId, section.id);
  const [dialog, setDialog] = useState(null); // { question? }
  const [deleting, setDeleting] = useState(null);

  const create = useExamMutation(orgId, examId, (body) => examsApi.createQuestion(orgId, examId, section.id, body));
  const replace = useExamMutation(orgId, examId, ({ id, body }) => examsApi.replaceQuestion(orgId, examId, section.id, id, body));
  const remove = useExamMutation(orgId, examId, (id) => examsApi.deleteQuestion(orgId, examId, section.id, id));
  const reorder = useExamMutation(orgId, examId, (ids) => examsApi.reorderQuestions(orgId, examId, section.id, ids));

  const save = async (body) => {
    if (dialog?.question) await replace.mutateAsync({ id: dialog.question.id, body });
    else await create.mutateAsync(body);
    showSuccess("Question saved.");
  };

  const eff = section.effective ?? {};
  const facts = [
    eff.duration_minutes ? `${eff.duration_minutes} min limit` : "Shares the exam timer",
    eff.question_pick_count ? `${eff.question_pick_count} drawn per candidate` : null,
    eff.shuffle_questions ? "Questions shuffled" : null,
    eff.shuffle_options ? "Options shuffled" : null,
    eff.lock_on_complete ? "Locks when left" : null,
    eff.locked_at_start ? "Locked at start" : null,
  ].filter(Boolean);

  return (
    <Panel
      title={section.name}
      subtitle={facts.join(" · ")}
      actions={
        editable && (
          <>
            <Button size="small" variant="outlined" onClick={onEdit}>
              Edit section
            </Button>
            <Button size="small" color="secondary" onClick={onDelete}>
              Delete
            </Button>
          </>
        )
      }
    >
      {isLoading ? (
        <Loading />
      ) : questions.length === 0 ? (
        <EmptyState
          title="No questions in this section"
          action={
            editable && (
              <Button variant="outlined" onClick={() => setDialog({})}>
                Add question
              </Button>
            )
          }
        />
      ) : (
        <>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell width={40}>#</TableCell>
                <TableCell>Question</TableCell>
                <TableCell>Type</TableCell>
                <TableCell align="right">Marks</TableCell>
                {editable && <TableCell align="right">Actions</TableCell>}
              </TableRow>
            </TableHead>
            <TableBody>
              {questions.map((q, i) => (
                <TableRow key={q.id} hover>
                  <TableCell>{i + 1}</TableCell>
                  <TableCell sx={{ maxWidth: 420 }}>
                    <Typography noWrap sx={{ fontSize: "0.875rem" }}>
                      {q.prompt}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2">{QUESTION_TYPES[q.type] ?? q.type}</Typography>
                  </TableCell>
                  <TableCell align="right">{q.points}</TableCell>
                  {editable && (
                    <TableCell align="right" sx={{ whiteSpace: "nowrap" }}>
                      <Button
                        size="small"
                        disabled={i === 0}
                        onClick={() => reorder.mutateAsync(move(questions, i, i - 1).map((x) => x.id)).catch((err) => showError(apiError(err).message))}
                      >
                        Up
                      </Button>
                      <Button
                        size="small"
                        disabled={i === questions.length - 1}
                        onClick={() => reorder.mutateAsync(move(questions, i, i + 1).map((x) => x.id)).catch((err) => showError(apiError(err).message))}
                      >
                        Down
                      </Button>
                      <Button size="small" onClick={() => setDialog({ question: q })}>
                        Edit
                      </Button>
                      <Button size="small" color="secondary" onClick={() => setDeleting(q)}>
                        Delete
                      </Button>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {editable && (
            <Box sx={{ p: 2, borderTop: "1px solid #e4e4e7" }}>
              <Button variant="outlined" size="small" onClick={() => setDialog({})}>
                Add question
              </Button>
            </Box>
          )}
        </>
      )}

      {dialog && (
        <QuestionDialog open sectionType={section.section_type} question={dialog.question} onSave={save} onClose={() => setDialog(null)} />
      )}
      <ConfirmDialog
        open={Boolean(deleting)}
        title="Delete question?"
        confirmLabel="Delete"
        busy={remove.isPending}
        onClose={() => setDeleting(null)}
        onConfirm={async () => {
          try {
            await remove.mutateAsync(deleting.id);
            showSuccess("Question deleted.");
          } catch (err) {
            showError(apiError(err).message);
          }
          setDeleting(null);
        }}
      >
        This question will be removed from {section.name}.
      </ConfirmDialog>
    </Panel>
  );
}
