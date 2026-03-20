import { Badge } from "@/components/ui/badge";
import type { Agent } from "@/lib/types";

const agentConfig: Record<string, { label: string; color: string; bg: string }> = {
  claude_code: { label: "Claude", color: "text-orange-400", bg: "bg-orange-500/15 border-orange-500/30" },
  gemini_cli: { label: "Gemini", color: "text-blue-400", bg: "bg-blue-500/15 border-blue-500/30" },
  codex_cli: { label: "Codex", color: "text-green-400", bg: "bg-green-500/15 border-green-500/30" },
};

export function AgentBadge({ agent }: { agent: Agent }) {
  const config = agentConfig[agent.tool] ?? { label: agent.tool, color: "text-zinc-400", bg: "bg-zinc-500/15 border-zinc-500/30" };
  return (
    <Badge variant="outline" className={`${config.bg} ${config.color} font-mono text-xs`}>
      {config.label}
      {agent.model && <span className="ml-1 opacity-60">{agent.model.split("-").slice(-1)[0]}</span>}
    </Badge>
  );
}
