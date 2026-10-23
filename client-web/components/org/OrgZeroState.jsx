"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import Divider from "@mui/material/Divider";
import AuthField from "@/components/auth/AuthField";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import { useOrg } from "@/context/OrgContext";
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

export default function OrgZeroState() {
  const { createOrg, isCreatingOrg } = useOrg();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugCustomized, setSlugCustomized] = useState(false);
  const [feedback, setFeedback] = useState(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

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

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!name.trim()) {
      setFeedback({ type: "error", message: "Please enter an organization name." });
      return;
    }

    setIsSubmitting(true);
    try {
      await createOrg({
        name: name.trim(),
        slug: slug.trim() || undefined,
      });
    } catch (err) {
      setIsSubmitting(false);
      setFeedback({
        type: "error",
        message: getApiErrorMessage(err, "Failed to create organization."),
      });
    }
  };

  return (
    <Box
      sx={{
        maxWidth: 520,
        mx: "auto",
        py: { xs: 4, md: 8 },
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
      }}
    >
      <Paper
        elevation={0}
        sx={{
          width: "100%",
          p: { xs: 3, sm: 4.5 },
          backgroundColor: "#ffffff",
          border: "1px solid #e4e4e7",
          borderRadius: 2,
        }}
      >
        <Box sx={{ mb: 3 }}>
          <Box
            sx={{
              display: "inline-block",
              fontSize: "0.6875rem",
              fontWeight: 700,
              textTransform: "uppercase",
              letterSpacing: "0.08em",
              color: "#71717a",
              mb: 1,
            }}
          >
            Workspace Onboarding
          </Box>
          <Typography
            component="h1"
            sx={{
              fontSize: "1.375rem",
              fontWeight: 600,
              color: "#09090b",
              letterSpacing: "-0.015em",
              lineHeight: 1.25,
            }}
          >
            Create an organization
          </Typography>
          <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.75, lineHeight: 1.5 }}>
            Exams, questions, candidate evaluations, and role permissions are scoped to organizations. Create your workspace to begin authoring tests and inviting your team.
          </Typography>
        </Box>

        <AuthFeedback
          type={feedback?.type}
          message={feedback?.message}
          onClose={() => setFeedback(null)}
        />

        <Box component="form" onSubmit={handleSubmit} noValidate>
          <AuthField
            id="zero-org-name"
            name="name"
            label="Organization Name"
            placeholder="e.g. Stanford University or Acme Labs"
            value={name}
            onChange={handleNameChange}
            required
            autoFocus
          />

          <AuthField
            id="zero-org-slug"
            name="slug"
            label="Organization Identifier"
            placeholder="acme-labs"
            value={slug}
            onChange={handleSlugChange}
            helperText={slug ? `benchmarq.io/org/${slug}` : "Used for URLs and candidate invitations."}
          />

          <AuthButton
            type="submit"
            loading={isCreatingOrg || isSubmitting}
            sx={{ mt: 1.5 }}
          >
            Create Workspace
          </AuthButton>
        </Box>

        <Divider sx={{ my: 3, borderColor: "#f4f4f5" }} />

        <Box sx={{ textAlign: "center" }}>
          <Typography sx={{ fontSize: "0.75rem", color: "#71717a" }}>
            Invited by a team administrator? Check your inbox for your invitation link or contact your organization owner.
          </Typography>
        </Box>
      </Paper>
    </Box>
  );
}
