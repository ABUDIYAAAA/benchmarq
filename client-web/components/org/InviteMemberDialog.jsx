"use client";

import { useState } from "react";
import Dialog from "@mui/material/Dialog";
import DialogTitle from "@mui/material/DialogTitle";
import DialogContent from "@mui/material/DialogContent";
import DialogActions from "@mui/material/DialogActions";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import FormControl from "@mui/material/FormControl";
import Select from "@mui/material/Select";
import MenuItem from "@mui/material/MenuItem";
import AuthField from "@/components/auth/AuthField";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import { useCreateInvitationMutation } from "@/hooks/api/useOrgs";
import { getApiErrorMessage } from "@/hooks/api/useAuth";

const ROLES = [
  { value: "admin", label: "Admin", desc: "Full administrative access to tests, members, and settings" },
  { value: "instructor", label: "Instructor", desc: "Create, edit, and grade exams and question banks" },
  { value: "proctor", label: "Proctor", desc: "Monitor ongoing sessions and candidate integrity" },
  { value: "candidate", label: "Candidate", desc: "Take assigned exams and view permitted results" },
];

export default function InviteMemberDialog({ open, onClose, org }) {
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("instructor");
  const [feedback, setFeedback] = useState(null);

  const inviteMutation = useCreateInvitationMutation(org?.id, {
    onSuccess: () => {
      handleClose();
    },
    onError: (error) => {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(error, "Failed to send invitation."),
      });
    },
  });

  const handleClose = () => {
    if (inviteMutation.isPending) return;
    setEmail("");
    setRole("instructor");
    setFeedback(null);
    onClose();
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!email.trim()) {
      setFeedback({ type: "error", message: "Email address is required." });
      return;
    }

    inviteMutation.mutate({
      email: email.trim(),
      role,
    });
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="xs"
      fullWidth
      slotProps={{
        paper: {
          elevation: 0,
          sx: {
            border: "1px solid #e4e4e7",
            borderRadius: 2,
            p: 1,
          },
        },
      }}
    >
      <DialogTitle sx={{ pb: 1, pt: 2, px: 3 }}>
        <Typography sx={{ fontSize: "1.125rem", fontWeight: 600, color: "#09090b" }}>
          Invite to {org?.name || "Organization"}
        </Typography>
        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.5 }}>
          Send an email invitation with defined role permissions.
        </Typography>
      </DialogTitle>

      <Box component="form" onSubmit={handleSubmit} noValidate>
        <DialogContent sx={{ px: 3, py: 1 }}>
          <AuthFeedback
            type={feedback?.type}
            message={feedback?.message}
            onClose={() => setFeedback(null)}
          />

          <AuthField
            id="invite-email"
            name="email"
            type="email"
            label="Email Address"
            placeholder="colleague@institution.edu"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value);
              if (feedback) setFeedback(null);
            }}
            required
            autoFocus
          />

          <Box sx={{ mb: 2 }}>
            <Box
              component="label"
              htmlFor="invite-role"
              sx={{
                display: "block",
                fontSize: "0.8125rem",
                fontWeight: 500,
                color: "#09090b",
                mb: 0.75,
              }}
            >
              Assigned Role *
            </Box>
            <FormControl fullWidth size="small">
              <Select
                id="invite-role"
                value={role}
                onChange={(e) => setRole(e.target.value)}
                sx={{
                  borderRadius: "6px",
                  fontSize: "0.875rem",
                  "& .MuiOutlinedInput-notchedOutline": {
                    borderColor: "#e4e4e7",
                  },
                  "&:hover .MuiOutlinedInput-notchedOutline": {
                    borderColor: "#a1a1aa",
                  },
                  "&.Mui-focused .MuiOutlinedInput-notchedOutline": {
                    borderColor: "#09090b",
                    borderWidth: "1.5px",
                  },
                }}
              >
                {ROLES.map((r) => (
                  <MenuItem key={r.value} value={r.value}>
                    <Box sx={{ py: 0.5 }}>
                      <Typography sx={{ fontSize: "0.875rem", fontWeight: 500, color: "#09090b" }}>
                        {r.label}
                      </Typography>
                      <Typography sx={{ fontSize: "0.75rem", color: "#71717a" }}>
                        {r.desc}
                      </Typography>
                    </Box>
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Box>
        </DialogContent>

        <DialogActions sx={{ px: 3, pb: 2, pt: 1, gap: 1 }}>
          <AuthButton
            type="button"
            variant="outlined"
            onClick={handleClose}
            disabled={inviteMutation.isPending}
            fullWidth={false}
            sx={{ px: 2 }}
          >
            Cancel
          </AuthButton>
          <AuthButton
            type="submit"
            loading={inviteMutation.isPending}
            fullWidth={false}
            sx={{ px: 2.5 }}
          >
            Send Invitation
          </AuthButton>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
