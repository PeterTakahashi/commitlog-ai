import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { fetchStats } from "@/lib/api";
import type { Stats } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const AGENT_LABELS: Record<
  string,
  { label: string; icon: string; color: string }
> = {
  claude_code: {
    label: "Claude",
    icon: "/icons/claude.svg",
    color: "bg-orange-500",
  },
  gemini_cli: {
    label: "Gemini",
    icon: "/icons/gemini.svg",
    color: "bg-blue-500",
  },
  codex_cli: {
    label: "Codex",
    icon: "/icons/codex.svg",
    color: "bg-green-500",
  },
};

export function StatsPage() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchStats()
      .then(setStats)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen text-muted-foreground">
        Loading...
      </div>
    );
  }

  if (!stats) {
    return (
      <div className="flex items-center justify-center h-screen text-muted-foreground">
        No stats available
      </div>
    );
  }

  const totalByAgent = Object.values(stats.by_agent).reduce((a, b) => a + b, 0);

  return (
    <div className="min-h-screen p-6 max-w-4xl mx-auto space-y-6">
      <div className="flex items-center gap-4">
        <Link
          to="/"
          className="text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          &larr; Back
        </Link>
        <h1 className="text-xl font-bold font-mono">Stats</h1>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard title="Sessions" value={stats.total_sessions} />
        <StatCard title="Messages" value={stats.total_messages} />
        <StatCard title="Linked" value={stats.linked} />
        <StatCard title="Timeline" value={stats.total_entries} />
      </div>

      {/* Agent breakdown */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-mono">Sessions by Agent</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {Object.entries(stats.by_agent).map(([agent, count]) => {
            const config = AGENT_LABELS[agent] ?? {
              label: agent,
              icon: "",
              color: "bg-zinc-500",
            };
            const pct = totalByAgent > 0 ? (count / totalByAgent) * 100 : 0;
            return (
              <div key={agent} className="space-y-1">
                <div className="flex justify-between text-sm">
                  <span className="flex items-center gap-2">
                    {config.icon && (
                      <img
                        src={config.icon}
                        alt={config.label}
                        className="w-4 h-4"
                      />
                    )}
                    {config.label}
                  </span>
                  <span className="font-mono text-muted-foreground">
                    {count} ({Math.round(pct)}%)
                  </span>
                </div>
                <div className="h-2 rounded-full bg-muted overflow-hidden">
                  <div
                    className={`h-full rounded-full ${config.color}`}
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </div>
            );
          })}
        </CardContent>
      </Card>

      {/* Diff by Agent */}
      {stats.diff_by_agent && Object.keys(stats.diff_by_agent).length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-mono">Diff by Agent</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {(() => {
              const entries = Object.entries(stats.diff_by_agent);
              const maxLines = Math.max(
                ...entries.map(([, d]) => d.additions + d.deletions),
                1,
              );
              return entries.map(([agent, diff]) => {
                const config = AGENT_LABELS[agent] ?? {
                  label: agent,
                  icon: "",
                  color: "bg-zinc-500",
                };
                const total = diff.additions + diff.deletions;
                const addPct =
                  maxLines > 0 ? (diff.additions / maxLines) * 100 : 0;
                const delPct =
                  maxLines > 0 ? (diff.deletions / maxLines) * 100 : 0;
                return (
                  <div key={agent} className="space-y-2">
                    <div className="flex justify-between text-sm">
                      <span className="flex items-center gap-2">
                        {config.icon && (
                          <img
                            src={config.icon}
                            alt={config.label}
                            className="w-4 h-4"
                          />
                        )}
                        {config.label}
                      </span>
                      <span className="font-mono text-muted-foreground text-xs">
                        {diff.commits} commit{diff.commits !== 1 ? "s" : ""} /{" "}
                        {diff.files_changed} file
                        {diff.files_changed !== 1 ? "s" : ""}
                      </span>
                    </div>
                    <div className="flex h-3 rounded-full bg-muted overflow-hidden">
                      <div
                        className="h-full bg-green-500"
                        style={{ width: `${addPct}%` }}
                      />
                      <div
                        className="h-full bg-red-500"
                        style={{ width: `${delPct}%` }}
                      />
                    </div>
                    <div className="flex gap-4 text-xs font-mono">
                      <span className="text-green-500">
                        +{diff.additions.toLocaleString()}
                      </span>
                      <span className="text-red-500">
                        -{diff.deletions.toLocaleString()}
                      </span>
                      <span className="text-muted-foreground">
                        {total.toLocaleString()} total
                      </span>
                    </div>
                  </div>
                );
              });
            })()}
          </CardContent>
        </Card>
      )}

      {/* Tokens by Agent */}
      {stats.token_by_agent && Object.keys(stats.token_by_agent).length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-mono">Tokens by Agent</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {(() => {
              const entries = Object.entries(stats.token_by_agent);
              const maxTokens = Math.max(
                ...entries.map(([, t]) => t.input_tokens + t.output_tokens),
                1,
              );
              return entries.map(([agent, tok]) => {
                const config = AGENT_LABELS[agent] ?? {
                  label: agent,
                  icon: "",
                  color: "bg-zinc-500",
                };
                const total = tok.input_tokens + tok.output_tokens;
                const inPct =
                  maxTokens > 0 ? (tok.input_tokens / maxTokens) * 100 : 0;
                const outPct =
                  maxTokens > 0 ? (tok.output_tokens / maxTokens) * 100 : 0;
                const cacheTokens =
                  tok.cache_read_input_tokens + tok.cache_creation_input_tokens;
                return (
                  <div key={agent} className="space-y-2">
                    <div className="flex justify-between text-sm">
                      <span className="flex items-center gap-2">
                        {config.icon && (
                          <img
                            src={config.icon}
                            alt={config.label}
                            className="w-4 h-4"
                          />
                        )}
                        {config.label}
                      </span>
                      <span className="font-mono text-muted-foreground text-xs">
                        {total.toLocaleString()} tokens
                      </span>
                    </div>
                    <div className="flex h-3 rounded-full bg-muted overflow-hidden">
                      <div
                        className="h-full bg-blue-500"
                        style={{ width: `${inPct}%` }}
                      />
                      <div
                        className="h-full bg-amber-500"
                        style={{ width: `${outPct}%` }}
                      />
                    </div>
                    <div className="flex gap-4 text-xs font-mono">
                      <span className="text-blue-500">
                        in: {tok.input_tokens.toLocaleString()}
                      </span>
                      <span className="text-amber-500">
                        out: {tok.output_tokens.toLocaleString()}
                      </span>
                      {cacheTokens > 0 && (
                        <span className="text-muted-foreground">
                          cache: {cacheTokens.toLocaleString()}
                        </span>
                      )}
                    </div>
                  </div>
                );
              });
            })()}
          </CardContent>
        </Card>
      )}

      {/* Link breakdown */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-mono">Link Status</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-3 gap-4 text-center">
            <div>
              <div className="text-2xl font-bold font-mono text-green-500">
                {stats.linked}
              </div>
              <div className="text-xs text-muted-foreground">Linked</div>
            </div>
            <div>
              <div className="text-2xl font-bold font-mono text-muted-foreground">
                {stats.commit_only}
              </div>
              <div className="text-xs text-muted-foreground">Commit Only</div>
            </div>
            <div>
              <div className="text-2xl font-bold font-mono text-muted-foreground">
                {stats.session_only}
              </div>
              <div className="text-xs text-muted-foreground">Session Only</div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function StatCard({ title, value }: { title: string; value: number }) {
  return (
    <Card>
      <CardContent className="pt-4">
        <div className="text-2xl font-bold font-mono">{value}</div>
        <div className="text-xs text-muted-foreground">{title}</div>
      </CardContent>
    </Card>
  );
}
