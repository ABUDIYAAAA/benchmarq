"use client";

import { createContext, useContext, useMemo, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  useCurrentUser,
  useLoginMutation,
  useSignUpMutation,
  useLogoutMutation,
} from "@/hooks/api/useAuth";

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const router = useRouter();

  // Primary user query backed by TanStack Query & cookie session
  const {
    data: user,
    isLoading,
    isFetching,
    error,
    refetch,
  } = useCurrentUser();

  const loginMutation = useLoginMutation();
  const signUpMutation = useSignUpMutation();
  const logoutMutation = useLogoutMutation({
    onSuccess: () => {
      router.push("/login");
    },
  });

  const isAuthenticated = Boolean(user && !error);
  const isEmailVerified = Boolean(user?.is_email_verified);

  const login = useCallback(
    async (credentials) => {
      return await loginMutation.mutateAsync(credentials);
    },
    [loginMutation]
  );

  const signup = useCallback(
    async (payload) => {
      return await signUpMutation.mutateAsync(payload);
    },
    [signUpMutation]
  );

  const logout = useCallback(async () => {
    return await logoutMutation.mutateAsync();
  }, [logoutMutation]);

  const value = useMemo(
    () => ({
      user: user || null,
      isAuthenticated,
      isEmailVerified,
      isLoading,
      isFetching,
      error,
      login,
      signup,
      logout,
      refetchUser: refetch,
      isLoggingIn: loginMutation.isPending,
      isSigningUp: signUpMutation.isPending,
      isLoggingOut: logoutMutation.isPending,
    }),
    [
      user,
      isAuthenticated,
      isEmailVerified,
      isLoading,
      isFetching,
      error,
      login,
      signup,
      logout,
      refetch,
      loginMutation.isPending,
      signUpMutation.isPending,
      logoutMutation.isPending,
    ]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}

export default AuthContext;
