export interface Group {
  cn: string;
  gidNumber: number;
  members: string[];
  repositories?: string[];
  description?: string;
  dn: string;
}

export interface GroupPage {
  items: Group[];
  total: number;
  offset: number;
  limit: number;
  hasMore: boolean;
}

export interface GroupFilter {
  cn?: string;
}
