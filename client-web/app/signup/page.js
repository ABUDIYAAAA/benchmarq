"use client";

import { useEffect, useState } from "react";
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
import { useAuth } from "@/context/AuthContext";

export default function SignUpPage() {
  const router = useRouter();
  const { isAuthenticated } = useAuth();

  // Signed-in users (including right after signing up) belong on the dashboard.
  useEffect(() => {
    if (isAuthenticated) router.replace("/dashboard");
  }, [isAuthenticated, router]);
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
        message: getApiErrorMessage(error, "Failed to create your account. Please try again."),
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
        title={createdUser ? "Check Your Email" : "Create Account"}
        subtitle={
          createdUser
            ? "We've sent a verification link to activate your account."
            : "Create an account to take exams or manage your organization."
        }
        footer={
          <Typography sx={{ fontSize: "0.8125rem", color: "#71717a" }}>
            {createdUser ? (
              <>
                Need to sign in?{" "}
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
              </>
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
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
            <Box
              sx={{
                p: 2.5,
                borderRadius: "8px",
                backgroundColor: "#fafafa",
                border: "1px solid #e4e4e7",
                textAlign: "center",
              }}
            >
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
                  margin: "0 auto 12px",
                  color: "#09090b",
                }}
              >
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
                  <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
                  <polyline points="22,6 12,13 2,6" />
                </svg>
              </Box>
              <Typography sx={{ fontSize: "0.875rem", fontWeight: 600, color: "#09090b" }}>
                Verification Link Dispatched
              </Typography>
              <Typography sx={{ fontSize: "0.8125rem", color: "#71717a", mt: 0.5, lineHeight: 1.5 }}>
                We sent a confirmation link to <strong>{createdUser.email}</strong>. Please click the link in your email to verify your address.
              </Typography>
            </Box>

            <AuthButton
              type="button"
              variant="contained"
              onClick={() =>
                router.push(`/login?registered=true&email=${encodeURIComponent(createdUser.email)}`)
              }
            >
              Proceed to Sign In
            </AuthButton>
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
              placeholder="you@example.com"
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
