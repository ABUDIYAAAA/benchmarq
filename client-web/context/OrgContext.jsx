"use client";

import { createContext, useContext, useState, useMemo, useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useMyOrganizations, useCreateOrganizationMutation, orgKeys } from "@/hooks/api/useOrgs";
import { useAuth } from "@/context/AuthContext";

const OrgContext = createContext(null);
const STORAGE_KEY = "benchmarq_active_org_id";

export function OrgProvider({ children }) {
  const queryClient = useQueryClient();
  const { isAuthenticated } = useAuth();
  // Organizations only exist for signed-in users; skipping the request on
  // public pages avoids a pointless 401 and token refresh.
  const { data: organizations = [], isPending, refetch: refetchOrgs } = useMyOrganizations({ enabled: isAuthenticated });
  // Until the list has been fetched once, "no organizations" is unknown, not
  // empty: treating it as empty flashes the create-organization screen.
  const isLoadingOrgs = isAuthenticated && isPending;

  const [selectedOrgId, setSelectedOrgId] = useState(() => {
    if (typeof window !== "undefined") {
      return localStorage.getItem(STORAGE_KEY) || null;
    }
    return null;
  });

  const currentOrg = useMemo(() => {
    if (!organizations || organizations.length === 0) return null;
    if (selectedOrgId) {
      const match = organizations.find((o) => o.id === selectedOrgId);
      if (match) return match;
    }
    return organizations[0];
  }, [organizations, selectedOrgId]);

  const selectOrg = useCallback((orgOrId) => {
    const id = typeof orgOrId === "object" ? orgOrId.id : orgOrId;
    setSelectedOrgId(id);
    if (typeof window !== "undefined" && id) {
      localStorage.setItem(STORAGE_KEY, id);
    }
  }, []);

  const createMutation = useCreateOrganizationMutation();

  const createOrg = useCallback(
    async (payload) => {
      const res = await createMutation.mutateAsync(payload);
      const newOrg = res?.data?.id ? res.data : (res?.id ? res : null);

      if (newOrg?.id) {
        // 1. Synchronously inject new organization into TanStack cache
        queryClient.setQueryData(orgKeys.mine(), (old) => {
          const list = Array.isArray(old) ? old : [];
          if (!list.some((o) => o.id === newOrg.id)) {
            return [newOrg, ...list];
          }
          return list;
        });

        // 2. Select newly created organization
        selectOrg(newOrg.id);
      }

      // 3. Guarantee fresh server sync
      await refetchOrgs();
      return res;
    },
    [createMutation, queryClient, selectOrg, refetchOrgs]
  );

  const value = useMemo(
    () => ({
      organizations,
      currentOrg,
      selectOrg,
      hasOrgs: organizations.length > 0,
      isLoadingOrgs,
      refetchOrgs,
      createOrg,
      isCreatingOrg: createMutation.isPending,
    }),
    [
      organizations,
      currentOrg,
      selectOrg,
      isLoadingOrgs,
      refetchOrgs,
      createOrg,
      createMutation.isPending,
    ]
  );

  return <OrgContext.Provider value={value}>{children}</OrgContext.Provider>;
}

export function useOrg() {
  const context = useContext(OrgContext);
  if (!context) {
    throw new Error("useOrg must be used within an OrgProvider");
  }
  return context;
}

export default OrgContext;
