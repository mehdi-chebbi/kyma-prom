import { graphqlRequest } from "./graphqlRequest";
import type {
  User,
  CreateUserInput,
  UpdateUserInput,
  UserPage,
  UserFilter,
} from "../GQL/models/user";

// Keycloak configuration
const KEYCLOAK_URL =
  import.meta.env.VITE_KEYCLOAK_URL || "http://localhost:30080";
const KEYCLOAK_REALM = import.meta.env.VITE_KEYCLOAK_REALM || "devplatform";
const KEYCLOAK_CLIENT_ID =
  import.meta.env.VITE_KEYCLOAK_CLIENT_ID || "frontend-admin";
const KEYCLOAK_CLIENT_SECRET =
  import.meta.env.VITE_KEYCLOAK_CLIENT_SECRET || "";

let logoutTimer: ReturnType<typeof setTimeout> | null = null;

export interface KeycloakTokenResponse {
  access_token: string;
  expires_in: number;
  refresh_expires_in: number;
  refresh_token: string;
  token_type: string;
  session_state: string;
  scope: string;
}

export interface LoginResult {
  token: string;
  user: {
    uid: string;
    mail: string;
    cn: string;
    givenName: string;
    sn: string;
  };
}

export const setupAutoLogout = (
  expirationTime: number,
  navigate: (path: string) => void,
) => {
  if (logoutTimer) clearTimeout(logoutTimer);

  const delay = expirationTime - Date.now();
  if (delay > 0) {
    logoutTimer = setTimeout(() => {
      alert("Session expired. Please log in again.");
      logout(navigate);
    }, delay);
  } else {
    logout(navigate);
  }
};

export const checkExistingSession = (navigate: (path: string) => void) => {
  const expiresAt = localStorage.getItem("expiresAt");
  if (expiresAt) {
    setupAutoLogout(parseInt(expiresAt, 10), navigate);
  }
};

export const login = async (
  variables: { uid: string; password: string },
  navigate: (path: string) => void,
): Promise<{ login: LoginResult }> => {
  const tokenUrl = `${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}/protocol/openid-connect/token`;

  const params = new URLSearchParams();
  params.append("grant_type", "password");
  params.append("client_id", KEYCLOAK_CLIENT_ID);
  params.append("client_secret", KEYCLOAK_CLIENT_SECRET);
  params.append("username", variables.uid);
  params.append("password", variables.password);

  const response = await fetch(tokenUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: params.toString(),
  });

  if (!response.ok) {
    const error = await response
      .json()
      .catch(() => ({ error_description: "Login failed" }));
    throw new Error(error.error_description || "Authentication failed");
  }

  const tokenData: KeycloakTokenResponse = await response.json();
  const expiresAt = Date.now() + tokenData.expires_in * 1000;

  localStorage.setItem("token", tokenData.access_token);
  localStorage.setItem("expiresAt", expiresAt.toString());

  // Decode JWT to get user info
  const payload = JSON.parse(atob(tokenData.access_token.split(".")[1]));

  const loginResult = {
    login: {
      token: tokenData.access_token,
      user: {
        uid: payload.preferred_username || variables.uid,
        mail: payload.email || "",
        cn: payload.name || "",
        givenName: payload.given_name || "",
        sn: payload.family_name || "",
      },
    },
  };
  localStorage.setItem("user", JSON.stringify(loginResult.login.user));
  setupAutoLogout(expiresAt, navigate);

  return loginResult;
};

export const logout = (navigate: (path: string) => void): void => {
  if (logoutTimer) clearTimeout(logoutTimer);
  localStorage.removeItem("token");
  localStorage.removeItem("user");
  localStorage.removeItem("expiresAt");
  navigate("/login");
};

