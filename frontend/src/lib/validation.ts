export const TENANT_NAME_MAX_LENGTH = 40;

const DNS1123_LABEL = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

/**
 * Validates a tenant name as a DNS-1123 label, max 40 chars
 * (leaves room for the `kubespaces-tenant-` namespace prefix).
 * Returns an error message, or null when valid.
 */
export function validateTenantName(name: string): string | null {
  if (name.length === 0) {
    return "Name is required.";
  }
  if (name.length > TENANT_NAME_MAX_LENGTH) {
    return `Max ${TENANT_NAME_MAX_LENGTH} characters (currently ${name.length}).`;
  }
  if (!DNS1123_LABEL.test(name)) {
    return "Lowercase letters, digits and hyphens only; must start and end with a letter or digit.";
  }
  return null;
}
