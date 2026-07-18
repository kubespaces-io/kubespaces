import { Badge } from "@/components/ui/Badge";
import type { BadgeTone } from "@/components/ui/Badge";
import { isTransitional } from "@/lib/types";
import type { TenantPhase } from "@/lib/types";

const PHASE_TONES: Record<TenantPhase, BadgeTone> = {
  Pending: "amber",
  Provisioning: "amber",
  Ready: "green",
  Failed: "red",
  Deleting: "gray",
  Unknown: "neutral",
};

export function PhaseBadge({ phase }: { phase: TenantPhase }) {
  return (
    <Badge tone={PHASE_TONES[phase] ?? "neutral"} pulse={isTransitional(phase)}>
      {phase}
    </Badge>
  );
}
