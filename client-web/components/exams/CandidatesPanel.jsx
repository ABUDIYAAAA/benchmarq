"use client";

import { useMemo, useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import FormControlLabel from "@mui/material/FormControlLabel";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { ConfirmDialog, EmptyState, FormErrors, Loading, Panel, dialogPaper, formatDateTime } from "@/components/exams/common";
import { useToast } from "@/context/ToastContext";
import { apiError, useAssignments, useExamMutation, useOrgMembersForAssign } from "@/hooks/api/useExams";
import examsApi from "@/lib/api/exams.api";
import { ROLE_LABELS } from "@/lib/permissions";

const fullName = (p) => [p.first_name, p.last_name].filter(Boolean).join(" ");

/**
 * Candidates assigned to the exam. Anyone in the organization can be
 * assigned; people outside it are first invited from Users & Team (as
 * candidates), then assigned here.
 */
export default function CandidatesPanel({ orgId, exam, canAssign }) {
  const { showSuccess, showError } = useToast();
  const { data, isLoading } = useAssignments(orgId, exam.id);
  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState(null);
  const unassign = useExamMutation(orgId, exam.id, (id) => examsApi.unassign(orgId, exam.id, id));
  const resend = useExamMutation(orgId, exam.id, (userIds) => examsApi.assign(orgId, exam.id, { user_ids: userIds, send_email: true }));

  const assignments = data?.items ?? [];
  const published = exam.status === "published";

  return (
    <Panel
      title="Candidates"
      subtitle={`${assignments.length} assigned`}
      actions={
        canAssign && (
          <>
            {published && assignments.length > 0 && (
              <Button
                size="small"
                variant="outlined"
                disabled={resend.isPending}
                onClick={async () => {
                  try {
                    const res = await resend.mutateAsync(assignments.map((a) => a.user_id));
                    showSuccess(`Invitation emailed to ${res.emailed} candidate(s).`);
                  } catch (err) {
                    showError(apiError(err).message);
                  }
                }}
              >
                Email everyone
              </Button>
            )}
            <Button size="small" variant="contained" onClick={() => setAdding(true)}>
              Assign candidates
            </Button>
          </>
        )
      }
    >
      {isLoading ? (
        <Loading />
      ) : assignments.length === 0 ? (
        <EmptyState title="No candidates assigned">
          Assign members of your organization. To add someone new, invite them as a candidate from Users &amp; Team first.
        </EmptyState>
      ) : (
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Email</TableCell>
              <TableCell>Assigned</TableCell>
              <TableCell>Invitation emailed</TableCell>
              {canAssign && <TableCell align="right" />}
            </TableRow>
          </TableHead>
          <TableBody>
            {assignments.map((a) => (
              <TableRow key={a.id} hover>
                <TableCell>{fullName(a)}</TableCell>
                <TableCell>
                  <Typography variant="body2">{a.email}</Typography>
                </TableCell>
                <TableCell>
                  <Typography variant="body2">{formatDateTime(a.created_at)}</Typography>
                </TableCell>
                <TableCell>
                  <Typography variant="body2">{a.invite_email_sent_at ? formatDateTime(a.invite_email_sent_at) : "Not sent"}</Typography>
                </TableCell>
                {canAssign && (
                  <TableCell align="right">
                    <Button size="small" color="secondary" onClick={() => setRemoving(a)}>
                      Unassign
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {adding && <AssignDialog orgId={orgId} exam={exam} assignedIds={new Set(assignments.map((a) => a.user_id))} onClose={() => setAdding(false)} />}

      <ConfirmDialog
        open={Boolean(removing)}
        title="Unassign candidate?"
        confirmLabel="Unassign"
        busy={unassign.isPending}
        onClose={() => setRemoving(null)}
        onConfirm={async () => {
          try {
            await unassign.mutateAsync(removing.id);
            showSuccess("Candidate unassigned.");
          } catch (err) {
            showError(apiError(err).message);
          }
          setRemoving(null);
        }}
      >
        {removing && `${fullName(removing)} will no longer be able to take this exam.`}
      </ConfirmDialog>
    </Panel>
  );
}

function AssignDialog({ orgId, exam, assignedIds, onClose }) {
  const { showSuccess } = useToast();
  const { data, isLoading } = useOrgMembersForAssign(orgId, true);
  const [selected, setSelected] = useState(new Set());
  const [search, setSearch] = useState("");
  const [allRoles, setAllRoles] = useState(false);
  const [sendEmail, setSendEmail] = useState(exam.status === "published");
  const [error, setError] = useState(null);
  const assign = useExamMutation(orgId, exam.id, (body) => examsApi.assign(orgId, exam.id, body));

  const members = useMemo(() => {
    const q = search.trim().toLowerCase();
    return (data?.items ?? []).filter(
      (m) => (allRoles || m.role === "candidate") && (!q || fullName(m).toLowerCase().includes(q) || m.email.toLowerCase().includes(q)),
    );
  }, [data, search, allRoles]);

  const selectable = members.filter((m) => !assignedIds.has(m.user_id));
  const allChecked = selectable.length > 0 && selectable.every((m) => selected.has(m.user_id));

  const toggle = (id) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const submit = async () => {
    setError(null);
    try {
      const res = await assign.mutateAsync({ user_ids: [...selected], send_email: sendEmail });
      showSuccess(`${res.created} candidate(s) assigned${res.emailed ? `, ${res.emailed} emailed` : ""}.`);
      onClose();
    } catch (err) {
      setError(apiError(err, "Could not assign candidates."));
    }
  };

  return (
    <Dialog open onClose={() => !assign.isPending && onClose()} maxWidth="sm" fullWidth slotProps={dialogPaper}>
      <DialogTitle sx={{ fontSize: "1.125rem", fontWeight: 600 }}>Assign candidates</DialogTitle>
      <DialogContent>
        <FormErrors error={error} />
        <Box sx={{ display: "flex", gap: 2, alignItems: "center", mb: 1 }}>
          <TextField size="small" placeholder="Search members" value={search} onChange={(e) => setSearch(e.target.value)} fullWidth />
          <FormControlLabel
            control={<Checkbox size="small" checked={allRoles} onChange={(e) => setAllRoles(e.target.checked)} />}
            label="All roles"
            sx={{ whiteSpace: "nowrap", "& .MuiFormControlLabel-label": { fontSize: "0.8125rem" } }}
          />
        </Box>

        {isLoading ? (
          <Loading />
        ) : members.length === 0 ? (
          <EmptyState title="No members found">
            {allRoles ? "Nobody matches your search." : "There are no members with the candidate role. Invite candidates from Users & Team."}
          </EmptyState>
        ) : (
          <Box sx={{ maxHeight: 360, overflowY: "auto", border: "1px solid #e4e4e7", borderRadius: 1.5 }}>
            <Table size="small" stickyHeader>
              <TableHead>
                <TableRow>
                  <TableCell padding="checkbox">
                    <Checkbox
                      size="small"
                      checked={allChecked}
                      disabled={selectable.length === 0}
                      onChange={() => setSelected(allChecked ? new Set() : new Set(selectable.map((m) => m.user_id)))}
                    />
                  </TableCell>
                  <TableCell>Name</TableCell>
                  <TableCell>Role</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {members.map((m) => {
                  const assigned = assignedIds.has(m.user_id);
                  return (
                    <TableRow key={m.user_id} hover={!assigned} onClick={() => !assigned && toggle(m.user_id)} sx={{ cursor: assigned ? "default" : "pointer" }}>
                      <TableCell padding="checkbox">
                        <Checkbox size="small" checked={assigned || selected.has(m.user_id)} disabled={assigned} />
                      </TableCell>
                      <TableCell>
                        <Typography sx={{ fontSize: "0.875rem" }}>{fullName(m)}</Typography>
                        <Typography variant="body2">{m.email}</Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">{assigned ? "Already assigned" : ROLE_LABELS[m.role]}</Typography>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </Box>
        )}

        <FormControlLabel
          sx={{ mt: 2 }}
          control={<Checkbox size="small" checked={sendEmail} disabled={exam.status !== "published"} onChange={(e) => setSendEmail(e.target.checked)} />}
          label={exam.status === "published" ? "Email them an invitation" : "Invitations can be emailed once the exam is published"}
        />
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={onClose} disabled={assign.isPending}>
          Cancel
        </Button>
        <Button variant="contained" onClick={submit} disabled={assign.isPending || selected.size === 0}>
          {assign.isPending ? <CircularProgress size={18} color="inherit" /> : `Assign ${selected.size || ""}`.trim()}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
