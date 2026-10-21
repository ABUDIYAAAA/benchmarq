"use client";

import { createContext, useContext, useState, useMemo, useCallback } from "react";
import { useMyOrganizations, useCreateOrganizationMutation } from "@/hooks/api/useOrgs";

const OrgContext = createContext(null);
const STORAGE_KEY = "benchmarq_active_org_id";

export function OrgProvider({ children }) {
  const { data: organizations = [], isLoading: isLoadingOrgs, refetch: refetchOrgs } = useMyOrganizations();

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

  const createMutation = useCreateOrganizationMutation({
    onSuccess: (res) => {
      const newOrg = res.data;
      if (newOrg?.id) {
        selectOrg(newOrg.id);
      }
    },
  });

  const createOrg = useCallback(
    async (payload) => {
      return await createMutation.mutateAsync(payload);
    },
    [createMutation]
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
