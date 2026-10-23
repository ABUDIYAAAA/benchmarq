"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import examsApi from "@/lib/api/exams.api";

export const examKeys = {
  all: ["exams"],
  list: (orgId, filters) => ["exams", orgId, "list", filters],
  detail: (orgId, examId) => ["exams", orgId, examId],
  readiness: (orgId, examId) => ["exams", orgId, examId, "readiness"],
  questions: (orgId, examId, sectionId) => ["exams", orgId, examId, "questions", sectionId],
  assignments: (orgId, examId) => ["exams", orgId, examId, "assignments"],
  attempts: (orgId, examId) => ["exams", orgId, examId, "attempts"],
  members: (orgId) => ["exams", orgId, "members"],
  mine: ["me", "exams"],
};

export function useExamList(orgId, filters) {
  return useQuery({
    queryKey: examKeys.list(orgId, filters),
    queryFn: () => examsApi.list(orgId, filters),
    enabled: Boolean(orgId),
  });
}

export function useExam(orgId, examId) {
  return useQuery({
    queryKey: examKeys.detail(orgId, examId),
    queryFn: () => examsApi.get(orgId, examId),
    enabled: Boolean(orgId && examId),
  });
}

export function useReadiness(orgId, examId, enabled = true) {
  return useQuery({
    queryKey: examKeys.readiness(orgId, examId),
    queryFn: () => examsApi.readiness(orgId, examId),
    enabled: Boolean(orgId && examId && enabled),
  });
}

export function useQuestions(orgId, examId, sectionId) {
  return useQuery({
    queryKey: examKeys.questions(orgId, examId, sectionId),
    queryFn: () => examsApi.listQuestions(orgId, examId, sectionId),
    enabled: Boolean(orgId && examId && sectionId),
  });
}

export function useAssignments(orgId, examId) {
  return useQuery({
    queryKey: examKeys.assignments(orgId, examId),
    queryFn: () => examsApi.listAssignments(orgId, examId),
    enabled: Boolean(orgId && examId),
  });
}

export function useAttempts(orgId, examId) {
  return useQuery({
    queryKey: examKeys.attempts(orgId, examId),
    queryFn: () => examsApi.listAttempts(orgId, examId),
    enabled: Boolean(orgId && examId),
    refetchInterval: 30_000,
  });
}

export function useOrgMembersForAssign(orgId, enabled) {
  return useQuery({
    queryKey: examKeys.members(orgId),
    queryFn: () => examsApi.listMembers(orgId),
    enabled: Boolean(orgId && enabled),
  });
}

export function useMyExams() {
  return useQuery({ queryKey: examKeys.mine, queryFn: examsApi.myExams, refetchInterval: 60_000 });
}

/**
 * Mutation that refreshes everything about one exam afterwards: structure,
 * readiness, questions and assignments all depend on each other.
 */
export function useExamMutation(orgId, examId, mutationFn) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: examKeys.detail(orgId, examId) });
      queryClient.invalidateQueries({ queryKey: ["exams", orgId, "list"] });
    },
  });
}

export function useCreateExam(orgId) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body) => examsApi.create(orgId, body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["exams", orgId, "list"] }),
  });
}

/** Turns an API error into { message, fields } for forms. */
export function apiError(err, fallback = "Something went wrong") {
  const body = err?.response?.data;
  return { message: body?.message || err?.message || fallback, fields: body?.errors || {} };
}
