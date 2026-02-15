const CODESERVER_ENDPOINT = import.meta.env.VITE_CODESERVER_API_ENDPOINT;

async function codeserverRequest<T>(query: string, variables?: Record<string, any>): Promise<T> {
  const token = localStorage.getItem("authToken");

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(CODESERVER_ENDPOINT, {
    method: "POST",
    headers,
    body: JSON.stringify({ query, variables }),
  });

  const json = await res.json();

  if (json.errors) {
    throw new Error(json.errors.map((err: any) => err.message).join(", "));
  }

  return json.data as T;
}

export interface CodeServerInstance {
  id: string;
  userId: string;
  repoName: string;
  repoOwner: string;
  url: string;
  status: string;
  createdAt: string;
  lastAccessedAt?: string;
  storageUsed?: string;
  errorMessage?: string;
}

export interface ProvisionResult {
  instance: CodeServerInstance;
  message: string;
  isNew: boolean;
}

export interface InstanceStats {
  totalInstances: number;
  runningInstances: number;
  stoppedInstances: number;
  pendingInstances: number;
  totalStorageUsed: string;
}

const INSTANCE_FIELDS = `
  id
  userId
  repoName
  repoOwner
  url
  status
  createdAt
  lastAccessedAt
  storageUsed
  errorMessage
`;

export async function provisionCodeServer(repoOwner: string, repoName: string, branch?: string): Promise<ProvisionResult> {
  const mutation = `
    mutation ($owner: String!, $name: String!, $branch: String) {
      provisionCodeServer(repoOwner: $owner, repoName: $name, branch: $branch) {
        instance { ${INSTANCE_FIELDS} }
        message
        isNew
      }
    }
  `;

  return codeserverRequest<{ provisionCodeServer: ProvisionResult }>(mutation, {
    owner: repoOwner,
    name: repoName,
    branch: branch || null,
  }).then(res => res.provisionCodeServer);
}

// ─── Queries ────────────────────────────────────────────────

export async function listMyCodeServers(): Promise<CodeServerInstance[]> {
  const query = `query { myCodeServers { ${INSTANCE_FIELDS} } }`;
  return codeserverRequest<{ myCodeServers: CodeServerInstance[] }>(query)
    .then(res => res.myCodeServers);
}

export async function getCodeServer(id: string): Promise<CodeServerInstance | null> {
  const query = `query ($id: String!) { codeServer(id: $id) { ${INSTANCE_FIELDS} } }`;
  return codeserverRequest<{ codeServer: CodeServerInstance | null }>(query, { id })
    .then(res => res.codeServer);
}

export async function getCodeServerStatus(id: string): Promise<string> {
  const query = `query ($id: String!) { codeServerStatus(id: $id) }`;
  return codeserverRequest<{ codeServerStatus: string }>(query, { id })
    .then(res => res.codeServerStatus);
}

export async function getCodeServerLogs(id: string, lines?: number): Promise<string> {
  const query = `query ($id: String!, $lines: Int) { codeServerLogs(id: $id, lines: $lines) }`;
  return codeserverRequest<{ codeServerLogs: string }>(query, { id, lines: lines ?? null })
    .then(res => res.codeServerLogs);
}

export async function getInstanceStats(): Promise<InstanceStats> {
  const query = `query {
    instanceStats {
      totalInstances
      runningInstances
      stoppedInstances
      pendingInstances
      totalStorageUsed
    }
  }`;
  return codeserverRequest<{ instanceStats: InstanceStats }>(query)
    .then(res => res.instanceStats);
}

// ─── Mutations ──────────────────────────────────────────────

export async function stopCodeServer(id: string): Promise<boolean> {
  const mutation = `mutation ($id: String!) { stopCodeServer(id: $id) }`;
  return codeserverRequest<{ stopCodeServer: boolean }>(mutation, { id })
    .then(res => res.stopCodeServer);
}

export async function startCodeServer(id: string): Promise<CodeServerInstance> {
  const mutation = `mutation ($id: String!) { startCodeServer(id: $id) { ${INSTANCE_FIELDS} } }`;
  return codeserverRequest<{ startCodeServer: CodeServerInstance }>(mutation, { id })
    .then(res => res.startCodeServer);
}

export async function deleteCodeServer(id: string): Promise<boolean> {
  const mutation = `mutation ($id: String!) { deleteCodeServer(id: $id) }`;
  return codeserverRequest<{ deleteCodeServer: boolean }>(mutation, { id })
    .then(res => res.deleteCodeServer);
}

export async function syncRepository(id: string): Promise<boolean> {
  const mutation = `mutation ($id: String!) { syncRepository(id: $id) }`;
  return codeserverRequest<{ syncRepository: boolean }>(mutation, { id })
    .then(res => res.syncRepository);
}
