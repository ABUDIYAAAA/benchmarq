"use client";

import { useState, useMemo } from "react";
import Box from "@mui/material/Box";
import Container from "@mui/material/Container";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import OutlinedInput from "@mui/material/OutlinedInput";
import InputAdornment from "@mui/material/InputAdornment";
import Select from "@mui/material/Select";
import MenuItem from "@mui/material/MenuItem";
import Menu from "@mui/material/Menu";
import IconButton from "@mui/material/IconButton";
import CircularProgress from "@mui/material/CircularProgress";
import ButtonBase from "@mui/material/ButtonBase";
import AuthButton from "@/components/auth/AuthButton";
import InviteMemberDialog from "@/components/org/InviteMemberDialog";
import ChangeRoleDialog from "@/components/org/ChangeRoleDialog";
import RemoveMemberDialog from "@/components/org/RemoveMemberDialog";
import { useAuth } from "@/context/AuthContext";
import { useOrg } from "@/context/OrgContext";
import { useToast } from "@/context/ToastContext";
import {
  useOrganizationMembers,
  useOrganizationInvitations,
  useResendInvitationMutation,
  useRevokeInvitationMutation,
} from "@/hooks/api/useOrgs";
import { getApiErrorMessage } from "@/hooks/api/useAuth";

export default function UsersManagementPage() {
  const { user: currentUser } = useAuth();
  const { currentOrg, hasOrgs } = useOrg();
  const { showSuccess, showError } = useToast();

  const [activeTab, setActiveTab] = useState("members"); // "members" | "invitations"
  const [searchQuery, setSearchQuery] = useState("");
  const [roleFilter, setRoleFilter] = useState("all");
  const [inviteDialogOpen, setInviteDialogOpen] = useState(false);

  // Action Menu state for Members
  const [menuAnchorEl, setMenuAnchorEl] = useState(null);
  const [selectedMember, setSelectedMember] = useState(null);
  const [changeRoleOpen, setChangeRoleOpen] = useState(false);
  const [removeOpen, setRemoveOpen] = useState(false);

  // In-flight invitation actions
  const [resendingId, setResendingId] = useState(null);
  const [revokingId, setRevokingId] = useState(null);

  const { data: members = [], isLoading: isLoadingMembers } = useOrganizationMembers(currentOrg?.id);
  const { data: invitations = [], isLoading: isLoadingInvitations } = useOrganizationInvitations(currentOrg?.id);

  const resendMutation = useResendInvitationMutation(currentOrg?.id);
  const revokeMutation = useRevokeInvitationMutation(currentOrg?.id);

  // Active / pending invitation count
  const pendingInvitations = useMemo(() => {
    return invitations.filter((inv) => inv.status === "pending" || !inv.status);
  }, [invitations]);

  // Filtered Members
  const filteredMembers = useMemo(() => {
    return members.filter((m) => {
      const matchesSearch =
        searchQuery.trim() === "" ||
        `${m.first_name || ""} ${m.last_name || ""}`
          .toLowerCase()
          .includes(searchQuery.toLowerCase()) ||
        (m.email || "").toLowerCase().includes(searchQuery.toLowerCase());

      const matchesRole = roleFilter === "all" || m.role === roleFilter;

      return matchesSearch && matchesRole;
    });
  }, [members, searchQuery, roleFilter]);

  // Filtered Invitations
  const filteredInvitations = useMemo(() => {
    return invitations.filter((inv) => {
      const matchesSearch =
        searchQuery.trim() === "" ||
        (inv.email || "").toLowerCase().includes(searchQuery.toLowerCase());

      const matchesRole = roleFilter === "all" || inv.role === roleFilter;

      return matchesSearch && matchesRole;
    });
  }, [invitations, searchQuery, roleFilter]);

  const handleOpenMenu = (event, member) => {
    setMenuAnchorEl(event.currentTarget);
    setSelectedMember(member);
  };

  const handleCloseMenu = () => {
    setMenuAnchorEl(null);
  };

  const handleOpenChangeRole = () => {
    handleCloseMenu();
    setChangeRoleOpen(true);
  };

  const handleOpenRemove = () => {
    handleCloseMenu();
    setRemoveOpen(true);
  };

  const handleResendInvitation = async (inv) => {
    setResendingId(inv.id);
    try {
      await resendMutation.mutateAsync(inv.id);
      showSuccess(`Invitation resent to ${inv.email}.`);
    } catch (err) {
      showError(getApiErrorMessage(err, "Failed to resend invitation."));
    } finally {
      setResendingId(null);
    }
  };

  const handleRevokeInvitation = async (inv) => {
    setRevokingId(inv.id);
    try {
      await revokeMutation.mutateAsync(inv.id);
      showSuccess(`Invitation for ${inv.email} revoked.`);
    } catch (err) {
      showError(getApiErrorMessage(err, "Failed to revoke invitation."));
    } finally {
      setRevokingId(null);
    }
  };

  const canManage = currentOrg?.my_role === "owner" || currentOrg?.my_role === "admin";

  if (!hasOrgs || !currentOrg) {
    return (
      <Container maxWidth="lg" sx={{ py: 6 }}>
        <Typography sx={{ color: "#71717a", textAlign: "center" }}>
          Please select or create an organization to view team members.
        </Typography>
      </Container>
    );
  }

  return (
    <Container maxWidth="lg" sx={{ py: 4, px: { xs: 2.5, sm: 4 }, flex: 1 }}>
      {/* Header */}
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          justifyContent: "space-between",
          alignItems: { xs: "flex-start", sm: "center" },
          gap: 2,
          mb: 3,
        }}
      >
        <Box>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}>
            <Typography sx={{ fontSize: "0.75rem", color: "#71717a", textTransform: "uppercase", letterSpacing: "0.06em" }}>
              {currentOrg.name}
            </Typography>
            <Typography sx={{ fontSize: "0.75rem", color: "#a1a1aa" }}>/</Typography>
            <Typography sx={{ fontSize: "0.75rem", fontWeight: 600, color: "#09090b", textTransform: "uppercase", letterSpacing: "0.06em" }}>
              Users & Team
            </Typography>
          </Box>
          <Typography
            component="h1"
            sx={{
              fontSize: "1.5rem",
              fontWeight: 600,
              letterSpacing: "-0.015em",
              color: "#09090b",
            }}
          >
            Team Members
          </Typography>
          <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.25 }}>
            Manage staff members, assigned roles, and test authoring privileges.
          </Typography>
        </Box>

        {canManage && (
          <AuthButton
            variant="contained"
            fullWidth={false}
            onClick={() => setInviteDialogOpen(true)}
            sx={{ px: 2.5, py: 0.9, fontSize: "0.8125rem" }}
          >
            + Invite Member
          </AuthButton>
        )}
      </Box>

      {/* Tabs */}
      <Box
        sx={{
          display: "flex",
          gap: 1.5,
          borderBottom: "1px solid #e4e4e7",
          mb: 3,
        }}
      >
        <ButtonBase
          onClick={() => setActiveTab("members")}
          sx={{
            pb: 1.5,
            pt: 0.5,
            px: 1.5,
            fontSize: "0.875rem",
            fontWeight: activeTab === "members" ? 600 : 500,
            color: activeTab === "members" ? "#09090b" : "#71717a",
            borderBottom: "2px solid",
            borderColor: activeTab === "members" ? "#09090b" : "transparent",
            display: "flex",
            alignItems: "center",
            gap: 1,
            transition: "color 120ms ease-out, border-color 120ms ease-out",
            "&:hover": { color: "#09090b" },
          }}
        >
          <span>Active Members</span>
          <Box
            sx={{
              px: 1,
              py: 0.1,
              fontSize: "0.6875rem",
              fontWeight: 600,
              borderRadius: "10px",
              backgroundColor: activeTab === "members" ? "#09090b" : "#f4f4f5",
              color: activeTab === "members" ? "#ffffff" : "#71717a",
            }}
          >
            {members.length}
          </Box>
        </ButtonBase>

        <ButtonBase
          onClick={() => setActiveTab("invitations")}
          sx={{
            pb: 1.5,
            pt: 0.5,
            px: 1.5,
            fontSize: "0.875rem",
            fontWeight: activeTab === "invitations" ? 600 : 500,
            color: activeTab === "invitations" ? "#09090b" : "#71717a",
            borderBottom: "2px solid",
            borderColor: activeTab === "invitations" ? "#09090b" : "transparent",
            display: "flex",
            alignItems: "center",
            gap: 1,
            transition: "color 120ms ease-out, border-color 120ms ease-out",
            "&:hover": { color: "#09090b" },
          }}
        >
          <span>Pending Invitations</span>
          <Box
            sx={{
              px: 1,
              py: 0.1,
              fontSize: "0.6875rem",
              fontWeight: 600,
              borderRadius: "10px",
              backgroundColor: activeTab === "invitations" ? "#09090b" : "#f4f4f5",
              color: activeTab === "invitations" ? "#ffffff" : pendingInvitations.length > 0 ? "#09090b" : "#71717a",
              border: "1px solid",
              borderColor: activeTab === "invitations" ? "#09090b" : "#e4e4e7",
            }}
          >
            {invitations.length}
          </Box>
        </ButtonBase>
      </Box>

      {/* Filter and Search Bar */}
      <Paper
        elevation={0}
        sx={{
          p: 2,
          mb: 3,
          backgroundColor: "#ffffff",
          border: "1px solid #e4e4e7",
          borderRadius: 2,
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          gap: 2,
          alignItems: { xs: "stretch", sm: "center" },
          justifyContent: "space-between",
        }}
      >
        <Box sx={{ display: "flex", flex: 1, gap: 1.5, flexWrap: "wrap" }}>
          {/* Search Input */}
          <OutlinedInput
            size="small"
            placeholder={
              activeTab === "members"
                ? "Search by name or email..."
                : "Search invitations by email..."
            }
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            startAdornment={
              <InputAdornment position="start">
                <Box component="span" sx={{ color: "#71717a", display: "flex" }}>
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <circle cx="11" cy="11" r="8" />
                    <line x1="21" y1="21" x2="16.65" y2="16.65" />
                  </svg>
                </Box>
              </InputAdornment>
            }
            sx={{
              maxWidth: { sm: 320 },
              flex: 1,
              borderRadius: "6px",
              fontSize: "0.8125rem",
            }}
          />

          {/* Role Filter */}
          <Select
            size="small"
            value={roleFilter}
            onChange={(e) => setRoleFilter(e.target.value)}
            sx={{
              minWidth: 140,
              borderRadius: "6px",
              fontSize: "0.8125rem",
            }}
          >
            <MenuItem value="all">All Roles</MenuItem>
            <MenuItem value="owner">Owner</MenuItem>
            <MenuItem value="admin">Admin</MenuItem>
            <MenuItem value="instructor">Instructor</MenuItem>
            <MenuItem value="proctor">Proctor</MenuItem>
            <MenuItem value="candidate">Candidate</MenuItem>
          </Select>
        </Box>

        <Typography sx={{ fontSize: "0.75rem", color: "#71717a", alignSelf: { sm: "center" } }}>
          {activeTab === "members"
            ? `Showing ${filteredMembers.length} of ${members.length} members`
            : `Showing ${filteredInvitations.length} of ${invitations.length} invitations`}
        </Typography>
      </Paper>

      {/* TAB 1: MEMBERS */}
      {activeTab === "members" && (
        <Paper
          elevation={0}
          sx={{
            backgroundColor: "#ffffff",
            border: "1px solid #e4e4e7",
            borderRadius: 2,
            overflow: "hidden",
          }}
        >
          {isLoadingMembers ? (
            <Box sx={{ py: 8, display: "flex", justifyContent: "center" }}>
              <CircularProgress size={24} sx={{ color: "#09090b" }} />
            </Box>
          ) : filteredMembers.length === 0 ? (
            <Box sx={{ py: 8, textAlign: "center", px: 3 }}>
              <Typography sx={{ fontSize: "0.875rem", fontWeight: 500, color: "#09090b" }}>
                No members match your criteria
              </Typography>
              <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.5 }}>
                Try adjusting your search terms or role filters.
              </Typography>
              {(searchQuery || roleFilter !== "all") && (
                <ButtonBase
                  onClick={() => {
                    setSearchQuery("");
                    setRoleFilter("all");
                  }}
                  sx={{
                    mt: 2,
                    px: 2,
                    py: 0.75,
                    fontSize: "0.75rem",
                    fontWeight: 500,
                    color: "#09090b",
                    border: "1px solid #e4e4e7",
                    borderRadius: "6px",
                    "&:hover": { backgroundColor: "#f4f4f5" },
                  }}
                >
                  Reset Filters
                </ButtonBase>
              )}
            </Box>
          ) : (
            <Box sx={{ width: "100%", overflowX: "auto" }}>
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
                    <th>User</th>
                    <th>Email</th>
                    <th>Role</th>
                    <th>Joined Date</th>
                    {canManage && <th style={{ textAlign: "right" }}>Actions</th>}
                  </tr>
                </thead>
                <tbody>
                  {filteredMembers.map((m) => {
                    const isSelf = m.user_id === currentUser?.id;
                    const initials = `${m.first_name?.[0] || ""}${m.last_name?.[0] || ""}`.toUpperCase() || "U";

                    return (
                      <tr key={m.user_id}>
                        {/* Name with initials glyph */}
                        <td>
                          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                            <Box
                              sx={{
                                width: 30,
                                height: 30,
                                borderRadius: "6px",
                                backgroundColor: "#f4f4f5",
                                border: "1px solid #e4e4e7",
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "center",
                                fontSize: "0.75rem",
                                fontWeight: 600,
                                color: "#09090b",
                                flexShrink: 0,
                              }}
                            >
                              {initials}
                            </Box>
                            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                              <Typography sx={{ fontSize: "0.875rem", fontWeight: 500, color: "#09090b" }}>
                                {m.first_name} {m.last_name || ""}
                              </Typography>
                              {isSelf && (
                                <Box
                                  component="span"
                                  sx={{
                                    fontSize: "0.625rem",
                                    fontWeight: 600,
                                    color: "#71717a",
                                    border: "1px solid #e4e4e7",
                                    borderRadius: "4px",
                                    px: 0.75,
                                    py: 0.1,
                                  }}
                                >
                                  You
                                </Box>
                              )}
                            </Box>
                          </Box>
                        </td>

                        {/* Email */}
                        <td>
                          <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
                            {m.email}
                          </Typography>
                        </td>

                        {/* Role Badge */}
                        <td>
                          <Box
                            sx={{
                              display: "inline-block",
                              fontSize: "0.6875rem",
                              fontWeight: 700,
                              textTransform: "uppercase",
                              letterSpacing: "0.04em",
                              px: 1,
                              py: 0.25,
                              borderRadius: "4px",
                              backgroundColor: m.role === "owner" ? "#09090b" : "#f4f4f5",
                              color: m.role === "owner" ? "#ffffff" : "#09090b",
                              border: "1px solid",
                              borderColor: m.role === "owner" ? "#09090b" : "#e4e4e7",
                            }}
                          >
                            {m.role}
                          </Box>
                        </td>

                        {/* Joined Date */}
                        <td>
                          <Typography sx={{ fontSize: "0.75rem", color: "#71717a" }}>
                            {m.joined_at ? new Date(m.joined_at).toLocaleDateString() : "—"}
                          </Typography>
                        </td>

                        {/* Actions */}
                        {canManage && (
                          <td style={{ textAlign: "right" }}>
                            <IconButton
                              size="small"
                              onClick={(e) => handleOpenMenu(e, m)}
                              sx={{
                                color: "#71717a",
                                "&:hover": { color: "#09090b", backgroundColor: "#f4f4f5" },
                              }}
                            >
                              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                                <circle cx="12" cy="12" r="1" />
                                <circle cx="19" cy="12" r="1" />
                                <circle cx="5" cy="12" r="1" />
                              </svg>
                            </IconButton>
                          </td>
                        )}
                      </tr>
                    );
                  })}
                </tbody>
              </Box>
            </Box>
          )}
        </Paper>
      )}

      {/* TAB 2: INVITATIONS */}
      {activeTab === "invitations" && (
        <Paper
          elevation={0}
          sx={{
            backgroundColor: "#ffffff",
            border: "1px solid #e4e4e7",
            borderRadius: 2,
            overflow: "hidden",
          }}
        >
          {isLoadingInvitations ? (
            <Box sx={{ py: 8, display: "flex", justifyContent: "center" }}>
              <CircularProgress size={24} sx={{ color: "#09090b" }} />
            </Box>
          ) : filteredInvitations.length === 0 ? (
            <Box sx={{ py: 8, textAlign: "center", px: 3 }}>
              <Box
                sx={{
                  width: 44,
                  height: 44,
                  borderRadius: "50%",
                  backgroundColor: "#f4f4f5",
                  border: "1px solid #e4e4e7",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  margin: "0 auto 16px",
                  color: "#71717a",
                }}
              >
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
                  <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
                  <polyline points="22,6 12,13 2,6" />
                </svg>
              </Box>
              <Typography sx={{ fontSize: "0.875rem", fontWeight: 500, color: "#09090b" }}>
                No pending invitations
              </Typography>
              <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.5, maxWidth: 360, mx: "auto" }}>
                When you invite team members, their invitations will appear here until they accept.
              </Typography>
              {canManage && (
                <ButtonBase
                  onClick={() => setInviteDialogOpen(true)}
                  sx={{
                    mt: 2.5,
                    px: 2,
                    py: 0.85,
                    fontSize: "0.8125rem",
                    fontWeight: 500,
                    color: "#ffffff",
                    backgroundColor: "#09090b",
                    borderRadius: "6px",
                    transition: "transform 120ms ease-out, background-color 120ms ease-out",
                    "&:hover": { backgroundColor: "#27272a" },
                    "&:active": { transform: "scale(0.985)" },
                  }}
                >
                  + Invite Member
                </ButtonBase>
              )}
            </Box>
          ) : (
            <Box sx={{ width: "100%", overflowX: "auto" }}>
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
                    <th>Invitee</th>
                    <th>Role</th>
                    <th>Status</th>
                    <th>Invited Date</th>
                    <th>Expires</th>
                    {canManage && <th style={{ textAlign: "right" }}>Actions</th>}
                  </tr>
                </thead>
                <tbody>
                  {filteredInvitations.map((inv) => {
                    const isPending = inv.status === "pending" || !inv.status;
                    const isExpired = inv.status === "expired";
                    const isRevoked = inv.status === "revoked";

                    return (
                      <tr key={inv.id}>
                        {/* Invitee Email */}
                        <td>
                          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                            <Box
                              sx={{
                                width: 28,
                                height: 28,
                                borderRadius: "6px",
                                backgroundColor: "#f4f4f5",
                                border: "1px solid #e4e4e7",
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "center",
                                color: "#71717a",
                                flexShrink: 0,
                              }}
                            >
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                                <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
                                <polyline points="22,6 12,13 2,6" />
                              </svg>
                            </Box>
                            <Typography sx={{ fontSize: "0.875rem", fontWeight: 500, color: "#09090b" }}>
                              {inv.email}
                            </Typography>
                          </Box>
                        </td>

                        {/* Role Badge */}
                        <td>
                          <Box
                            sx={{
                              display: "inline-block",
                              fontSize: "0.6875rem",
                              fontWeight: 700,
                              textTransform: "uppercase",
                              letterSpacing: "0.04em",
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

                        {/* Status Badge */}
                        <td>
                          <Box
                            sx={{
                              display: "inline-block",
                              fontSize: "0.6875rem",
                              fontWeight: 600,
                              textTransform: "uppercase",
                              letterSpacing: "0.04em",
                              px: 1,
                              py: 0.25,
                              borderRadius: "4px",
                              backgroundColor: isPending ? "#fafafa" : "#f4f4f5",
                              color: isPending ? "#09090b" : "#71717a",
                              border: "1px solid",
                              borderColor: isPending ? "#d4d4d8" : "#e4e4e7",
                            }}
                          >
                            {inv.status || "pending"}
                          </Box>
                        </td>

                        {/* Sent Date */}
                        <td>
                          <Typography sx={{ fontSize: "0.75rem", color: "#71717a" }}>
                            {inv.created_at ? new Date(inv.created_at).toLocaleDateString() : "—"}
                          </Typography>
                        </td>

                        {/* Expires */}
                        <td>
                          <Typography sx={{ fontSize: "0.75rem", color: "#71717a" }}>
                            {inv.expires_at ? new Date(inv.expires_at).toLocaleDateString() : "—"}
                          </Typography>
                        </td>

                        {/* Actions */}
                        {canManage && (
                          <td style={{ textAlign: "right" }}>
                            <Box sx={{ display: "inline-flex", alignItems: "center", gap: 1 }}>
                              {/* Resend button */}
                              <ButtonBase
                                onClick={() => handleResendInvitation(inv)}
                                disabled={resendingId === inv.id || isRevoked}
                                sx={{
                                  px: 1.5,
                                  py: 0.5,
                                  fontSize: "0.75rem",
                                  fontWeight: 500,
                                  color: "#09090b",
                                  border: "1px solid #e4e4e7",
                                  borderRadius: "4px",
                                  backgroundColor: "#ffffff",
                                  transition: "background-color 120ms ease-out, border-color 120ms ease-out",
                                  "&:hover": { backgroundColor: "#f4f4f5", borderColor: "#a1a1aa" },
                                  "&:disabled": { opacity: 0.5, cursor: "not-allowed" },
                                }}
                              >
                                {resendingId === inv.id ? (
                                  <CircularProgress size={12} sx={{ color: "#09090b", mr: 0.75 }} />
                                ) : null}
                                Resend
                              </ButtonBase>

                              {/* Revoke button */}
                              {isPending && (
                                <ButtonBase
                                  onClick={() => handleRevokeInvitation(inv)}
                                  disabled={revokingId === inv.id}
                                  sx={{
                                    px: 1.5,
                                    py: 0.5,
                                    fontSize: "0.75rem",
                                    fontWeight: 500,
                                    color: "#71717a",
                                    border: "1px solid #e4e4e7",
                                    borderRadius: "4px",
                                    backgroundColor: "#ffffff",
                                    transition: "color 120ms ease-out, background-color 120ms ease-out, border-color 120ms ease-out",
                                    "&:hover": { color: "#09090b", backgroundColor: "#f4f4f5", borderColor: "#a1a1aa" },
                                    "&:disabled": { opacity: 0.5, cursor: "not-allowed" },
                                  }}
                                >
                                  {revokingId === inv.id ? (
                                    <CircularProgress size={12} sx={{ color: "#71717a", mr: 0.75 }} />
                                  ) : null}
                                  Revoke
                                </ButtonBase>
                              )}
                            </Box>
                          </td>
                        )}
                      </tr>
                    );
                  })}
                </tbody>
              </Box>
            </Box>
          )}
        </Paper>
      )}

      {/* Action Menu for Member Actions */}
      <Menu
        anchorEl={menuAnchorEl}
        open={Boolean(menuAnchorEl)}
        onClose={handleCloseMenu}
        slotProps={{
          paper: {
            elevation: 0,
            sx: {
              mt: 0.5,
              minWidth: 160,
              border: "1px solid #e4e4e7",
              borderRadius: "6px",
              boxShadow: "0 4px 12px rgba(0, 0, 0, 0.05)",
              p: 0.5,
            },
          },
        }}
      >
        <MenuItem
          onClick={handleOpenChangeRole}
          disabled={selectedMember?.user_id === currentUser?.id}
          sx={{
            borderRadius: "4px",
            fontSize: "0.8125rem",
            color: "#09090b",
            py: 0.75,
            px: 1.5,
          }}
        >
          Change Role
        </MenuItem>
        <MenuItem
          onClick={handleOpenRemove}
          disabled={selectedMember?.user_id === currentUser?.id}
          sx={{
            borderRadius: "4px",
            fontSize: "0.8125rem",
            color: "#09090b",
            py: 0.75,
            px: 1.5,
          }}
        >
          Remove Member
        </MenuItem>
      </Menu>

      {/* Change Role Dialog */}
      <ChangeRoleDialog
        open={changeRoleOpen}
        onClose={() => setChangeRoleOpen(false)}
        member={selectedMember}
        orgId={currentOrg.id}
      />

      {/* Remove Member Dialog */}
      <RemoveMemberDialog
        open={removeOpen}
        onClose={() => setRemoveOpen(false)}
        member={selectedMember}
        orgId={currentOrg.id}
      />

      {/* Invite Member Dialog */}
      <InviteMemberDialog
        open={inviteDialogOpen}
        onClose={() => setInviteDialogOpen(false)}
        org={currentOrg}
        onSuccess={() => {
          // Switch to invitations tab immediately after sending invite so user sees it right away
          setActiveTab("invitations");
        }}
      />
    </Container>
  );
}
