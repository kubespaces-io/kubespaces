import { redirect } from "next/navigation";
import { auth, signIn } from "@/auth";

export default async function LandingPage() {
  const session = await auth();
  if (session) redirect("/tenants");

  return (
    <section aria-labelledby="landing-heading" className="py-16 sm:py-24">
      <p className="text-[0.6875rem] font-medium uppercase tracking-[0.24em] text-accent">
        KubeSpaces — self-service portal
      </p>
      <h1
        id="landing-heading"
        className="mt-6 max-w-2xl font-display text-[clamp(2.75rem,7vw,5rem)] font-medium leading-[1.02] tracking-tight"
      >
        A Kubernetes cluster of your own,{" "}
        <span className="text-accent">in seconds.</span>
      </h1>
      <p className="mt-8 max-w-md text-[0.9375rem] leading-relaxed text-ink-muted">
        Sign in to create virtual tenants, watch them provision, and download a
        kubeconfig the moment they turn Ready.
      </p>

      <form
        className="mt-12"
        action={async () => {
          "use server";
          await signIn("keycloak", { redirectTo: "/tenants" });
        }}
      >
        <button
          type="submit"
          className="inline-flex items-center gap-3 bg-accent px-6 py-3 font-display text-sm font-medium tracking-tight text-white transition-colors hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        >
          Sign in with Keycloak
          <span aria-hidden="true">→</span>
        </button>
      </form>

      <dl className="mt-24 grid max-w-2xl grid-cols-1 gap-x-12 gap-y-8 border-t border-rule pt-8 sm:grid-cols-3">
        {[
          ["01", "Create", "Name a tenant, set optional quotas."],
          ["02", "Watch", "Provisioning status, live."],
          ["03", "Connect", "Download the kubeconfig, kubectl away."],
        ].map(([index, title, text]) => (
          <div key={index}>
            <dt className="flex items-baseline gap-3">
              <span className="font-display text-xs tabular-nums text-ink-faint">
                {index}
              </span>
              <span className="font-display text-base font-medium tracking-tight">
                {title}
              </span>
            </dt>
            <dd className="mt-2 text-[0.8125rem] leading-relaxed text-ink-muted">
              {text}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
