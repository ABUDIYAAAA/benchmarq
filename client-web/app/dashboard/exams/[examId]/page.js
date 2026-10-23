"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Container from "@mui/material/Container";
import Tab from "@mui/material/Tab";
import Tabs from "@mui/material/Tabs";
import Typography from "@mui/material/Typography";
import AttemptsPanel from "@/components/exams/AttemptsPanel";
import CandidatesPanel from "@/components/exams/CandidatesPanel";
import ExamSettings from "@/components/exams/ExamSettings";
import SectionsEditor from "@/components/exams/SectionsEditor";
import { ConfirmDialog, EmptyState, Loading, PageHeader, Panel, StatusChip } from "@/components/exams/common";
import { useOrg } from "@/context/OrgContext";
import { useToast } from "@/context/ToastContext";
import { apiError, useExam, useExamMutation, useReadiness } from "@/hooks/api/useExams";
import examsApi from "@/lib/api/exams.api";
import { can } from "@/lib/permissions";

const ACTIONS = {
  publish: { label: "Publish", title: "Publish exam?", body: "Published exams can no longer be edited. Assigned candidates can launch it once it opens.", done: "Exam published." },
  unpublish: { label: "Unpublish", title: "Return to draft?", body: "Candidates cannot launch the exam while it is a draft. This is only possible before anyone has attempted it.", done: "Exam returned to draft." },
  archive: { label: "Archive", title: "Archive exam?", body: "Archived exams are closed to candidates. Results and attempts are kept.", done: "Exam archived." },
  restore: { label: "Restore", title: "Restore to draft?", body: "The exam becomes an editable draft again. Only possible if nobody has attempted it.", done: "Exam restored to draft." },
};

