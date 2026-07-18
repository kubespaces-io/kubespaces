import type { Metadata } from "next";
import { Inter, Space_Grotesk } from "next/font/google";
import { SiteHeader } from "@/components/layout/SiteHeader";
import "./globals.css";

const spaceGrotesk = Space_Grotesk({
  subsets: ["latin"],
  variable: "--font-space-grotesk",
  display: "swap",
});

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
});

export const metadata: Metadata = {
  title: {
    default: "KubeSpaces",
    template: "%s — KubeSpaces",
  },
  description:
    "Open control plane for virtual Kubernetes tenants. Create a tenant, get a cluster.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${spaceGrotesk.variable} ${inter.variable}`}>
      <body className="flex min-h-dvh flex-col">
        <SiteHeader />
        <main className="mx-auto w-full max-w-4xl flex-1 px-6 py-12">
          {children}
        </main>
        <footer className="border-t border-rule">
          <div className="mx-auto flex max-w-4xl items-baseline justify-between px-6 py-5 text-xs text-ink-faint">
            <span>KubeSpaces — open control plane for virtual Kubernetes tenants</span>
            <span className="tabular-nums">v0.1</span>
          </div>
        </footer>
      </body>
    </html>
  );
}
