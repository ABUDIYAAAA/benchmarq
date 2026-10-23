import apiClient from "./axios";

export const orgsApi = {
  /**
   * List all organizations current user belongs to
   */
  async listMine() {
    const res = await apiClient.get("/orgs");
    return res.data;
  },

  /**
   * Create a new organization
   * @param {{ name: string, slug?: string, website?: string, logo_url?: string }} data
   */
  async create(data) {
    const res = await apiClient.post("/orgs", data);
    return res.data;
  },

  /**
   * Get single organization details by ID
   * @param {string} orgId
   */
  async get(orgId) {
    const res = await apiClient.get(`/orgs/${orgId}`);
    return res.data;
  },

  /**
   * List members of an organization
   * @param {string} orgId
   */
  async listMembers(orgId) {
    const res = await apiClient.get(`/orgs/${orgId}/members`);
    return res.data;
  },

  /**
   * List invitations of an organization
   * @param {string} orgId
   * @param {string} [status]
   */
  async listInvitations(orgId, status) {
    const params = status ? { status } : {};
    const res = await apiClient.get(`/orgs/${orgId}/invitations`, { params });
    return res.data;
  },

  /**
   * Invite member to organization
   * @param {string} orgId
   * @param {{ email: string, role: string }} data
   */
  async createInvitation(orgId, data) {
    const res = await apiClient.post(`/orgs/${orgId}/invitations`, data);
    return res.data;
  },

  /**
   * Resend invitation
   * @param {string} orgId
   * @param {string} invitationId
   */
  async resendInvitation(orgId, invitationId) {
    const res = await apiClient.post(`/orgs/${orgId}/invitations/${invitationId}/resend`, {});
    return res.data;
  },

  /**
   * Revoke invitation
   * @param {string} orgId
   * @param {string} invitationId
   */
  async revokeInvitation(orgId, invitationId) {
    const res = await apiClient.delete(`/orgs/${orgId}/invitations/${invitationId}`);
    return res.data;
  },

  /**
   * Update member role in organization
   * @param {string} orgId
   * @param {string} userId
   * @param {string} role
   */
  async updateMemberRole(orgId, userId, role) {
    const res = await apiClient.patch(`/orgs/${orgId}/members/${userId}`, { role });
    return res.data;
  },

  /**
   * Remove member from organization
   * @param {string} orgId
   * @param {string} userId
   */
  async removeMember(orgId, userId) {
    const res = await apiClient.delete(`/orgs/${orgId}/members/${userId}`);
    return res.data;
  },

  /**
   * Accept invitation (used by invitee)
   * @param {string} token
   */
  async acceptInvitation(token) {
    const res = await apiClient.post("/invitations/accept", { token });
    return res.data;
  },
};

export default orgsApi;
