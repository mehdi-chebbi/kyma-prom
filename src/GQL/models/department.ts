export interface Department {
  ou: string;
  description?: string;
  manager?: string;
  members: string[];
  repositories: string[];
  dn: string;
}

export interface CreateDepartmentInput {
  ou: string;
  description?: string;
  manager?: string;
  repositories?: string[];
}

export interface UpdateDepartmentInput {
  ou: string;
  description?: string;
  manager?: string;
  repositories?: string[];
}

export interface DepartmentPage {
  items: Department[];
  hasMore: boolean;
  limit: number;
  offset: number;
  total: number;
}

export interface DepartmentFilter {
  ou?: string;
  description?: string;
  manager?: string;
  repository?: string;
  query?: string;
}