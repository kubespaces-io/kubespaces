import Link from "next/link";
import { auth, signOut } from "@/auth";

export async function SiteHeader() {
  const session = await auth();

  return (
    <header className="border-b border-rule">
      <div className="mx-auto flex max-w-4xl items-baseline justify-between px-6 py-5">
        <Link
          href={session ? "/tenants" : "/"}
          className="font-display text-lg font-semibold tracking-tight text-ink transition-colors hover:text-accent"
        >
          KubeSpaces
          <span aria-hidden="true" className="ml-1.5 text-accent">
            ⌁
          </span>
        </Link>

        {session?.user && (
          <div className="flex items-baseline gap-5">
            <span className="hidden text-[0.8125rem] text-ink-muted sm:inline">
              {session.user.email ?? session.user.name}
            </span>
            <form
              action={async () => {
                "use server";
                await signOut({ redirectTo: "/" });
              }}
            >
              <button
                type="submit"
                className="font-display text-sm font-medium text-ink underline-offset-4 transition-colors hover:text-accent hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
              >
                Sign out
              </button>
            </form>
          </div>
        )}
      </div>
    </header>
  );
}
