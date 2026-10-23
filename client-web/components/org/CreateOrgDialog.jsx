"use client";

import { useState } from "react";
import Dialog from "@mui/material/Dialog";
import DialogTitle from "@mui/material/DialogTitle";
import DialogContent from "@mui/material/DialogContent";
import DialogActions from "@mui/material/DialogActions";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import AuthField from "@/components/auth/AuthField";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import { useOrg } from "@/context/OrgContext";
import { useToast } from "@/context/ToastContext";
import { getApiErrorMessage } from "@/hooks/api/useAuth";

function slugify(text) {
  return text
    .toString()
    .toLowerCase()
    .trim()
    .replace(/\s+/g, "-")
    .replace(/[^\w\-]+/g, "")
    .replace(/\-\-+/g, "-")
    .replace(/^-+/, "")
    .replace(/-+$/, "");
}

export default function CreateOrgDialog({ open, onClose }) {
  const { createOrg, isCreatingOrg } = useOrg();
  const { showSuccess } = useToast();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugCustomized, setSlugCustomized] = useState(false);
  const [website, setWebsite] = useState("");
  const [feedback, setFeedback] = useState(null);

  const handleNameChange = (e) => {
    const val = e.target.value;
    setName(val);
    if (!slugCustomized) {
      setSlug(slugify(val));
    }
    if (feedback) setFeedback(null);
  };

  const handleSlugChange = (e) => {
    setSlugCustomized(true);
    setSlug(slugify(e.target.value));
    if (feedback) setFeedback(null);
  };

  const handleClose = () => {
    if (isCreatingOrg) return;
    setName("");
    setSlug("");
    setWebsite("");
    setSlugCustomized(false);
    setFeedback(null);
    onClose();
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!name.trim()) {
      setFeedback({ type: "error", message: "Organization name is required." });
      return;
    }

    try {
      const res = await createOrg({
        name: name.trim(),
        slug: slug.trim() || undefined,
        website: website.trim() || undefined,
      });
      const orgName = res?.name || name.trim();
      handleClose();
      showSuccess(`Organization "${orgName}" created.`);
    } catch (error) {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(error, "Failed to create organization."),
      });
    }
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
          Create Organization
        </Typography>
        <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.5 }}>
          Set up a collaborative workspace for assessments and staff.
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
            id="org-name"
            name="name"
            label="Organization Name"
            placeholder="e.g. Stanford University or Acme Corp"
            value={name}
            onChange={handleNameChange}
            required
            autoFocus
          />

          <AuthField
            id="org-slug"
            name="slug"
            label="Workspace Identifier (Slug)"
            placeholder="e.g. acme-corp"
            value={slug}
            onChange={handleSlugChange}
            helperText="Used in public URLs and candidate portals."
          />

          <AuthField
            id="org-website"
            name="website"
            type="url"
            label="Website"
            placeholder="https://example.com"
            value={website}
            onChange={(e) => setWebsite(e.target.value)}
          />
        </DialogContent>

        <DialogActions sx={{ px: 3, pb: 2, pt: 1, gap: 1 }}>
          <AuthButton
            type="button"
            variant="outlined"
            onClick={handleClose}
            disabled={isCreatingOrg}
            fullWidth={false}
            sx={{ px: 2 }}
          >
            Cancel
          </AuthButton>
          <AuthButton
            type="submit"
            loading={isCreatingOrg}
            fullWidth={false}
            sx={{ px: 2.5 }}
          >
            Create Organization
          </AuthButton>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
