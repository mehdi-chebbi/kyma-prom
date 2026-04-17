import { graphqlRequest } from "./graphqlRequest";
import type { Group, GroupPage, GroupFilter } from "../GQL/models/group";

/* Paginated list of groups with filtering */
export async function listGroups(
  filter?: GroupFilter,
  offset?: number,
  limit?: number
): Promise<GroupPage> {
  const query = `
    query ($filter: GroupFilterInput, $offset: Int, $limit: Int) {
      groups(filter: $filter, offset: $offset, limit: $limit) {
        items {
          cn
          gidNumber
          members
          repositories
          dn
        }
        total
        limit
        offset
        hasMore
      }
    }
  `;
  return graphqlRequest<
    { groups: GroupPage },
    { filter?: GroupFilter; offset?: number; limit?: number }
  >(query, { filter, offset, limit }).then(res => res.groups);
}

/* Get all groups without pagination */
export async function listAllGroups(): Promise<Group[]> {
  const query = `
    query {
      groupsAll {
        cn gidNumber members repositories dn
      }
    }
  `;
  return graphqlRequest<{ groupsAll: Group[] }>(query).then(res => res.groupsAll);
}

export async function getGroup(cn: string): Promise<Group> {
  const query = `
    query ($cn: String!) {
      group(cn: $cn) {
        cn gidNumber members repositories dn
      }
    }
  `;
  return graphqlRequest<{ group: Group }, { cn: string }>(query, { cn }).then(res => res.group);
}

export async function createGroup(cn: string, description?: string): Promise<Group> {
  const mutation = `
    mutation ($cn: String!, $description: String) {
      createGroup(cn: $cn, description: $description) {
        cn gidNumber members dn
      }
    }
  `;
  return graphqlRequest<{ createGroup: Group }, { cn: string; description?: string }>(mutation, { cn, description }).then(res => res.createGroup);
}

export async function deleteGroup(cn: string): Promise<boolean> {
  const mutation = `
    mutation ($cn: String!) {
      deleteGroup(cn: $cn)
    }
  `;
  return graphqlRequest<{ deleteGroup: boolean }, { cn: string }>(mutation, { cn }).then(res => res.deleteGroup);
}

export async function addUserToGroup(uid: string, groupCn: string): Promise<boolean> {
  const mutation = `
    mutation ($uid: String!, $groupCn: String!) {
      addUserToGroup(uid: $uid, groupCn: $groupCn)
    }
  `;
  return graphqlRequest<{ addUserToGroup: boolean }, { uid: string; groupCn: string }>(mutation, { uid, groupCn }).then(res => res.addUserToGroup);
}

export async function removeUserFromGroup(uid: string, groupCn: string): Promise<boolean> {
  const mutation = `
    mutation ($uid: String!, $groupCn: String!) {
      removeUserFromGroup(uid: $uid, groupCn: $groupCn)
    }
  `;
  return graphqlRequest<{ removeUserFromGroup: boolean }, { uid: string; groupCn: string }>(mutation, { uid, groupCn }).then(res => res.removeUserFromGroup);
}

export async function assignRepoToGroup(groupCn: string, repositories: string[]): Promise<Group> {
  const mutation = `
    mutation ($groupCn: String!, $repositories: [String!]!) {
      assignRepoToGroup(groupCn: $groupCn, repositories: $repositories) {
        cn gidNumber members repositories dn
      }
    }
  `;
  return graphqlRequest<{ assignRepoToGroup: Group }, { groupCn: string; repositories: string[] }>(mutation, { groupCn, repositories }).then(res => res.assignRepoToGroup);
}
