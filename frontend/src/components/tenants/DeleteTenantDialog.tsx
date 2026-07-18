"use client";

import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Field } from "@/components/ui/Field";

interface DeleteTenantDialogProps {
  tenantName: string;
  open: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
}

export function DeleteTenantDialog({
  tenantName,
  open,
  onClose,
  onConfirm,
}: DeleteTenantDialogProps) {
  const [typed, setTyped] = useState("");
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const matches = typed === tenantName;

  const close = () => {
    setTyped("");
    setError(null);
    onClose();
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!matches || deleting) return;
    setDeleting(true);
    setError(null);
    try {
      await onConfirm();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Failed to delete tenant.",
      );
      setDeleting(false);
    }
  };

  return (
    <Dialog open={open} onClose={close} labelledBy="delete-tenant-title">
      <form onSubmit={handleSubmit}>
        <p className="text-[0.6875rem] font-medium uppercase tracking-[0.2em] text-status-red">
          Irreversible
        </p>
        <h2
          id="delete-tenant-title"
          className="mt-1 font-display text-2xl font-medium tracking-tight"
        >
          Delete {tenantName}
        </h2>
        <p className="mt-4 text-sm leading-relaxed text-ink-muted">
          The virtual cluster and everything running in it will be torn down.
          Type the tenant name to confirm.
        </p>

        <div className="mt-6">
          <Field
            id="delete-confirm"
            label={`Type “${tenantName}” to confirm`}
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            autoComplete="off"
            spellCheck={false}
          />
        </div>

        {error && (
          <p
            role="alert"
            className="mt-4 border-l-2 border-status-red bg-status-red-bg px-3 py-2 text-sm text-status-red"
          >
            {error}
          </p>
        )}

        <div className="mt-8 flex items-center justify-end gap-3">
          <Button variant="ghost" onClick={close} disabled={deleting}>
            Cancel
          </Button>
          <Button type="submit" variant="danger" disabled={!matches || deleting}>
            {deleting ? "Deleting…" : "Delete tenant"}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
