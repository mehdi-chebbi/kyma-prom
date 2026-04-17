import { graphqlRequest } from "./graphqlRequest";
import type {
  Repository,
  RepositoryPage,
  RepositoryStats,
  PaginationInput,
  CreateRepositoryInput,
  MigrateRepositoryInput,
} from "../GQL/models/repository";

export async function getRepository(
  owner: string,
  name: string,
): Promise<Repository> {
  const query = `
    query ($owner: String!, $name: String!) {
      getRepository(owner: $owner, name: $name) {
        cloneUrl
        createdAt
        defaultBranch
        description
        fork
        forks
        fullName
        htmlUrl
        id
        name
        private
        size
        sshUrl
        stars
        updatedAt
        owner {
          avatarUrl
          email
          fullName
          id
          login
        }
      }
    }
  `;

  return graphqlRequest<
    { getRepository: Repository },
    { owner: string; name: string }
  >(query, { owner, name }, true).then((res) => res.getRepository);
}

export async function listRepositories(
  pagination?: PaginationInput,
): Promise<RepositoryPage> {
  const query = `
    query ($limit: Int, $offset: Int) {
      listRepositories(limit: $limit, offset: $offset) {
        hasMore
        limit
        offset
        total
        items {
          cloneUrl
          createdAt
          defaultBranch
          description
          fork
          forks
          fullName
          htmlUrl
          id
          name
          private
          size
          sshUrl
          stars
          updatedAt
          owner {
            avatarUrl
            email
            fullName
            id
            login
          }
        }
      }
    }
  `;

  return graphqlRequest<
    { listRepositories: RepositoryPage },
    PaginationInput | undefined
  >(query, pagination, true).then((res) => res.listRepositories);
}

export async function myRepositories(
  pagination?: PaginationInput,
): Promise<RepositoryPage> {
  const query = `
    query ($limit: Int, $offset: Int) {
      myRepositories(limit: $limit, offset: $offset) {
        hasMore
        limit
        offset
        total
        items {
          cloneUrl
          createdAt
          defaultBranch
          description
          fork
          forks
          fullName
          htmlUrl
          id
          name
          private
          size
          sshUrl
          stars
          updatedAt
          owner {
            avatarUrl
            email
            fullName
            id
            login
          }
        }
      }
    }
  `;

  return graphqlRequest<
    { myRepositories: RepositoryPage },
    PaginationInput | undefined
  >(query, pagination, true).then((res) => res.myRepositories);
}

export async function searchRepositories(
  queryText: string,
  pagination?: PaginationInput,
): Promise<RepositoryPage> {
  const query = `
    query ($query: String!, $limit: Int, $offset: Int) {
      searchRepositories(query: $query, limit: $limit, offset: $offset) {
        hasMore
        limit
        offset
        total
        items {
          cloneUrl
          createdAt
          defaultBranch
          description
          fork
          forks
          fullName
          htmlUrl
          id
          name
          private
          size
          sshUrl
          stars
          updatedAt
          owner {
            avatarUrl
            email
            fullName
            id
            login
          }
        }
      }
    }
  `;

  return graphqlRequest<
    { searchRepositories: RepositoryPage },
    { query: string } & PaginationInput
  >(query, { query: queryText, ...pagination }, true).then(
    (res) => res.searchRepositories,
  );
}

export async function getRepositoryStats(): Promise<RepositoryStats> {
  const query = `
    query {
      repositoryStats {
        privateCount
        publicCount
        totalCount
        languages {
          language
          count
        }
      }
    }
  `;

  return graphqlRequest<{ repositoryStats: RepositoryStats }>(
    query,
    {},
    true,
  ).then((res) => res.repositoryStats);
}

/* ----------------------------- Mutations ----------------------------- */

