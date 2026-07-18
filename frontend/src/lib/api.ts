import type { CreateTenantInput, Tenant } from "@/lib/types";

/** Error raised for non-2xx responses from the KubeSpaces API. */
export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    ...init,
    headers: { Accept: "application/json", ...init?.headers },
  });

  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const body: { error?: string } = await response.json();
      if (body.error) message = body.error;
    } catch {
      // non-JSON error body — keep the generic message
    }
    if (response.status === 401) {
      message = "Your session has expired — please sign in again.";
    }
    throw new ApiError(response.status, message);
  }

  if (response.status === 202 || response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

export function listTenants(): Promise<Tenant[]> {
  return request<Tenant[]>("/tenants");
}

export function getTenant(name: string): Promise<Tenant> {
  return request<Tenant>(`/tenants/${encodeURIComponent(name)}`);
}

export function createTenant(input: CreateTenantInput): Promise<Tenant> {
  return request<Tenant>("/tenants", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export function deleteTenant(name: string): Promise<void> {
  return request<void>(`/tenants/${encodeURIComponent(name)}`, {
    method: "DELETE",
  });
}

/** Fetches the kubeconfig YAML and triggers a browser download. */
export async function downloadKubeconfig(name: string): Promise<void> {
  const response = await fetch(
    `/api/v1/tenants/${encodeURIComponent(name)}/kubeconfig`,
  );
  if (!response.ok) {
    let message = `Kubeconfig download failed (${response.status})`;
    try {
      const body: { error?: string } = await response.json();
      if (body.error) message = body.error;
    } catch {
      // ignore non-JSON body
    }
    throw new ApiError(response.status, message);
  }

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `${name}-kubeconfig.yaml`;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}
