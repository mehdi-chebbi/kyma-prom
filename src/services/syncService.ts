import { graphqlRequest } from "./graphqlRequest";

export interface GiteaUser {
  id: number;
  login: string;
  fullName: string;
  email: string;
  isAdmin: boolean;
  created: string;
}

export interface RepoSyncResult {
  uid: string;
  reposCount: number;
  repositories: string[];
}

/** Sync all LDAP users → Gitea (creates Gitea accounts for LDAP users) */
export async function syncAllLDAPUsersToGitea(
  defaultPassword: string = "changeme123"
): Promise<GiteaUser[]> {
  const mutation = `
    mutation ($defaultPassword: String) {
      syncAllLDAPUsers(defaultPassword: $defaultPassword) {
        id
        login
        fullName
        email
        isAdmin
        created
      }
    }
  `;
  return graphqlRequest<{ syncAllLDAPUsers: GiteaUser[] }>(
    mutation,
    { defaultPassword },
    true
  ).then((res) => res.syncAllLDAPUsers || []);
}

/** Sync a single LDAP user → Gitea */
export async function syncLDAPUserToGitea(
  uid: string,
  defaultPassword: string = "changeme123"
): Promise<GiteaUser> {
  const mutation = `
    mutation ($uid: String!, $defaultPassword: String) {
      syncLDAPUser(uid: $uid, defaultPassword: $defaultPassword) {
        id
        login
        fullName
        email
        isAdmin
        created
      }
    }
  `;
  return graphqlRequest<{ syncLDAPUser: GiteaUser }>(
    mutation,
    { uid, defaultPassword },
    true
  ).then((res) => res.syncLDAPUser);
}

/** Sync all users' Gitea repos → LDAP (updates LDAP githubRepository attributes) */
export async function syncAllGiteaReposToLDAP(): Promise<RepoSyncResult[]> {
  const mutation = `
    mutation {
      syncAllGiteaReposToLDAP {
        uid
        reposCount
        repositories
      }
    }
  `;
  return graphqlRequest<{ syncAllGiteaReposToLDAP: RepoSyncResult[] }>(
    mutation,
    {},
    true
  ).then((res) => res.syncAllGiteaReposToLDAP || []);
}

/** Sync a single user's Gitea repos → LDAP */
export async function syncGiteaReposToLDAP(
  uid: string
): Promise<RepoSyncResult> {
  const mutation = `
    mutation ($uid: String!) {
      syncGiteaReposToLDAP(uid: $uid) {
        uid
        reposCount
        repositories
      }
    }
  `;
  return graphqlRequest<{ syncGiteaReposToLDAP: RepoSyncResult }>(
    mutation,
    { uid },
    true
  ).then((res) => res.syncGiteaReposToLDAP);
}
