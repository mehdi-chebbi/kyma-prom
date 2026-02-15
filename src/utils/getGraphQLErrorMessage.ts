export function getGraphQLErrorMessage(err: unknown): string {
  if (typeof err === "object" && err !== null) {
    const anyErr = err as any;

    if (anyErr.errors?.length) {
      return anyErr.errors.map((e: any) => e.message).join("\n");
    }

    if (anyErr.message) {
      return anyErr.message;
    }
  }

  return "An unexpected error occurred. Please try again.";
}
