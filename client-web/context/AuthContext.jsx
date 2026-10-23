"use client";

import { createContext, useContext, useMemo, useCallback, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { SESSION_EXPIRED_EVENT } from "@/lib/api/axios";
import {
  useCurrentUser,
  useLoginMutation,
  useSignUpMutation,
  authKeys,
  useLogoutMutation,
} from "@/hooks/api/useAuth";

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const router = useRouter();
  const queryClient = useQueryClient();

  // A failed token refresh ends the session everywhere in the app.
  useEffect(() => {
    // Visitors on public pages were never signed in; only redirect when a
    // known user just lost their session.
    const onExpired = () => {
      const wasSignedIn = Boolean(queryClient.getQueryData(authKeys.me()));
      queryClient.clear();
      if (wasSignedIn) router.push("/login");
    };
    window.addEventListener(SESSION_EXPIRED_EVENT, onExpired);
    return () => window.removeEventListener(SESSION_EXPIRED_EVENT, onExpired);
  }, [queryClient, router]);

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
