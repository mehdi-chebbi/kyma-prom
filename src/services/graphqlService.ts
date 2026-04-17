import type { HealthQuery } from "../GQL/apis/apis";
import { graphqlRequest } from "./graphqlRequest";

export const getHealth = async (): Promise<HealthQuery> => {
  const query = `
    query {
      health {
        status
        timestamp
        ldap
      }
    }
  `;
  return graphqlRequest<HealthQuery>(query);
};


