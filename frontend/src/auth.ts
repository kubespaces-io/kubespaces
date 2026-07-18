import NextAuth from "next-auth";
import Keycloak from "next-auth/providers/keycloak";
import { refreshAccessToken } from "@/lib/keycloak";

const ACCESS_TOKEN_LEEWAY_SECONDS = 60;

export const { handlers, auth, signIn, signOut } = NextAuth({
  trustHost: true,
  providers: [
    Keycloak({
      clientId: process.env.AUTH_KEYCLOAK_ID ?? "kubespaces",
      issuer: process.env.AUTH_KEYCLOAK_ISSUER,
      // Public client: PKCE instead of a client secret.
      client: { token_endpoint_auth_method: "none" },
      checks: ["pkce", "state"],
    }),
  ],
  session: { strategy: "jwt" },
  callbacks: {
    async jwt({ token, account }) {
      // Initial sign-in: persist Keycloak tokens in the (encrypted) JWT cookie.
      if (account) {
        return {
          ...token,
          accessToken: account.access_token,
          refreshToken: account.refresh_token,
          expiresAt: account.expires_at,
        };
      }

      const expiresAtMs = (token.expiresAt ?? 0) * 1000;
      const isFresh =
        Date.now() < expiresAtMs - ACCESS_TOKEN_LEEWAY_SECONDS * 1000;
      if (isFresh || !token.refreshToken) return token;

      const refreshed = await refreshAccessToken(token.refreshToken);
      if (!refreshed) return { ...token, error: "RefreshTokenError" as const };
      return {
        ...token,
        accessToken: refreshed.accessToken,
        refreshToken: refreshed.refreshToken,
        expiresAt: refreshed.expiresAt,
        error: undefined,
      };
    },
    // NOTE: tokens are intentionally NOT copied into the session object —
    // the session payload is visible to the browser via /api/auth/session.
    async session({ session }) {
      return session;
    },
  },
});
