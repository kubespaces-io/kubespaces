"use client";

import { useCallback, useEffect, useState } from "react";
import * as api from "@/lib/api";
import { ApiError } from "@/lib/api";
import { isTransitional } from "@/lib/types";
import type { Tenant } from "@/lib/types";

const POLL_INTERVAL_MS = 5_000;

interface UseTenantResult {
  tenant: Tenant | null;
  error: string | null;
  /** True once a GET returned 404 (e.g. deletion finished). */
  isGone: boolean;
  refresh: () => Promise<void>;
}

export function useTenant(name: string): UseTenantResult {
  const [tenant, setTenant] = useState<Tenant | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isGone, setIsGone] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const fresh = await api.getTenant(name);
      setTenant(fresh);
      setError(null);
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 404) {
        setIsGone(true);
        return;
      }
      setError(cause instanceof Error ? cause.message : "Failed to load tenant.");
    }
  }, [name]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const shouldPoll = !isGone && (tenant ? isTransitional(tenant.phase) : false);

  useEffect(() => {
    if (!shouldPoll) return;
    const id = setInterval(() => void refresh(), POLL_INTERVAL_MS);
    return () => clearInterval(id);
  }, [shouldPoll, refresh]);

  return { tenant, error, isGone, refresh };
}
