import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { fetchStats } from "@/lib/api";
import type { Stats } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const AGENT_LABELS: Record<string, { label: string; color: string }> = {
  claude_code: { label: "Claude", color: "bg-orange-500" },
  gemini_cli: { label: "Gemini", color: "bg-blue-500" },
  codex_cli: { label: "Codex", color: "bg-green-500" },
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
              color: "bg-zinc-500",
            };
            const pct = totalByAgent > 0 ? (count / totalByAgent) * 100 : 0;
            return (
              <div key={agent} className="space-y-1">
                <div className="flex justify-between text-sm">
                  <span>{config.label}</span>
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
