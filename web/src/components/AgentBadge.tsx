import { Badge } from "@/components/ui/badge";
import type { Agent } from "@/lib/types";

const agentConfig: Record<string, { label: string; icon: string }> = {
  claude_code: {
    label: "Claude",
    icon: "/icons/claude.svg",
  },
  gemini_cli: {
    label: "Gemini",
    icon: "/icons/gemini.svg",
  },
  codex_cli: {
    label: "Codex",
    icon: "/icons/codex.svg",
  },
};

export function AgentBadge({ agent }: { agent: Agent }) {
  const config = agentConfig[agent.tool] ?? {
    label: agent.tool,
    icon: "",
  };
  return (
    <Badge
      variant="outline"
      className="bg-zinc-800/50 border-zinc-700 text-zinc-300 font-mono text-xs gap-1.5"
    >
      {config.icon && (
        <img src={config.icon} alt={config.label} className="w-3.5 h-3.5" />
      )}
      {config.label}
      {agent.model && (
        <span className="ml-0.5 opacity-60">
          {agent.model.split("-").slice(-1)[0]}
        </span>
      )}
    </Badge>
  );
}
