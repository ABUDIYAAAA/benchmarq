// Mirrors the server's role -> permission table (server/internal/authz).
// Roles are per organization, so the same person can author exams in one
// organization and take exams as a candidate in another. The server enforces
// all of this; the UI only uses it to decide what to show.

const STAFF_EXAM = ["owner", "admin", "instructor"];

const PERMISSIONS = {
  viewMembers: ["owner", "admin", "instructor", "proctor"],
  manageMembers: ["owner", "admin"],
  viewExams: STAFF_EXAM, // includes answer keys
  authorExams: STAFF_EXAM,
  publishExams: STAFF_EXAM,
  deleteExams: ["owner", "admin"],
  assignExams: STAFF_EXAM,
  monitorAttempts: [...STAFF_EXAM, "proctor"],
};

/** Reports whether a role grants a permission, e.g. can(org.my_role, "authorExams"). */
export function can(role, permission) {
  return Boolean(role && PERMISSIONS[permission]?.includes(role));
}

/** Candidates have no management access; they only take exams. */
export function isStaff(role) {
  return can(role, "viewMembers");
}

export const ROLE_LABELS = {
  owner: "Owner",
  admin: "Admin",
  instructor: "Instructor",
  proctor: "Proctor",
  candidate: "Candidate",
};