export default function ExamEditorPage() {
  const { examId } = useParams();
  const router = useRouter();
  const { currentOrg } = useOrg();
  const { showSuccess, showError } = useToast();
  const [tab, setTab] = useState("settings");
  const [pending, setPending] = useState(null); // action key or "delete"

  const orgId = currentOrg?.id;
  const role = currentOrg?.my_role;
  const { data, isLoading, error } = useExam(can(role, "viewExams") ? orgId : null, examId);
  const exam = data?.exam;
  const isDraft = exam?.status === "draft";
  const { data: readiness } = useReadiness(orgId, examId, Boolean(isDraft));

  const transition = useExamMutation(orgId, examId, (action) => examsApi.transition(orgId, examId, action));
  const remove = useExamMutation(orgId, examId, () => examsApi.remove(orgId, examId));

  if (!currentOrg || isLoading) return <Loading />;
  if (!can(role, "viewExams") || error || !exam) {
    return (
      <Container maxWidth="lg" sx={{ py: 4 }}>
        <EmptyState title="Exam not found" action={<Button variant="outlined" component={Link} href="/dashboard/exams">Back to exams</Button>}>
          It may have been deleted, or it belongs to another organization.
        </EmptyState>
      </Container>
    );
  }

  const editable = isDraft && can(role, "authorExams");
  const canPublish = can(role, "publishExams");

  const actions = [];
  if (canPublish) {
    if (exam.status === "draft") actions.push("publish");
    if (exam.status === "published") actions.push("unpublish", "archive");
    if (exam.status === "archived") actions.push("restore");
  }

  const run = async () => {
    const key = pending;
    try {
      if (key === "delete") {
        await remove.mutateAsync();
        showSuccess("Exam deleted.");
        router.push("/dashboard/exams");
      } else {
        await transition.mutateAsync(key);
        showSuccess(ACTIONS[key].done);
      }
    } catch (err) {
      showError(apiError(err).message);
    }
    setPending(null);
  };

  return (
    <Container maxWidth="lg" sx={{ py: 4, px: { xs: 2.5, sm: 4 } }}>
      <Typography variant="body2" sx={{ mb: 1 }}>
        <Link href="/dashboard/exams" style={{ color: "inherit" }}>
          Exams
        </Link>{" "}
        / {exam.title}
      </Typography>
      <PageHeader
        title={
          <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 1.5 }}>
            {exam.title} <StatusChip status={exam.status} />
          </Box>
        }
        subtitle={`${data.sections.length} sections · ${data.sections.reduce((n, s) => n + s.question_count, 0)} questions · ${exam.total_marks} marks · ${data.config.schedule.total_duration_minutes} minutes`}
        actions={
          <>
            {actions.map((key) => (
              <Button
                key={key}
                variant={key === "publish" ? "contained" : "outlined"}
                disabled={key === "publish" && !readiness?.ready}
                onClick={() => setPending(key)}
              >
                {ACTIONS[key].label}
              </Button>
            ))}
            {can(role, "deleteExams") && exam.status !== "published" && (
              <Button color="secondary" onClick={() => setPending("delete")}>
                Delete
              </Button>
            )}
          </>
        }
      />

      {isDraft && readiness && <ReadinessPanel readiness={readiness} />}
      {!isDraft && exam.status === "published" && (
        <Typography variant="body2" sx={{ mb: 3 }}>
          Published exams cannot be edited. Unpublish it to make changes; that is only possible before any candidate has started.
        </Typography>
      )}

      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 3, borderBottom: "1px solid #e4e4e7" }}>
        <Tab value="settings" label="Settings" />
        <Tab value="sections" label="Sections and questions" />
        <Tab value="candidates" label="Candidates" />
        {can(role, "monitorAttempts") && exam.status !== "draft" && <Tab value="attempts" label="Attempts" />}
      </Tabs>

      {tab === "settings" && <ExamSettings key={`${exam.updated_at}`} orgId={orgId} exam={exam} config={data.config} editable={editable} />}
      {tab === "sections" && <SectionsEditor orgId={orgId} exam={exam} config={data.config} sections={data.sections} editable={editable} />}
      {tab === "candidates" && <CandidatesPanel orgId={orgId} exam={exam} canAssign={can(role, "assignExams")} />}
      {tab === "attempts" && <AttemptsPanel orgId={orgId} examId={examId} />}

      <ConfirmDialog
        open={Boolean(pending)}
        title={pending === "delete" ? "Delete exam?" : ACTIONS[pending]?.title}
        confirmLabel={pending === "delete" ? "Delete" : ACTIONS[pending]?.label}
        busy={transition.isPending || remove.isPending}
        onClose={() => setPending(null)}
        onConfirm={run}
      >
        {pending === "delete" ? "The exam and its questions will be removed. Exams with attempts must be archived instead." : ACTIONS[pending]?.body}
      </ConfirmDialog>
    </Container>
  );
}

function ReadinessPanel({ readiness }) {
  const { errors = [], warnings = [] } = readiness;
  return (
    <Panel
      title={readiness.ready ? "Ready to publish" : `${errors.length} issue${errors.length === 1 ? "" : "s"} to fix before publishing`}
      subtitle={`${readiness.total_marks} marks · ${readiness.served_questions} of ${readiness.total_questions} questions served per candidate`}
      sx={{ mb: 3 }}
    >
      {(errors.length > 0 || warnings.length > 0) && (
        <Box sx={{ px: 3, py: 2 }}>
          {errors.length > 0 && (
            <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
              {errors.map((e) => (
                <Typography component="li" key={e.path + e.code} sx={{ fontSize: "0.875rem", fontWeight: 500 }}>
                  {e.message}
                </Typography>
              ))}
            </Box>
          )}
          {warnings.length > 0 && (
            <>
              <Typography variant="body2" sx={{ mt: errors.length ? 1.5 : 0, fontWeight: 600 }}>
                Warnings
              </Typography>
              <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
                {warnings.map((w) => (
                  <Typography component="li" variant="body2" key={w.path + w.code}>
                    {w.message}
                  </Typography>
                ))}
              </Box>
            </>
          )}
        </Box>
      )}
    </Panel>
  );
}
