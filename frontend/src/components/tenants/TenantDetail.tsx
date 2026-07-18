"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { DeleteTenantDialog } from "@/components/tenants/DeleteTenantDialog";
import { PhaseBadge } from "@/components/tenants/PhaseBadge";
import { useTenant } from "@/hooks/useTenant";
import * as api from "@/lib/api";
import { formatTimestamp } from "@/lib/format";
import { isTransitional } from "@/lib/types";

function SpecRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-6 border-t border-rule py-3.5">
      <dt className="text-[0.6875rem] font-medium uppercase tracking-[0.12em] text-ink-faint">
        {label}
      </dt>
      <dd className="text-right text-sm text-ink">{value}</dd>
    </div>
  );
}

export function TenantDetail({ name }: { name: string }) {
  const router = useRouter();
  const { tenant, error, isGone, refresh } = useTenant(name);
  const [isDeleteOpen, setDeleteOpen] = useState(false);
  const [downloadError, setDownloadError] = useState<string | null>(null);

  if (isGone) {
    return (
      <div className="py-24">
        <p className="font-display text-3xl font-medium tracking-tight">
          {name} is gone.
        </p>
        <p className="mt-4 text-sm text-ink-muted">
          This tenant no longer exists — it may have just finished deleting.
        </p>
        <Link
          href="/tenants"
          className="mt-8 inline-block font-display text-sm font-medium text-accent underline-offset-4 hover:underline"
        >
          ← Back to tenants
        </Link>
      </div>
    );
  }

  const handleDownload = async () => {
    setDownloadError(null);
    try {
      await api.downloadKubeconfig(name);
    } catch (cause) {
      setDownloadError(
        cause instanceof Error ? cause.message : "Download failed.",
      );
    }
  };

  const handleDelete = async () => {
    await api.deleteTenant(name);
    setDeleteOpen(false);
    router.push("/tenants");
    router.refresh();
  };

  const isReady = tenant?.phase === "Ready";

  return (
    <article aria-labelledby="tenant-heading">
      <nav aria-label="Breadcrumb" className="text-sm">
        <Link
          href="/tenants"
          className="text-ink-muted underline-offset-4 transition-colors hover:text-accent hover:underline"
        >
          ← Tenants
        </Link>
      </nav>

      <header className="mt-6 border-b-2 border-ink pb-6">
        <div className="flex flex-wrap items-center gap-4">
          <h1
            id="tenant-heading"
            className="font-display text-4xl font-medium tracking-tight"
          >
            {name}
          </h1>
          {tenant && <PhaseBadge phase={tenant.phase} />}
        </div>
        {tenant?.displayName && (
          <p className="mt-2 text-[0.9375rem] text-ink-muted">
            {tenant.displayName}
          </p>
        )}
        {tenant?.message && (
          <p
            className="mt-3 text-sm leading-relaxed text-ink-muted"
            aria-live="polite"
          >
            {tenant.message}
          </p>
        )}
        {tenant && isTransitional(tenant.phase) && (
          <p className="mt-2 text-xs text-ink-faint">
            Status refreshes every 5 seconds.
          </p>
        )}
      </header>

      {error && (
        <p
          role="alert"
          className="mt-6 border-l-2 border-status-red bg-status-red-bg px-3 py-2 text-sm text-status-red"
        >
          {error}{" "}
          <button
            type="button"
            onClick={() => void refresh()}
            className="font-medium underline underline-offset-2"
          >
            Retry
          </button>
        </p>
      )}

      {tenant && (
        <div className="mt-10 grid gap-12 md:grid-cols-[1fr_20rem]">
          <section aria-labelledby="spec-heading">
            <h2
              id="spec-heading"
              className="text-[0.6875rem] font-medium uppercase tracking-[0.2em] text-ink-muted"
            >
              Specification
            </h2>
            <dl className="mt-4">
              <SpecRow label="Owner" value={tenant.owner} />
              <SpecRow label="Created" value={formatTimestamp(tenant.createdAt)} />
              <SpecRow label="CPU quota" value={tenant.resources.cpu ?? "—"} />
              <SpecRow
                label="Memory quota"
                value={tenant.resources.memory ?? "—"}
              />
              <SpecRow
                label="Storage quota"
                value={tenant.resources.storage ?? "—"}
              />
              <SpecRow
                label="Namespace"
                value={`kubespaces-tenant-${tenant.name}`}
              />
            </dl>
          </section>

          <aside className="flex flex-col gap-12">
            <section aria-labelledby="access-heading">
              <h2
                id="access-heading"
                className="text-[0.6875rem] font-medium uppercase tracking-[0.2em] text-ink-muted"
              >
                Access
              </h2>
              <p className="mt-4 text-sm leading-relaxed text-ink-muted">
                {isReady
                  ? "The cluster is ready. Download its kubeconfig to connect with kubectl."
                  : "The kubeconfig becomes available once the tenant is Ready."}
              </p>
              <div className="mt-4">
                <Button
                  variant="secondary"
                  onClick={() => void handleDownload()}
                  disabled={!isReady}
                  aria-disabled={!isReady}
                >
                  Download kubeconfig
                </Button>
              </div>
              {downloadError && (
                <p role="alert" className="mt-3 text-xs text-status-red">
                  {downloadError}
                </p>
              )}
            </section>

            <section
              aria-labelledby="danger-heading"
              className="border-t border-status-red/30 pt-6"
            >
              <h2
                id="danger-heading"
                className="text-[0.6875rem] font-medium uppercase tracking-[0.2em] text-status-red"
              >
                Danger zone
              </h2>
              <p className="mt-4 text-sm leading-relaxed text-ink-muted">
                Deleting a tenant tears down its virtual cluster and all
                workloads inside it.
              </p>
              <div className="mt-4">
                <Button
                  variant="danger"
                  onClick={() => setDeleteOpen(true)}
                  disabled={tenant.phase === "Deleting"}
                >
                  Delete tenant
                </Button>
              </div>
            </section>
          </aside>
        </div>
      )}

      <DeleteTenantDialog
        tenantName={name}
        open={isDeleteOpen}
        onClose={() => setDeleteOpen(false)}
        onConfirm={handleDelete}
      />
    </article>
  );
}
