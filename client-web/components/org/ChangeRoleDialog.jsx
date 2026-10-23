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
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import { useUpdateMemberRoleMutation } from "@/hooks/api/useOrgs";
import { useToast } from "@/context/ToastContext";
import { getApiErrorMessage } from "@/hooks/api/useAuth";

const ROLES = [
  { value: "owner", label: "Owner", desc: "Full ownership of workspace and billing" },
  { value: "admin", label: "Admin", desc: "Full administrative access to tests and members" },
  { value: "instructor", label: "Instructor", desc: "Create, edit, and grade exams" },
  { value: "proctor", label: "Proctor", desc: "Monitor ongoing sessions and candidate integrity" },
  { value: "candidate", label: "Candidate", desc: "Take assigned exams" },
];

export default function ChangeRoleDialog({ open, onClose, member, orgId }) {
  const { showSuccess } = useToast();
  const [role, setRole] = useState(member?.role || "instructor");
  const [feedback, setFeedback] = useState(null);

  const updateMutation = useUpdateMemberRoleMutation(orgId);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!member?.user_id) return;
    try {
      await updateMutation.mutateAsync({
        userId: member.user_id,
        role,
      });
      showSuccess(`Updated role for ${member.first_name} to ${role.toUpperCase()}.`);
      onClose();
    } catch (err) {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(err, "Failed to update member role."),
      });
    }
  };

  return (
    <Dialog
      open={open}
      onClose={() => !updateMutation.isPending && onClose()}
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
          Update Member Role
        </Typography>
        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.5 }}>
          Modify permissions for {member?.first_name} {member?.last_name || ""} ({member?.email}).
        </Typography>
      </DialogTitle>

      <Box component="form" onSubmit={handleSubmit}>
        <DialogContent sx={{ px: 3, py: 1 }}>
          <AuthFeedback
            type={feedback?.type}
            message={feedback?.message}
            onClose={() => setFeedback(null)}
          />

          <Box sx={{ mb: 2 }}>
            <Box
              component="label"
              htmlFor="change-role-select"
              sx={{
                display: "block",
                fontSize: "0.8125rem",
                fontWeight: 500,
                color: "#09090b",
                mb: 0.75,
              }}
            >
              Select New Role
            </Box>
            <FormControl fullWidth size="small">
              <Select
                id="change-role-select"
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
            onClick={onClose}
            disabled={updateMutation.isPending}
            fullWidth={false}
            sx={{ px: 2 }}
          >
            Cancel
          </AuthButton>
          <AuthButton
            type="submit"
            loading={updateMutation.isPending}
            fullWidth={false}
            sx={{ px: 2.5 }}
          >
            Save Role
          </AuthButton>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
