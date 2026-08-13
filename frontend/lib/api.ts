export interface ApiProblem {
  type?: string;
  title?: string;
  status: number;
  detail?: string;
  code?: string;
  request_id?: string;
  errors?: Record<string, string>;
}

export class ApiError extends Error {
  readonly status: number;
  readonly problem?: ApiProblem;

  constructor(status: number, message: string, problem?: ApiProblem) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.problem = problem;
  }
}

const apiBase = (process.env.NEXT_PUBLIC_API_URL || "").replace(/\/$/, "");

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    headers,
    credentials: "include",
  });

  if (!response.ok) {
    const problem = await readProblem(response);
    if (response.status === 401 && typeof window !== "undefined") {
      window.dispatchEvent(new Event("fitlog:unauthorized"));
    }
    throw new ApiError(
      response.status,
      problem?.detail || problem?.title || `API request failed (${response.status})`,
      problem,
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

export function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.message) {
    return error.message;
  }
  return fallback;
}

async function readProblem(response: Response): Promise<ApiProblem | undefined> {
  const contentType = response.headers.get("Content-Type") || "";
  if (!contentType.includes("json")) {
    const text = await response.text();
    return text ? { status: response.status, detail: text } : undefined;
  }
  try {
    return await response.json() as ApiProblem;
  } catch {
    return undefined;
  }
}
