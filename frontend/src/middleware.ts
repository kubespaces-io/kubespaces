import { auth } from "@/auth";

export default auth((request) => {
  if (!request.auth) {
    const signInUrl = new URL("/", request.nextUrl.origin);
    return Response.redirect(signInUrl);
  }
});

export const config = {
  matcher: ["/tenants/:path*"],
};
