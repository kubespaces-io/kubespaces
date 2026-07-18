export type TenantPhase =
  | "Pending"
  | "Provisioning"
  | "Ready"
  | "Deleting"
  | "Failed"
  | "Unknown";

export const TRANSITIONAL_PHASES: readonly TenantPhase[] = [
  "Pending",
  "Provisioning",
  "Deleting",
];

export function isTransitional(phase: TenantPhase): boolean {
  return TRANSITIONAL_PHASES.includes(phase);
}

export interface TenantResources {
  cpu?: string;
  memory?: string;
  storage?: string;
}

export interface Tenant {
  name: string;
  displayName: string;
  owner: string;
  phase: TenantPhase;
  message: string;
  resources: TenantResources;
  createdAt: string;
}

export interface CreateTenantInput {
  name: string;
  displayName?: string;
  resources?: TenantResources;
}
