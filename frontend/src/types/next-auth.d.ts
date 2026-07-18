import "next-auth/jwt";

declare module "next-auth/jwt" {
  interface JWT {
    /** Keycloak access token — server-side only, never sent to the browser. */
    accessToken?: string;
    refreshToken?: string;
    /** Unix seconds at which accessToken expires. */
    expiresAt?: number;
    error?: "RefreshTokenError";
  }
}
