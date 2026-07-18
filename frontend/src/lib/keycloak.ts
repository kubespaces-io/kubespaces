/**
 * Direct Keycloak token-endpoint helpers for the public client.
 * Public client => no client secret; refresh uses client_id only.
 * Edge-safe (fetch only), shared by the Auth.js jwt callback and the
 * API proxy route.
 */

export interface RefreshedTokens {
  accessToken: string;
  refreshToken: string;
  /** Unix seconds. */
  expiresAt: number;
}

function tokenEndpoint(): string | null {
  const issuer = process.env.AUTH_KEYCLOAK_ISSUER;
  if (!issuer) return null;
  return `${issuer.replace(/\/$/, "")}/protocol/openid-connect/token`;
}

export async function refreshAccessToken(
  refreshToken: string,
): Promise<RefreshedTokens | null> {
  const endpoint = tokenEndpoint();
  if (!endpoint) return null;

  try {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({
        grant_type: "refresh_token",
        refresh_token: refreshToken,
        client_id: process.env.AUTH_KEYCLOAK_ID ?? "kubespaces",
      }),
    });
    if (!response.ok) return null;

    const data: {
      access_token?: string;
      refresh_token?: string;
      expires_in?: number;
    } = await response.json();
    if (!data.access_token) return null;

    return {
      accessToken: data.access_token,
      refreshToken: data.refresh_token ?? refreshToken,
      expiresAt: Math.floor(Date.now() / 1000) + (data.expires_in ?? 60),
    };
  } catch {
    return null;
  }
}
