"use client";

import { useState } from "react";
import Dialog from "@mui/material/Dialog";
import DialogTitle from "@mui/material/DialogTitle";
import DialogContent from "@mui/material/DialogContent";
import DialogActions from "@mui/material/DialogActions";
import Typography from "@mui/material/Typography";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import { useRemoveMemberMutation } from "@/hooks/api/useOrgs";
import { useToast } from "@/context/ToastContext";
import { getApiErrorMessage } from "@/hooks/api/useAuth";

export default function RemoveMemberDialog({ open, onClose, member, orgId }) {
  const { showSuccess } = useToast();
  const [feedback, setFeedback] = useState(null);

  const removeMutation = useRemoveMemberMutation(orgId);

  const handleConfirm = async () => {
    if (!member?.user_id) return;
    try {
      await removeMutation.mutateAsync(member.user_id);
      showSuccess(`Removed ${member.first_name} from organization.`);
      onClose();
    } catch (err) {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(err, "Failed to remove member."),
      });
    }
  };

  return (
    <Dialog
      open={open}
      onClose={() => !removeMutation.isPending && onClose()}
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
          Remove Member
        </Typography>
      </DialogTitle>

      <DialogContent sx={{ px: 3, py: 1 }}>
        <AuthFeedback
          type={feedback?.type}
          message={feedback?.message}
          onClose={() => setFeedback(null)}
        />

        <Typography sx={{ fontSize: "0.875rem", color: "#09090b", lineHeight: 1.5 }}>
          Are you sure you want to remove{" "}
          <strong>
            {member?.first_name} {member?.last_name || ""}
          </strong>{" "}
          ({member?.email}) from this organization?
        </Typography>
        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 1 }}>
          Their access to exams, candidate pools, and organization resources will be revoked immediately.
        </Typography>
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 2, pt: 1, gap: 1 }}>
        <AuthButton
          type="button"
          variant="outlined"
          onClick={onClose}
          disabled={removeMutation.isPending}
          fullWidth={false}
          sx={{ px: 2 }}
        >
          Cancel
        </AuthButton>
        <AuthButton
          type="button"
          loading={removeMutation.isPending}
          onClick={handleConfirm}
          fullWidth={false}
          sx={{
            px: 2.5,
            backgroundColor: "#09090b",
            "&:hover": { backgroundColor: "#27272a" },
          }}
        >
          Confirm Removal
        </AuthButton>
      </DialogActions>
    </Dialog>
  );
}
