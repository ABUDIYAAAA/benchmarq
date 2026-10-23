import apiClient from "./axios";

// Every endpoint returns the standard envelope { success, message, data };
// these helpers return `data`.
const data = (res) => res.data?.data;

const examPath = (orgId, examId) => `/orgs/${orgId}/exams/${examId}`;
const sectionPath = (orgId, examId, sectionId) => `${examPath(orgId, examId)}/sections/${sectionId}`;

export const examsApi = {
  // Exams
  list: (orgId, params) => apiClient.get(`/orgs/${orgId}/exams`, { params }).then(data),
  create: (orgId, body) => apiClient.post(`/orgs/${orgId}/exams`, body).then(data),
  get: (orgId, examId) => apiClient.get(examPath(orgId, examId)).then(data),
  update: (orgId, examId, patch) => apiClient.patch(examPath(orgId, examId), patch).then(data),
  remove: (orgId, examId) => apiClient.delete(examPath(orgId, examId)).then(data),
  updateConfig: (orgId, examId, patch) => apiClient.patch(`${examPath(orgId, examId)}/config`, patch).then(data),
  readiness: (orgId, examId) => apiClient.get(`${examPath(orgId, examId)}/readiness`).then(data),
  transition: (orgId, examId, action) => apiClient.post(`${examPath(orgId, examId)}/${action}`, {}).then(data),

  // Sections
  createSection: (orgId, examId, body) => apiClient.post(`${examPath(orgId, examId)}/sections`, body).then(data),
  updateSection: (orgId, examId, sectionId, patch) => apiClient.patch(sectionPath(orgId, examId, sectionId), patch).then(data),
  deleteSection: (orgId, examId, sectionId) => apiClient.delete(sectionPath(orgId, examId, sectionId)).then(data),
  reorderSections: (orgId, examId, sectionIds) =>
    apiClient.put(`${examPath(orgId, examId)}/sections/order`, { section_ids: sectionIds }).then(data),

  // Questions
  listQuestions: (orgId, examId, sectionId) => apiClient.get(`${sectionPath(orgId, examId, sectionId)}/questions`).then(data),
  createQuestion: (orgId, examId, sectionId, body) => apiClient.post(`${sectionPath(orgId, examId, sectionId)}/questions`, body).then(data),
  replaceQuestion: (orgId, examId, sectionId, questionId, body) =>
    apiClient.put(`${sectionPath(orgId, examId, sectionId)}/questions/${questionId}`, body).then(data),
  deleteQuestion: (orgId, examId, sectionId, questionId) =>
    apiClient.delete(`${sectionPath(orgId, examId, sectionId)}/questions/${questionId}`).then(data),
  reorderQuestions: (orgId, examId, sectionId, questionIds) =>
    apiClient.put(`${sectionPath(orgId, examId, sectionId)}/questions/order`, { question_ids: questionIds }).then(data),

  // Candidates
  listAssignments: (orgId, examId) => apiClient.get(`${examPath(orgId, examId)}/assignments`, { params: { limit: 100 } }).then(data),
  assign: (orgId, examId, body) => apiClient.post(`${examPath(orgId, examId)}/assignments`, body).then(data),
  unassign: (orgId, examId, assignmentId) => apiClient.delete(`${examPath(orgId, examId)}/assignments/${assignmentId}`).then(data),
  listMembers: (orgId) => apiClient.get(`/orgs/${orgId}/members`, { params: { limit: 100 } }).then(data),

  // Attempts
  listAttempts: (orgId, examId) => apiClient.get(`${examPath(orgId, examId)}/attempts`, { params: { limit: 100 } }).then(data),
  releaseDevice: (orgId, examId, attemptId) => apiClient.post(`${examPath(orgId, examId)}/attempts/${attemptId}/release-device`, {}).then(data),

  // Candidate portal
  myExams: () => apiClient.get("/me/exams").then(data),
  launch: (examId) => apiClient.post(`/me/exams/${examId}/launch`, {}).then(data),
};

export default examsApi;
