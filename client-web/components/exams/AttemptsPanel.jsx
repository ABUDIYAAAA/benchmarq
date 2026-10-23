"use client";

import { useState } from "react";
import Button from "@mui/material/Button";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import Typography from "@mui/material/Typography";
import { ConfirmDialog, EmptyState, Loading, Panel, formatDateTime } from "@/components/exams/common";
import { useToast } from "@/context/ToastContext";
import { apiError, useAttempts, useExamMutation } from "@/hooks/api/useExams";
import examsApi from "@/lib/api/exams.api";

const STATUS_LABEL = {
  in_progress: "In progress",
  submitted: "Submitted",
  auto_submitted: "Auto-submitted",
  disqualified: "Disqualified",
  abandoned: "Abandoned",
  evaluated: "Graded",
};

/** Live view of attempts, refreshed every 30 seconds. */
export default function AttemptsPanel({ orgId, examId }) {
  const { showSuccess, showError } = useToast();
  const { data, isLoading } = useAttempts(orgId, examId);
  const [releasing, setReleasing] = useState(null);
  const release = useExamMutation(orgId, examId, (id) => examsApi.releaseDevice(orgId, examId, id));
  const attempts = data?.items ?? [];

  return (
    <Panel title="Attempts" subtitle="Updates every 30 seconds">
      {isLoading ? (
        <Loading />
      ) : attempts.length === 0 ? (
        <EmptyState title="No attempts yet" />
      ) : (
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Candidate</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Started</TableCell>
              <TableCell>Submitted</TableCell>
              <TableCell>Last seen</TableCell>
              <TableCell align="right">Score</TableCell>
              <TableCell align="right" />
            </TableRow>
          </TableHead>
          <TableBody>
            {attempts.map((a) => (
              <TableRow key={a.id} hover>
                <TableCell>
                  <Typography sx={{ fontSize: "0.875rem" }}>{[a.first_name, a.last_name].filter(Boolean).join(" ")}</Typography>
                  <Typography variant="body2">{a.email}</Typography>
                </TableCell>
                <TableCell>{STATUS_LABEL[a.status] ?? a.status}</TableCell>
                <TableCell>
                  <Typography variant="body2">{formatDateTime(a.started_at)}</Typography>
                </TableCell>
                <TableCell>
                  <Typography variant="body2">{formatDateTime(a.submitted_at)}</Typography>
                </TableCell>
                <TableCell>
                  <Typography variant="body2">{formatDateTime(a.last_heartbeat_at)}</Typography>
                </TableCell>
                <TableCell align="right">{a.total_score != null ? `${a.total_score}${a.max_score != null ? ` / ${a.max_score}` : ""}` : "—"}</TableCell>
                <TableCell align="right">
                  {a.status === "in_progress" && a.device_bound && (
                    <Button size="small" onClick={() => setReleasing(a)}>
                      Release device
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <ConfirmDialog
        open={Boolean(releasing)}
        title="Release device?"
        confirmLabel="Release"
        busy={release.isPending}
        onClose={() => setReleasing(null)}
        onConfirm={async () => {
          try {
            await release.mutateAsync(releasing.id);
            showSuccess("Device released. The candidate can continue on another computer.");
          } catch (err) {
            showError(apiError(err).message);
          }
          setReleasing(null);
        }}
      >
        Use this when a candidate&apos;s computer failed. Their attempt continues on whichever computer launches the exam next; the timer keeps running.
      </ConfirmDialog>
    </Panel>
  );
}
