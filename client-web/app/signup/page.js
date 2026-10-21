"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Divider from "@mui/material/Divider";
import AuthLayout from "@/components/auth/AuthLayout";
import AuthCard from "@/components/auth/AuthCard";
import AuthField from "@/components/auth/AuthField";
import AuthButton from "@/components/auth/AuthButton";
import AuthFeedback from "@/components/auth/AuthFeedback";
import { useSignUpMutation, getApiErrorMessage } from "@/hooks/api/useAuth";

export default function SignUpPage() {
  const router = useRouter();
  const [formData, setFormData] = useState({
    firstName: "",
    lastName: "",
    email: "",
    password: "",
    confirmPassword: "",
  });
  const [feedback, setFeedback] = useState(null);
  const [createdUser, setCreatedUser] = useState(null);

  const signUpMutation = useSignUpMutation({
    onSuccess: (res, variables) => {
      const user = res.data?.user || {
        first_name: variables.first_name,
        email: variables.email,
      };
      setCreatedUser(user);
      setFeedback({
        type: "success",
        message: "Account created successfully. A verification link has been sent to your email.",
      });
    },
    onError: (error) => {
      setFeedback({
        type: "error",
        message: getApiErrorMessage(error, "Failed to create administrator account. Please try again."),
      });
    },
  });

  const handleChange = (e) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
    if (feedback) setFeedback(null);
  };

  const handleSubmit = (e) => {
    e.preventDefault();

    if (!formData.firstName.trim()) {
      setFeedback({ type: "error", message: "First name is required." });
      return;
    }
    if (!formData.email.trim()) {
      setFeedback({ type: "error", message: "A valid email address is required." });
      return;
    }
    if (formData.password.length < 8) {
      setFeedback({
        type: "error",
        message: "Password must be at least 8 characters long.",
      });
      return;
    }
    if (formData.password !== formData.confirmPassword) {
      setFeedback({ type: "error", message: "Passwords do not match." });
      return;
    }

    signUpMutation.mutate({
      first_name: formData.firstName.trim(),
      last_name: formData.lastName.trim() || undefined,
      email: formData.email.trim(),
      password: formData.password,
    });
  };

  return (
    <AuthLayout>
      <AuthCard
        title={createdUser ? "Account Created" : "Create Admin Account"}
        subtitle={
          createdUser
            ? "Your administrator account has been provisioned successfully."
            : "Set up your primary administrator credentials to manage the platform."
        }
        footer={
          <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
            {createdUser ? (
              <Link
                href={`/login?registered=true&email=${encodeURIComponent(createdUser.email)}`}
                style={{
                  color: "#09090b",
                  fontWeight: 600,
                  textDecoration: "underline",
                  textUnderlineOffset: 3,
                }}
              >
                Proceed to sign in
              </Link>
            ) : (
              <>
                Already registered?{" "}
                <Link
                  href="/login"
                  style={{
                    color: "#09090b",
                    fontWeight: 600,
                    textDecoration: "underline",
                    textUnderlineOffset: 3,
                  }}
                >
                  Sign in
                </Link>
              </>
            )}
          </Typography>
        }
      >
        <AuthFeedback
          type={feedback?.type}
          message={feedback?.message}
          onClose={() => setFeedback(null)}
        />

        {createdUser ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Box
              sx={{
                p: 2,
                borderRadius: "6px",
                backgroundColor: "#fafafa",
                border: "1px solid #e4e4e7",
              }}
            >
              <Typography sx={{ fontSize: "0.75rem", color: "#71717a", textTransform: "uppercase", letterSpacing: "0.06em", mb: 0.5 }}>
                Registered Account
              </Typography>
              <Typography sx={{ fontSize: "0.875rem", fontWeight: 600, color: "#09090b" }}>
                {createdUser.first_name} {createdUser.last_name || ""}
              </Typography>
              <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
                {createdUser.email}
              </Typography>
            </Box>

            <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", lineHeight: 1.45 }}>
              A confirmation email has been dispatched. You may verify your email token now, continue directly into the admin console, or sign in.
            </Typography>

            <Divider sx={{ my: 1, borderColor: "#f4f4f5" }} />

            <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <AuthButton
                type="button"
                onClick={() => router.push("/dashboard")}
              >
                Continue to Admin Console
              </AuthButton>

              <AuthButton
                type="button"
                variant="outlined"
                onClick={() =>
                  router.push(`/login?registered=true&email=${encodeURIComponent(createdUser.email)}`)
                }
              >
                Sign In With Password
              </AuthButton>

              <AuthButton
                type="button"
                variant="outlined"
                onClick={() =>
                  router.push(`/verify-email?email=${encodeURIComponent(createdUser.email)}`)
                }
              >
                Verify Email Token
              </AuthButton>
            </Box>
          </Box>
        ) : (
          <Box component="form" onSubmit={handleSubmit} noValidate>
            <Box sx={{ display: "flex", gap: 1.5 }}>
              <AuthField
                id="signup-first-name"
                name="firstName"
                label="First Name"
                placeholder="Jane"
                value={formData.firstName}
                onChange={handleChange}
                required
                autoComplete="given-name"
                autoFocus
              />
              <AuthField
                id="signup-last-name"
                name="lastName"
                label="Last Name"
                placeholder="Doe"
                value={formData.lastName}
                onChange={handleChange}
                autoComplete="family-name"
              />
            </Box>

            <AuthField
              id="signup-email"
              name="email"
              type="email"
              label="Work Email"
              placeholder="admin@benchmarq.io"
              value={formData.email}
              onChange={handleChange}
              required
              autoComplete="email"
            />

            <AuthField
              id="signup-password"
              name="password"
              type="password"
              label="Password"
              placeholder="Minimum 8 characters"
              value={formData.password}
              onChange={handleChange}
              required
              autoComplete="new-password"
            />

            <AuthField
              id="signup-confirm-password"
              name="confirmPassword"
              type="password"
              label="Confirm Password"
              placeholder="Repeat password"
              value={formData.confirmPassword}
              onChange={handleChange}
              required
              autoComplete="new-password"
            />

            <AuthButton
              type="submit"
              loading={signUpMutation.isPending}
              sx={{ mt: 1 }}
            >
              Create Account
            </AuthButton>
          </Box>
        )}
      </AuthCard>
    </AuthLayout>
  );
}
