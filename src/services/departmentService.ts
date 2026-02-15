import { graphqlRequest } from "./graphqlRequest";
import type { Department, CreateDepartmentInput, DepartmentPage, DepartmentFilter } from "../GQL/models/department";

/* Get single department */
export async function getDepartment(ou: string): Promise<Department> {
  const query = `
    query ($ou: String!) {
      department(ou: $ou) {
        ou description manager members repositories dn
      }
    }
  `;
  return graphqlRequest<{ department: Department }, { ou: string }>(query, { ou }).then(res => res.department);
}

/* Paginated list of departments with filtering */
export async function listDepartments(
  filter?: DepartmentFilter,
  offset?: number,
  limit?: number
): Promise<DepartmentPage> {
  // Using departmentsAll as workaround for paginated query bug
  const query = `
    query {
      departmentsAll {
        description
        dn
        ou
        repositories
      }
    }
  `;

  return graphqlRequest<{ departmentsAll: Department[] }>(query).then(res => {
    let departments = res.departmentsAll || [];

    // Apply client-side filtering
    if (filter) {
      if (filter.ou) departments = departments.filter(d => d.ou?.toLowerCase().includes(filter.ou!.toLowerCase()));
      if (filter.description) departments = departments.filter(d => d.description?.toLowerCase().includes(filter.description!.toLowerCase()));
    }

    const total = departments.length;
    const start = offset || 0;
    const end = start + (limit || 8);
    const paged = departments.slice(start, end);

    return {
      items: paged,
      total,
      limit: limit || 8,
      offset: start,
      hasMore: end < total
    };
  });
}
export async function listAllDepartments(): Promise<Department[]> {
  const query = `
    query {
        departmentsAll {
          ou
          description
        }
    }
  `;

  return graphqlRequest<
    { departmentsAll: Department[] }
  >(query).then(res => res.departmentsAll);
}


/* Create a department */
export async function createDepartment(input: CreateDepartmentInput): Promise<Department> {
  const mutation = `
    mutation ($input: CreateDepartmentInput!) {
      createDepartment(input: $input) {
        description
        dn
        ou
        repositories      }
    }
  `;
  return graphqlRequest<{ createDepartment: Department }, { input: CreateDepartmentInput }>(mutation, { input }).then(res => res.createDepartment);
}

/* Delete a department */
export async function deleteDepartment(ou: string): Promise<boolean> {
  const mutation = `
    mutation ($ou: String!) {
      deleteDepartment(ou: $ou)
    }
  `;
  return graphqlRequest<{ deleteDepartment: boolean }, { ou: string }>(mutation, { ou }).then(res => res.deleteDepartment);
}
