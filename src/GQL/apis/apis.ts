import type { User } from "../models/user";
import type { AuthPayload } from "../models/authPayload";

// ------------------- Queries -------------------

export type MeQuery = {
  me: User | null;
};

export type UserQuery = {
  user: User | null;
};

export type UserQueryVariables = {
  uid: string;
};

export type HealthQuery = {
  health: {
    status: string;
    timestamp: number;
    ldap: boolean;
  };
};

export type StatsQuery = {
  stats: {
    poolSize: number;
    available: number;
    inUse: number;
    totalRequests: number;
  };
};

// ------------------- Mutations -------------------

export type LoginMutation = {
  login: AuthPayload;
};

export type LoginMutationVariables = {
  uid: string;
  password: string;
};

export type RegisterMutation = {
  register: AuthPayload;
};

export type RegisterMutationVariables = {
  username: string;
  password: string;
  email: string;
  firstName: string;
  lastName: string;
  userType: string;
};
