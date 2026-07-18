"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import * as api from "@/lib/api";
import { isTransitional } from "@/lib/types";
import type { CreateTenantInput, Tenant } from "@/lib/types";

const POLL_INTERVAL_MS = 5_000;

function byName(a: Tenant, b: Tenant): number {
  return a.name.localeCompare(b.name);
}

interface UseTenantsResult {
  tenants: Tenant[] | null;
  error: string | null;
  isRefreshing: boolean;
  refresh: () => Promise<void>;
  /** Optimistic create — rolls back and rethrows on failure. */
  create: (input: CreateTenantInput) => Promise<Tenant>;
}

export function useTenants(): UseTenantsResult {
  const [tenants, setTenants] = useState<Tenant[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  // Names with an in-flight optimistic create; polling must not clobber them.
  const pendingCreates = useRef<Set<string>>(new Set());

  const refresh = useCallback(async () => {
    setIsRefreshing(true);
    try {
      const fresh = await api.listTenants();
      setTenants((previous) => {
        const optimistic = (previous ?? []).filter(
          (t) =>
            pendingCreates.current.has(t.name) &&
            !fresh.some((f) => f.name === t.name),
        );
        return [...fresh, ...optimistic].sort(byName);
      });
      setError(null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Failed to load tenants.");
    } finally {
      setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const shouldPoll = tenants?.some((t) => isTransitional(t.phase)) ?? false;

  useEffect(() => {
    if (!shouldPoll) return;
    const id = setInterval(() => void refresh(), POLL_INTERVAL_MS);
    return () => clearInterval(id);
  }, [shouldPoll, refresh]);

  const create = useCallback(
    async (input: CreateTenantInput): Promise<Tenant> => {
      const optimistic: Tenant = {
        name: input.name,
        displayName: input.displayName ?? "",
        owner: "you",
        phase: "Pending",
        message: "Requesting tenant…",
        resources: input.resources ?? {},
        createdAt: new Date().toISOString(),
      };
      pendingCreates.current.add(input.name);
      setTenants((previous) => [...(previous ?? []), optimistic].sort(byName));

      try {
        const created = await api.createTenant(input);
        setTenants((previous) =>
          (previous ?? [])
            .map((t) => (t.name === created.name ? created : t))
            .sort(byName),
        );
        return created;
      } catch (cause) {
        // Roll back the optimistic row; the dialog surfaces the error.
        setTenants((previous) =>
          (previous ?? []).filter((t) => t.name !== input.name),
        );
        throw cause;
      } finally {
        pendingCreates.current.delete(input.name);
      }
    },
    [],
  );

  return { tenants, error, isRefreshing, refresh, create };
}
