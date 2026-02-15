const GRAPHQL_ENDPOINT = import.meta.env.VITE_GRAPHQL_ENDPOINT!;
const GITEA_ENDPOINT = import.meta.env.VITE_GITEA_API_ENDPOINT;

export async function graphqlRequest<T, V = Record<string, any>>(
  query: string,
  variables?: V,
  useGitea: boolean = false
): Promise<T> {
  const token = localStorage.getItem("authToken");

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const endpoint = useGitea ? GITEA_ENDPOINT : GRAPHQL_ENDPOINT;

  const res = await fetch(endpoint, {
    method: "POST",
    headers,
    body: JSON.stringify({ query, variables }),
  });

  const json = await res.json();

  if (json.errors) {
    throw new Error(
      json.errors.map((err: any) => err.message).join(", ")
    );
  }

  return json.data as T;
}
