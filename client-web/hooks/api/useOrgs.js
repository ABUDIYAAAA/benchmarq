"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import orgsApi from "@/lib/api/orgs.api";

export const orgKeys = {
  all: ["orgs"],
  mine: () => [...orgKeys.all, "mine"],
  detail: (orgId) => [...orgKeys.all, orgId],
  members: (orgId) => [...orgKeys.detail(orgId), "members"],
  invitations: (orgId, status) =>
    status
      ? [...orgKeys.detail(orgId), "invitations", status]
      : [...orgKeys.detail(orgId), "invitations"],
};

export function extractOrgList(response) {
  if (!response) return [];
  if (Array.isArray(response)) return response;
  if (Array.isArray(response.data)) return response.data;
  if (Array.isArray(response.data?.items)) return response.data.items;
  if (Array.isArray(response.items)) return response.items;
  return [];
}

export function useMyOrganizations(options = {}) {
  return useQuery({
    // The list carries the user's role in each organization, which another
    // admin can change at any time: re-check when the user comes back to the tab.
    refetchOnWindowFocus: true,
    staleTime: 30 * 1000,
    queryKey: orgKeys.mine(),
    queryFn: async () => {
      const response = await orgsApi.listMine();
      return extractOrgList(response);
    },
    ...options,
  });
}

export function useCreateOrganizationMutation(options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: (payload) => orgsApi.create(payload),
    onSuccess: (data, variables, context) => {
      const createdOrg = data?.data?.id ? data.data : (data?.id ? data : null);
      if (createdOrg) {
        queryClient.setQueryData(orgKeys.mine(), (old = []) => {
          const list = Array.isArray(old) ? old : [];
          if (!list.some((o) => o.id === createdOrg.id)) {
            return [createdOrg, ...list];
          }
          return list;
        });
      }
      queryClient.invalidateQueries({ queryKey: orgKeys.mine() });
      options.onSuccess?.(data, variables, context);
    },
  });
}

export function useOrganizationMembers(orgId, options = {}) {
  return useQuery({
    queryKey: orgKeys.members(orgId),
    queryFn: async () => {
      if (!orgId) return [];
      const response = await orgsApi.listMembers(orgId);
      return extractOrgList(response);
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
      return extractOrgList(response);
    },
    enabled: Boolean(orgId),
    ...options,
  });
}

export function useCreateInvitationMutation(orgId, options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: (payload) => orgsApi.createInvitation(orgId, payload),
    onSuccess: (data, variables, context) => {
      const newInv = data?.data?.id ? data.data : (data?.id ? data : null);
      if (newInv) {
        queryClient.setQueriesData(
          { queryKey: [...orgKeys.detail(orgId), "invitations"] },
          (old = []) => {
            const list = Array.isArray(old) ? old : [];
            if (!list.some((i) => i.id === newInv.id)) {
              return [newInv, ...list];
            }
            return list;
          }
        );
      }
      queryClient.invalidateQueries({
        queryKey: [...orgKeys.detail(orgId), "invitations"],
      });
      options.onSuccess?.(data, variables, context);
    },
  });
}

export function useResendInvitationMutation(orgId, options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: (invitationId) => orgsApi.resendInvitation(orgId, invitationId),
    onSuccess: (data, variables, context) => {
      queryClient.invalidateQueries({
        queryKey: [...orgKeys.detail(orgId), "invitations"],
      });
      options.onSuccess?.(data, variables, context);
    },
  });
}

export function useRevokeInvitationMutation(orgId, options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: (invitationId) => orgsApi.revokeInvitation(orgId, invitationId),
    onSuccess: (data, variables, context) => {
      queryClient.setQueriesData(
        { queryKey: [...orgKeys.detail(orgId), "invitations"] },
        (old = []) => {
          if (!Array.isArray(old)) return [];
          return old.filter((i) => i.id !== variables);
        }
      );
      queryClient.invalidateQueries({
        queryKey: [...orgKeys.detail(orgId), "invitations"],
      });
      options.onSuccess?.(data, variables, context);
    },
  });
}

export function useUpdateMemberRoleMutation(orgId, options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: ({ userId, role }) => orgsApi.updateMemberRole(orgId, userId, role),
    onSuccess: (data, variables, context) => {
      queryClient.setQueriesData(
        { queryKey: orgKeys.members(orgId) },
        (old = []) => {
          if (!Array.isArray(old)) return [];
          return old.map((m) =>
            m.user_id === variables.userId ? { ...m, role: variables.role } : m
          );
        }
      );
      queryClient.invalidateQueries({ queryKey: orgKeys.members(orgId) });
      queryClient.invalidateQueries({ queryKey: orgKeys.mine() });
      options.onSuccess?.(data, variables, context);
    },
  });
}

export function useRemoveMemberMutation(orgId, options = {}) {
  const queryClient = useQueryClient();

  return useMutation({
    ...options,
    mutationFn: (userId) => orgsApi.removeMember(orgId, userId),
    onSuccess: (data, variables, context) => {
      queryClient.setQueriesData(
        { queryKey: orgKeys.members(orgId) },
        (old = []) => {
          if (!Array.isArray(old)) return [];
          return old.filter((m) => m.user_id !== variables);
        }
      );
      queryClient.invalidateQueries({ queryKey: orgKeys.members(orgId) });
      queryClient.invalidateQueries({ queryKey: orgKeys.mine() });
      options.onSuccess?.(data, variables, context);
    },
  });
}