export const register = async (
  variables: RegisterMutationVariables,
): Promise<RegisterMutation> => {
  const mutation = `
    mutation ($username: String!, $password: String!, $email: String!, $firstName: String!, $lastName: String!, $userType: String!) {
      register(
        username: $username
        password: $password
        email: $email
        firstName: $firstName
        lastName: $lastName
        userType: $userType
      ) {
        token
        user {
          id
          username
          email
          firstName
          lastName
          userType
        }
      }
    }
  `;
  return graphqlRequest<RegisterMutation, RegisterMutationVariables>(
    mutation,
    variables,
  );
};

export async function getUser(uid: string): Promise<User> {
  const query = `
    query ($uid: String!) {
      user(uid: $uid) {
        cn
        department
        dn
        givenName
        mail
        repositories
        sn
        uid
      }
    }
  `;
  return graphqlRequest<{ user: User }, { uid: string }>(query, { uid }).then(
    (res) => res.user,
  );
}

export const getMe = async (): Promise<MeQuery> => {
  const query = `
    query {
      me {
        cn
        department
        dn
        givenName
        mail
        repositories
        sn
        uid
      }
    }
  `;
  return graphqlRequest<MeQuery>(query);
};

export async function listUsers(
  filter?: UserFilter,
  offset?: number,
  limit?: number,
): Promise<UserPage> {
  // Using usersAll as workaround for paginated query bug
  const query = `
    query {
      usersAll {
        cn
        department
        dn
        givenName
        mail
        repositories
        sn
        uid
      }
    }
  `;

  return graphqlRequest<{ usersAll: User[] }>(query).then((res) => {
    let users = res.usersAll || [];

    if (filter) {
      if (filter.cn)
        users = users.filter((u) =>
          u.cn?.toLowerCase().includes(filter.cn!.toLowerCase()),
        );
      if (filter.mail)
        users = users.filter((u) =>
          u.mail?.toLowerCase().includes(filter.mail!.toLowerCase()),
        );
      if (filter.department)
        users = users.filter((u) =>
          u.department
            ?.toLowerCase()
            .includes(filter.department!.toLowerCase()),
        );
    }

    const total = users.length;
    const start = offset || 0;
    const end = start + (limit || 8);
    const paged = users.slice(start, end);

    return {
      items: paged,
      total,
      limit: limit || 8,
      offset: start,
      hasMore: end < total,
    };
  });
}

export async function createUser(input: CreateUserInput): Promise<User> {
  const mutation = `
    mutation ($input: CreateUserInput!) {
      createUser(input: $input) {
        cn
        department
        dn
        givenName
        mail
        repositories
        sn
        uid
      }
    }
  `;
  return graphqlRequest<{ createUser: User }, { input: CreateUserInput }>(
    mutation,
    { input },
  ).then((res) => res.createUser);
}

export async function updateUser(input: UpdateUserInput): Promise<User> {
  const mutation = `
    mutation ($input: UpdateUserInput!) {
      updateUser(input: $input) {
        cn
        department
        dn
        givenName
        mail
        repositories
        sn
        uid      
      }
    }
  `;
  return graphqlRequest<{ updateUser: User }, { input: UpdateUserInput }>(
    mutation,
    { input },
  ).then((res) => res.updateUser);
}

export async function assignRepoToUser(uid: string, repositories: string[]): Promise<User> {
  const mutation = `
    mutation ($uid: String!, $repositories: [String!]!) {
      assignRepoToUser(uid: $uid, repositories: $repositories) {
        cn
        department
        dn
        givenName
        mail
        repositories
        sn
        uid
      }
    }
  `;
  return graphqlRequest<{ assignRepoToUser: User }, { uid: string; repositories: string[] }>(
    mutation,
    { uid, repositories },
  ).then((res) => res.assignRepoToUser);
}

export async function deleteUser(uid: string): Promise<boolean> {
  const mutation = `
    mutation ($uid: String!) {
      deleteUser(uid: $uid)
    }
  `;
  return graphqlRequest<{ deleteUser: boolean }, { uid: string }>(mutation, {
    uid,
  }).then((res) => res.deleteUser);
}
