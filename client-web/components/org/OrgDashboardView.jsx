"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import Divider from "@mui/material/Divider";
import CircularProgress from "@mui/material/CircularProgress";
import ButtonBase from "@mui/material/ButtonBase";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import InviteMemberDialog from "./InviteMemberDialog";
import {
  useOrganizationMembers,
  useOrganizationInvitations,
  useResendInvitationMutation,
  useRevokeInvitationMutation,
} from "@/hooks/api/useOrgs";
import { getApiErrorMessage } from "@/hooks/api/useAuth";

export default function OrgDashboardView({ org }) {
  const [activeTab, setActiveTab] = useState(0);
  const [inviteDialogOpen, setInviteDialogOpen] = useState(false);
  const [actionFeedback, setActionFeedback] = useState(null);

  const { data: members = [], isLoading: isLoadingMembers } = useOrganizationMembers(org?.id);
  const { data: invitations = [], isLoading: isLoadingInvites } = useOrganizationInvitations(org?.id);

  const resendMutation = useResendInvitationMutation(org?.id, {
    onSuccess: () => {
      setActionFeedback({
        type: "success",
        message: "Invitation resent successfully.",
      });
    },
    onError: (err) => {
      setActionFeedback({
        type: "error",
        message: getApiErrorMessage(err, "Failed to resend invitation."),
      });
    },
  });

  const revokeMutation = useRevokeInvitationMutation(org?.id, {
    onSuccess: () => {
      setActionFeedback({
        type: "success",
        message: "Invitation revoked.",
      });
    },
    onError: (err) => {
      setActionFeedback({
        type: "error",
        message: getApiErrorMessage(err, "Failed to revoke invitation."),
      });
    },
  });

  const canManage = org?.my_role === "owner" || org?.my_role === "admin";

  return (
    <Box sx={{ width: "100%" }}>
      <AuthFeedback
        type={actionFeedback?.type}
        message={actionFeedback?.message}
        onClose={() => setActionFeedback(null)}
      />

      {/* Organization Header */}
      <Paper
        elevation={0}
        sx={{
          p: { xs: 3, md: 4 },
          mb: 4,
          backgroundColor: "#ffffff",
          border: "1px solid #e4e4e7",
          borderRadius: 2,
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            justifyContent: "space-between",
            alignItems: { xs: "flex-start", sm: "center" },
            gap: 2,
          }}
        >
          <Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
              <Typography
                component="h1"
                sx={{
                  fontSize: "1.375rem",
                  fontWeight: 600,
                  letterSpacing: "-0.015em",
                  color: "#09090b",
                }}
              >
                {org.name}
              </Typography>
              <Box
                sx={{
                  fontSize: "0.6875rem",
                  fontWeight: 700,
                  textTransform: "uppercase",
                  letterSpacing: "0.06em",
                  px: 1,
                  py: 0.25,
                  borderRadius: "4px",
                  backgroundColor: "#09090b",
                  color: "#ffffff",
                }}
              >
                {org.my_role}
              </Box>
            </Box>
            <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.5 }}>
              Workspace slug: <code style={{ fontFamily: "monospace" }}>{org.slug}</code>
              {org.website && (
                <> • <a href={org.website} target="_blank" rel="noreferrer" style={{ textDecoration: "underline" }}>{org.website}</a></>
              )}
            </Typography>
          </Box>

          {canManage && (
            <AuthButton
              variant="contained"
              fullWidth={false}
              onClick={() => setInviteDialogOpen(true)}
              sx={{ px: 2, py: 0.85, fontSize: "0.8125rem" }}
            >
              + Invite Member
            </AuthButton>
          )}
        </Box>

        <Divider sx={{ my: 3, borderColor: "#f4f4f5" }} />

        {/* Minimal Tabs */}
        <Tabs
          value={activeTab}
          onChange={(_, val) => setActiveTab(val)}
          sx={{
            minHeight: 36,
            "& .MuiTabs-indicator": {
              backgroundColor: "#09090b",
              height: 2,
            },
          }}
        >
          <Tab
            label="Overview"
            sx={{
              textTransform: "none",
              fontSize: "0.875rem",
              fontWeight: 500,
              minHeight: 36,
              px: 2,
              color: "#71717a",
              "&.Mui-selected": { color: "#09090b", fontWeight: 600 },
            }}
          />
          <Tab
            label={`Team Members (${members.length})`}
            sx={{
              textTransform: "none",
              fontSize: "0.875rem",
              fontWeight: 500,
              minHeight: 36,
              px: 2,
              color: "#71717a",
              "&.Mui-selected": { color: "#09090b", fontWeight: 600 },
            }}
          />
          <Tab
            label={`Invitations (${invitations.length})`}
            sx={{
              textTransform: "none",
              fontSize: "0.875rem",
              fontWeight: 500,
              minHeight: 36,
              px: 2,
              color: "#71717a",
              "&.Mui-selected": { color: "#09090b", fontWeight: 600 },
            }}
          />
        </Tabs>
      </Paper>

      {/* Tab 0: Overview */}
      {activeTab === 0 && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          {/* Metric Cards */}
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr 1fr" },
              gap: 2.5,
            }}
          >
            <Paper
              elevation={0}
              sx={{
                p: 3,
                backgroundColor: "#ffffff",
                border: "1px solid #e4e4e7",
                borderRadius: 2,
              }}
            >
              <Typography sx={{ fontSize: "0.75rem", color: "#71717a", textTransform: "uppercase", letterSpacing: "0.06em", mb: 0.5 }}>
                Active Members
              </Typography>
              <Typography sx={{ fontSize: "1.75rem", fontWeight: 700, color: "#09090b" }}>
                {isLoadingMembers ? "..." : members.length}
              </Typography>
              <Typography sx={{ fontSize: "0.75rem", color: "#71717a", mt: 0.5 }}>
                Owners, instructors, & proctors
              </Typography>
            </Paper>

            <Paper
              elevation={0}
              sx={{
                p: 3,
                backgroundColor: "#ffffff",
                border: "1px solid #e4e4e7",
                borderRadius: 2,
              }}
            >
              <Typography sx={{ fontSize: "0.75rem", color: "#71717a", textTransform: "uppercase", letterSpacing: "0.06em", mb: 0.5 }}>
                Pending Invitations
              </Typography>
              <Typography sx={{ fontSize: "1.75rem", fontWeight: 700, color: "#09090b" }}>
                {isLoadingInvites ? "..." : invitations.length}
              </Typography>
              <Typography sx={{ fontSize: "0.75rem", color: "#71717a", mt: 0.5 }}>
                Awaiting registration or acceptance
              </Typography>
            </Paper>

            <Paper
              elevation={0}
              sx={{
                p: 3,
                backgroundColor: "#ffffff",
                border: "1px solid #e4e4e7",
                borderRadius: 2,
              }}
            >
              <Typography sx={{ fontSize: "0.75rem", color: "#71717a", textTransform: "uppercase", letterSpacing: "0.06em", mb: 0.5 }}>
                Workspace Status
              </Typography>
              <Typography sx={{ fontSize: "1.75rem", fontWeight: 700, color: "#09090b" }}>
                Active
              </Typography>
              <Typography sx={{ fontSize: "0.75rem", color: "#71717a", mt: 0.5 }}>
                Exam authoring enabled
              </Typography>
            </Paper>
          </Box>

          {/* Quick Action Modules */}
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
              gap: 2.5,
            }}
          >
            <Paper
              elevation={0}
              sx={{
                p: 3.5,
                backgroundColor: "#ffffff",
                border: "1px solid #e4e4e7",
                borderRadius: 2,
              }}
            >
              <Typography sx={{ fontSize: "1rem", fontWeight: 600, color: "#09090b", mb: 1 }}>
                Author Assessments
              </Typography>
              <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mb: 2.5, lineHeight: 1.5 }}>
                Create structured multi-section exams, multiple choice question banks, and custom grading standards for {org.name}.
              </Typography>
              <AuthButton
                variant="outlined"
                fullWidth={false}
                sx={{ px: 2, py: 0.75, fontSize: "0.8125rem" }}
              >
                Launch Exam Authoring
              </AuthButton>
            </Paper>

            <Paper
              elevation={0}
              sx={{
                p: 3.5,
                backgroundColor: "#ffffff",
                border: "1px solid #e4e4e7",
                borderRadius: 2,
              }}
            >
              <Typography sx={{ fontSize: "1rem", fontWeight: 600, color: "#09090b", mb: 1 }}>
                Team Collaboration
              </Typography>
              <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mb: 2.5, lineHeight: 1.5 }}>
                Invite co-instructors to write questions or proctors to oversee live candidate sessions.
              </Typography>
              <AuthButton
                variant="outlined"
                fullWidth={false}
                onClick={() => setInviteDialogOpen(true)}
                disabled={!canManage}
                sx={{ px: 2, py: 0.75, fontSize: "0.8125rem" }}
              >
                + Invite Collaborator
              </AuthButton>
            </Paper>
          </Box>
        </Box>
      )}

      {/* Tab 1: Team Members */}
      {activeTab === 1 && (
        <Paper
          elevation={0}
          sx={{
            backgroundColor: "#ffffff",
            border: "1px solid #e4e4e7",
            borderRadius: 2,
            overflow: "hidden",
          }}
        >
          <Box sx={{ p: 3, display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <Box>
              <Typography sx={{ fontSize: "1rem", fontWeight: 600, color: "#09090b" }}>
                Active Members
              </Typography>
              <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
                Personnel with assigned roles in {org.name}.
              </Typography>
            </Box>
            {canManage && (
              <AuthButton
                variant="outlined"
                fullWidth={false}
                onClick={() => setInviteDialogOpen(true)}
                sx={{ px: 2, py: 0.65, fontSize: "0.8125rem" }}
              >
                + Invite
              </AuthButton>
            )}
          </Box>

          <Divider sx={{ borderColor: "#f4f4f5" }} />

          {isLoadingMembers ? (
            <Box sx={{ p: 4, textAlign: "center" }}>
              <CircularProgress size={24} sx={{ color: "#09090b" }} />
            </Box>
          ) : members.length === 0 ? (
            <Box sx={{ p: 4, textAlign: "center" }}>
              <Typography sx={{ fontSize: "0.875rem", color: "#71717a" }}>
                No members found.
              </Typography>
            </Box>
          ) : (
            <Box component="div" sx={{ width: "100%", overflowX: "auto" }}>
              <Box
                component="table"
                sx={{
                  width: "100%",
                  borderCollapse: "collapse",
                  fontSize: "0.8125rem",
                  textAlign: "left",
                  "& th": {
                    px: 3,
                    py: 1.5,
                    borderBottom: "1px solid #e4e4e7",
                    fontSize: "0.6875rem",
                    fontWeight: 700,
                    textTransform: "uppercase",
                    letterSpacing: "0.06em",
                    color: "#71717a",
                    backgroundColor: "#fafafa",
                  },
                  "& td": {
                    px: 3,
                    py: 2,
                    borderBottom: "1px solid #f4f4f5",
                    color: "#09090b",
                  },
                }}
              >
                <thead>
                  <tr>
                    <th>Member</th>
                    <th>Email</th>
                    <th>Role</th>
                    <th>Joined</th>
                  </tr>
                </thead>
                <tbody>
                  {members.map((m) => (
                    <tr key={m.user_id}>
                      <td>
                        <Typography sx={{ fontSize: "0.875rem", fontWeight: 500, color: "#09090b" }}>
                          {m.first_name} {m.last_name || ""}
                        </Typography>
                      </td>
                      <td>
                        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
                          {m.email}
                        </Typography>
                      </td>
                      <td>
                        <Box
                          sx={{
                            display: "inline-block",
                            fontSize: "0.6875rem",
                            fontWeight: 700,
                            textTransform: "uppercase",
                            px: 1,
                            py: 0.25,
                            borderRadius: "4px",
                            backgroundColor: "#f4f4f5",
                            color: "#09090b",
                            border: "1px solid #e4e4e7",
                          }}
                        >
                          {m.role}
                        </Box>
                      </td>
                      <td>
                        <Typography sx={{ fontSize: "0.75rem", color: "#71717a" }}>
                          {m.joined_at ? new Date(m.joined_at).toLocaleDateString() : "—"}
                        </Typography>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </Box>
            </Box>
          )}
        </Paper>
      )}

      {/* Tab 2: Pending Invitations */}
      {activeTab === 2 && (
        <Paper
          elevation={0}
          sx={{
            backgroundColor: "#ffffff",
            border: "1px solid #e4e4e7",
            borderRadius: 2,
            overflow: "hidden",
          }}
        >
          <Box sx={{ p: 3, display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <Box>
              <Typography sx={{ fontSize: "1rem", fontWeight: 600, color: "#09090b" }}>
                Invitations
              </Typography>
              <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
                Pending and active invitations sent to external team members.
              </Typography>
            </Box>
            {canManage && (
              <AuthButton
                variant="contained"
                fullWidth={false}
                onClick={() => setInviteDialogOpen(true)}
                sx={{ px: 2, py: 0.65, fontSize: "0.8125rem" }}
              >
                + Send Invitation
              </AuthButton>
            )}
          </Box>

          <Divider sx={{ borderColor: "#f4f4f5" }} />

          {isLoadingInvites ? (
            <Box sx={{ p: 4, textAlign: "center" }}>
              <CircularProgress size={24} sx={{ color: "#09090b" }} />
            </Box>
          ) : invitations.length === 0 ? (
            <Box sx={{ p: 4, textAlign: "center" }}>
              <Typography sx={{ fontSize: "0.875rem", color: "#71717a", mb: 1 }}>
                No invitations currently pending.
              </Typography>
              {canManage && (
                <AuthButton
                  variant="outlined"
                  fullWidth={false}
                  onClick={() => setInviteDialogOpen(true)}
                  sx={{ px: 2, py: 0.65, fontSize: "0.8125rem" }}
                >
                  Invite First Collaborator
                </AuthButton>
              )}
            </Box>
          ) : (
            <Box component="div" sx={{ width: "100%", overflowX: "auto" }}>
              <Box
                component="table"
                sx={{
                  width: "100%",
                  borderCollapse: "collapse",
                  fontSize: "0.8125rem",
                  textAlign: "left",
                  "& th": {
                    px: 3,
                    py: 1.5,
                    borderBottom: "1px solid #e4e4e7",
                    fontSize: "0.6875rem",
                    fontWeight: 700,
                    textTransform: "uppercase",
                    letterSpacing: "0.06em",
                    color: "#71717a",
                    backgroundColor: "#fafafa",
                  },
                  "& td": {
                    px: 3,
                    py: 2,
                    borderBottom: "1px solid #f4f4f5",
                    color: "#09090b",
                  },
                }}
              >
                <thead>
                  <tr>
                    <th>Invitee Email</th>
                    <th>Role</th>
                    <th>Status</th>
                    <th>Sent At</th>
                    {canManage && <th style={{ textAlign: "right" }}>Actions</th>}
                  </tr>
                </thead>
                <tbody>
                  {invitations.map((inv) => (
                    <tr key={inv.id}>
                      <td>
                        <Typography sx={{ fontSize: "0.875rem", fontWeight: 500, color: "#09090b" }}>
                          {inv.email}
                        </Typography>
                      </td>
                      <td>
                        <Box
                          sx={{
                            display: "inline-block",
                            fontSize: "0.6875rem",
                            fontWeight: 700,
                            textTransform: "uppercase",
                            px: 1,
                            py: 0.25,
                            borderRadius: "4px",
                            backgroundColor: "#f4f4f5",
                            color: "#09090b",
                            border: "1px solid #e4e4e7",
                          }}
                        >
                          {inv.role}
                        </Box>
                      </td>
                      <td>
                        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", textTransform: "capitalize" }}>
                          {inv.status || "pending"}
                        </Typography>
                      </td>
                      <td>
                        <Typography sx={{ fontSize: "0.75rem", color: "#71717a" }}>
                          {inv.created_at ? new Date(inv.created_at).toLocaleDateString() : "—"}
                        </Typography>
                      </td>
                      {canManage && (
                        <td style={{ textAlign: "right" }}>
                          <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1 }}>
                            <ButtonBase
                              disabled={resendMutation.isPending}
                              onClick={() => resendMutation.mutate(inv.id)}
                              sx={{
                                fontSize: "0.75rem",
                                fontWeight: 600,
                                color: "#09090b",
                                px: 1,
                                py: 0.5,
                                borderRadius: "4px",
                                border: "1px solid #e4e4e7",
                                "&:hover": { backgroundColor: "#f4f4f5" },
                              }}
                            >
                              Resend
                            </ButtonBase>
                            <ButtonBase
                              disabled={revokeMutation.isPending}
                              onClick={() => revokeMutation.mutate(inv.id)}
                              sx={{
                                fontSize: "0.75rem",
                                fontWeight: 500,
                                color: "#71717a",
                                px: 1,
                                py: 0.5,
                                borderRadius: "4px",
                                border: "1px solid #e4e4e7",
                                "&:hover": { backgroundColor: "#f4f4f5", color: "#09090b" },
                              }}
                            >
                              Revoke
                            </ButtonBase>
                          </Box>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </Box>
            </Box>
          )}
        </Paper>
      )}

      {/* Invite Member Dialog */}
      <InviteMemberDialog
        open={inviteDialogOpen}
        onClose={() => setInviteDialogOpen(false)}
        org={org}
      />
    </Box>
  );
}
