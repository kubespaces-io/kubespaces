"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Field } from "@/components/ui/Field";
import { validateTenantName, TENANT_NAME_MAX_LENGTH } from "@/lib/validation";
import type { CreateTenantInput, Tenant } from "@/lib/types";

interface CreateTenantDialogProps {
  open: boolean;
  onClose: () => void;
  onCreate: (input: CreateTenantInput) => Promise<Tenant>;
}

const INITIAL = {
  name: "",
  displayName: "",
  cpu: "",
  memory: "",
  storage: "",
};

export function CreateTenantDialog({
  open,
  onClose,
  onCreate,
}: CreateTenantDialogProps) {
  const router = useRouter();
  const [values, setValues] = useState(INITIAL);
  const [touched, setTouched] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const nameError = touched ? validateTenantName(values.name) : null;

  const set = (key: keyof typeof INITIAL) => (value: string) =>
    setValues((previous) => ({ ...previous, [key]: value }));

  const close = () => {
    setValues(INITIAL);
    setTouched(false);
    setSubmitError(null);
    onClose();
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setTouched(true);
    if (validateTenantName(values.name)) return;

    const resources = {
      ...(values.cpu.trim() && { cpu: values.cpu.trim() }),
      ...(values.memory.trim() && { memory: values.memory.trim() }),
      ...(values.storage.trim() && { storage: values.storage.trim() }),
    };
    const input: CreateTenantInput = {
      name: values.name,
      ...(values.displayName.trim() && { displayName: values.displayName.trim() }),
      ...(Object.keys(resources).length > 0 && { resources }),
    };

    setSubmitting(true);
    setSubmitError(null);
    try {
      const created = await onCreate(input);
      close();
      router.push(`/tenants/${created.name}`);
    } catch (cause) {
      setSubmitError(
        cause instanceof Error ? cause.message : "Failed to create tenant.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onClose={close} labelledBy="create-tenant-title">
      <form onSubmit={handleSubmit} noValidate>
        <p className="text-[0.6875rem] font-medium uppercase tracking-[0.2em] text-accent">
          New
        </p>
        <h2
          id="create-tenant-title"
          className="mt-1 font-display text-2xl font-medium tracking-tight"
        >
          Create a tenant
        </h2>

        <div className="mt-8 flex flex-col gap-6">
          <Field
            id="tenant-name"
            label="Name"
            value={values.name}
            onChange={(e) => {
              setTouched(true);
              set("name")(e.target.value);
            }}
            error={nameError}
            hint={`DNS-1123 label, max ${TENANT_NAME_MAX_LENGTH} characters. Becomes namespace kubespaces-tenant-<name>.`}
            placeholder="acme-staging"
            autoComplete="off"
            spellCheck={false}
            maxLength={TENANT_NAME_MAX_LENGTH + 10}
            required
          />
          <Field
            id="tenant-display-name"
            label="Display name"
            value={values.displayName}
            onChange={(e) => set("displayName")(e.target.value)}
            placeholder="Acme Staging"
            autoComplete="off"
          />

          <fieldset>
            <legend className="text-[0.6875rem] font-medium uppercase tracking-[0.12em] text-ink-muted">
              Quotas <span className="normal-case tracking-normal text-ink-faint">— optional</span>
            </legend>
            <div className="mt-3 grid grid-cols-3 gap-4">
              <Field
                id="tenant-cpu"
                label="CPU"
                value={values.cpu}
                onChange={(e) => set("cpu")(e.target.value)}
                placeholder="4"
                autoComplete="off"
              />
              <Field
                id="tenant-memory"
                label="Memory"
                value={values.memory}
                onChange={(e) => set("memory")(e.target.value)}
                placeholder="8Gi"
                autoComplete="off"
              />
              <Field
                id="tenant-storage"
                label="Storage"
                value={values.storage}
                onChange={(e) => set("storage")(e.target.value)}
                placeholder="20Gi"
                autoComplete="off"
              />
            </div>
          </fieldset>
        </div>

        {submitError && (
          <p
            role="alert"
            className="mt-6 border-l-2 border-status-red bg-status-red-bg px-3 py-2 text-sm text-status-red"
          >
            {submitError}
          </p>
        )}

        <div className="mt-8 flex items-center justify-end gap-3">
          <Button variant="ghost" onClick={close} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? "Creating…" : "Create tenant"}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
