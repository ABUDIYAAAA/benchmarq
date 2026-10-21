"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import orgsApi from "@/lib/api/orgs.api";

export const orgKeys = {
  all: ["orgs"],
  mine: () => [...orgKeys.all, "mine"],
  detail: (orgId) => [...orgKeys.all, orgId],
  members: (orgId) => [...orgKeys.detail(orgId), "members"],
  invitations: (orgId, status) => [...orgKeys.detail(orgId), "invitations", status || "all"],
};

export function useMyOrganizations(options = {}) {
  return useQuery({
    queryKey: orgKeys.mine(),
    queryFn: async () => {
      const response = await orgsApi.listMine();
      // Backend returns pagination.Page[*OrganizationResponse] in response.data
      return response.data?.items || [];
    },
    ...options,
  });
}

export function useCreateOrganizationMutation(options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload) => orgsApi.create(payload),
    onSuccess: (data, variables, context) => {
      const createdOrg = data.data;
      if (createdOrg) {
        queryClient.setQueryData(orgKeys.mine(), (old = []) => {
          if (!old.some((o) => o.id === createdOrg.id)) {
            return [...old, createdOrg];
          }
          return old;
        });
      }
      queryClient.invalidateQueries({ queryKey: orgKeys.mine() });
      options.onSuccess?.(data, variables, context);
    },
    ...options,
  });
}

export function useOrganizationMembers(orgId, options = {}) {
  return useQuery({
    queryKey: orgKeys.members(orgId),
    queryFn: async () => {
      if (!orgId) return [];
      const response = await orgsApi.listMembers(orgId);
      return response.data?.items || [];
    },
    enabled: Boolean(orgId),
    ...options,
  });
}

export function useOrganizationInvitations(orgId, status, options = {}) {
  return useQuery({
    queryKey: orgKeys.invitations(orgId, status),
    queryFn: async () => {
      if (!orgId) return [];
      const response = await orgsApi.listInvitations(orgId, status);
      return response.data?.items || [];
    },
    enabled: Boolean(orgId),
    ...options,
  });
}

export function useCreateInvitationMutation(orgId, options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload) => orgsApi.createInvitation(orgId, payload),
    onSuccess: (data, variables, context) => {
      queryClient.invalidateQueries({ queryKey: orgKeys.invitations(orgId) });
      options.onSuccess?.(data, variables, context);
    },
    ...options,
  });
}

export function useResendInvitationMutation(orgId, options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (invitationId) => orgsApi.resendInvitation(orgId, invitationId),
    onSuccess: (data, variables, context) => {
      queryClient.invalidateQueries({ queryKey: orgKeys.invitations(orgId) });
      options.onSuccess?.(data, variables, context);
    },
    ...options,
  });
}

export function useRevokeInvitationMutation(orgId, options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (invitationId) => orgsApi.revokeInvitation(orgId, invitationId),
    onSuccess: (data, variables, context) => {
      queryClient.invalidateQueries({ queryKey: orgKeys.invitations(orgId) });
      options.onSuccess?.(data, variables, context);
    },
    ...options,
  });
}
