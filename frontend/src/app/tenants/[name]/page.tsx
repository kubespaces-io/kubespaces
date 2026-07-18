import type { Metadata } from "next";
import { TenantDetail } from "@/components/tenants/TenantDetail";

interface TenantPageProps {
  params: Promise<{ name: string }>;
}

export async function generateMetadata({
  params,
}: TenantPageProps): Promise<Metadata> {
  const { name } = await params;
  return { title: name };
}

export default async function TenantPage({ params }: TenantPageProps) {
  const { name } = await params;
  return <TenantDetail name={name} />;
}
