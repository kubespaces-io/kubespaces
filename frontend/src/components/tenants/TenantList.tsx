"use client";

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { CreateTenantDialog } from "@/components/tenants/CreateTenantDialog";
import { PhaseBadge } from "@/components/tenants/PhaseBadge";
import { useTenants } from "@/hooks/useTenants";
import { formatAge } from "@/lib/format";

export function TenantList() {
  const { tenants, error, create } = useTenants();
  const [isCreateOpen, setCreateOpen] = useState(false);

  return (
    <section aria-labelledby="tenants-heading">
      <header className="flex items-end justify-between border-b-2 border-ink pb-6">
        <div>
          <p className="text-[0.6875rem] font-medium uppercase tracking-[0.2em] text-ink-muted">
            {tenants === null
              ? "Loading"
              : `${tenants.length} tenant${tenants.length === 1 ? "" : "s"}`}
          </p>
          <h1
            id="tenants-heading"
            className="mt-1 font-display text-4xl font-medium tracking-tight"
          >
            Tenants
          </h1>
        </div>
        <Button onClick={() => setCreateOpen(true)}>New tenant</Button>
      </header>

      {error && (
        <p
          role="alert"
          className="mt-6 border-l-2 border-status-red bg-status-red-bg px-3 py-2 text-sm text-status-red"
        >
          {error}
        </p>
      )}

      {tenants !== null && tenants.length === 0 && !error && (
        <div className="py-24">
          <p className="font-display text-[clamp(2.5rem,6vw,4.5rem)] font-medium leading-[1.05] tracking-tight text-ink/90">
            No tenants yet.
          </p>
          <p className="mt-6 max-w-md text-[0.9375rem] leading-relaxed text-ink-muted">
            A tenant is a virtual Kubernetes cluster, provisioned in seconds.
            Create one and download its kubeconfig when it turns{" "}
            <span className="font-medium text-status-green">Ready</span>.
          </p>
          <div className="mt-10">
            <Button onClick={() => setCreateOpen(true)}>
              Create your first tenant
            </Button>
          </div>
        </div>
      )}

      {tenants !== null && tenants.length > 0 && (
        <table className="mt-2 w-full border-collapse">
          <thead>
            <tr className="text-left text-[0.6875rem] font-medium uppercase tracking-[0.12em] text-ink-faint">
              <th scope="col" className="py-3 pr-4 font-medium">
                Tenant
              </th>
              <th scope="col" className="py-3 pr-4 font-medium">
                Phase
              </th>
              <th scope="col" className="hidden py-3 pr-4 font-medium sm:table-cell">
                Owner
              </th>
              <th scope="col" className="py-3 text-right font-medium">
                Age
              </th>
            </tr>
          </thead>
          <tbody>
            {tenants.map((tenant) => (
              <tr
                key={tenant.name}
                className="group border-t border-rule transition-colors hover:bg-paper-raised"
              >
                <td className="py-5 pr-4">
                  <Link
                    href={`/tenants/${tenant.name}`}
                    className="font-display text-xl font-medium tracking-tight text-ink transition-colors group-hover:text-accent focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent"
                  >
                    {tenant.name}
                  </Link>
                  {tenant.displayName && (
                    <p className="mt-0.5 text-[0.8125rem] text-ink-muted">
                      {tenant.displayName}
                    </p>
                  )}
                </td>
                <td className="py-5 pr-4 align-middle">
                  <PhaseBadge phase={tenant.phase} />
                </td>
                <td className="hidden py-5 pr-4 align-middle text-sm text-ink-muted sm:table-cell">
                  {tenant.owner}
                </td>
                <td className="py-5 text-right align-middle text-sm tabular-nums text-ink-muted">
                  {formatAge(tenant.createdAt)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <CreateTenantDialog
        open={isCreateOpen}
        onClose={() => setCreateOpen(false)}
        onCreate={create}
      />
    </section>
  );
}