export async function createRepository(
  input: CreateRepositoryInput,
): Promise<Repository> {
  const userJson = localStorage.getItem("user");
  const currentUser = userJson ? JSON.parse(userJson) : null;
  const currentUsername = currentUser?.uid;
  console.log(currentUser);

  const mutation = `
    mutation ($name: String!, $owner: String, $description: String, $private: Boolean, $autoInit: Boolean, $gitignores: String, $license: String, $defaultBranch: String) {
      createRepository(
        name: $name
        owner: $owner
        description: $description
        private: $private
        autoInit: $autoInit
        gitignores: $gitignores
        license: $license
        defaultBranch: $defaultBranch
      ) {
        cloneUrl
        createdAt
        defaultBranch
        description
        fork
        forks
        fullName
        htmlUrl
        id
        name
        private
        size
        sshUrl
        stars
        updatedAt
        owner {
          login
          id
        }
      }
    }
  `;

  const variables = {
    ...input,
    owner: currentUsername,
  };

  if (!variables.owner) {
    throw new Error(
      "Could not determine repository owner. Please log in again.",
    );
  }

  return graphqlRequest<{ createRepository: Repository }, any>(
    mutation,
    variables,
    true,
  ).then((res) => res.createRepository);
}

export async function migrateRepository(
  input: MigrateRepositoryInput,
): Promise<Repository> {
  const userJson = localStorage.getItem("user");
  const currentUser = userJson ? JSON.parse(userJson) : null;
  const currentUsername = currentUser?.uid;

  const mutation = `
    mutation (
      $cloneAddr: String!, 
      $repoName: String!, 
      $owner: String!, 
      $service: String!, 
      $private: Boolean, 
      $authToken: String, 
      $mirror: Boolean,
      $wiki: Boolean,
      $issues: Boolean,
      $pullRequests: Boolean,
      $releases: Boolean,
      $milestones: Boolean,
      $labels: Boolean,
      $description: String
    ) {
      migrateRepository(
        cloneAddr: $cloneAddr
        repoName: $repoName
        repoOwner: $owner
        service: $service
        private: $private
        authToken: $authToken
        mirror: $mirror
        wiki: $wiki
        issues: $issues
        pullRequests: $pullRequests
        releases: $releases
        milestones: $milestones
        labels: $labels
        description: $description
      ) {
        id
        name
        fullName
        private
        cloneUrl
        htmlUrl
        owner {
          login
          id
        }
      }
    }
  `;

  const variables = {
    ...input,
    owner: currentUsername,
  };

  if (!variables.owner) {
    throw new Error("User session not found. Please log in again.");
  }

  return graphqlRequest<{ migrateRepository: Repository }, any>(
    mutation,
    variables,
    true,
  ).then((res) => res.migrateRepository);
}

export async function updateRepository(
  owner: string,
  name: string,
  description: string | undefined,
  privateRepo: boolean,
  defaultBranch: string,
): Promise<Repository> {
  const mutation = `
    mutation ($owner: String!, $name: String!,$description: String, $privateRepo: Boolean,$defaultBranch: String) {
      updateRepository(owner: $owner, name: $name, description: $description, private: $privateRepo, defaultBranch: $defaultBranch) {
        cloneUrl
        createdAt
        defaultBranch
        description
        fork
        forks
        fullName
        htmlUrl
        id
        name
        private
        size
        sshUrl
        stars
        updatedAt
      }
    }
  `;

  return graphqlRequest<
    { updateRepository: Repository },
    {
      owner: string;
      name: string;
      description: string | undefined;
      privateRepo: boolean;
      defaultBranch: string;
    }
  >(
    mutation,
    { owner, name, description, privateRepo, defaultBranch },
    true,
  ).then((res) => res.updateRepository);
}

export async function deleteRepository(
  owner: string,
  name: string,
): Promise<boolean> {
  const mutation = `
    mutation ($owner: String!, $name: String!) {
      deleteRepository(owner: $owner, name: $name)
    }
  `;

  return graphqlRequest<
    { deleteRepository: boolean },
    { owner: string; name: string }
  >(mutation, { owner, name }, true).then((res) => res.deleteRepository);
}

/* ----------------------------- Branches ----------------------------- */

export interface Branch {
  name: string;
  commit?: { id: string; message: string };
}

export async function listBranches(
  owner: string,
  repo: string,
): Promise<Branch[]> {
  const query = `
    query ($owner: String!, $repo: String!) {
      listBranches(owner: $owner, repo: $repo) {
        name
        commit {
          id
          url
        }
      }
    }
  `;

  return graphqlRequest<
    { listBranches: Branch[] },
    { owner: string; repo: string }
  >(query, { owner, repo }, true).then((res) => res.listBranches);
}
