import { Link } from "react-router-dom";
import type { TimelineEntry } from "@/lib/types";
import { AgentBadge } from "./AgentBadge";
import { ConfidenceDot } from "./ConfidenceDot";

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function formatTime(dateStr: string) {
  return new Date(dateStr).toLocaleTimeString("en-US", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function groupByDate(entries: TimelineEntry[]): Map<string, TimelineEntry[]> {
  const groups = new Map<string, TimelineEntry[]>();
  for (const entry of entries) {
    const ts = entry.commit?.timestamp ?? entry.session?.started_at ?? "";
    const date = ts ? formatDate(ts) : "Unknown";
    if (!groups.has(date)) groups.set(date, []);
    groups.get(date)!.push(entry);
  }
  return groups;
}

export function TimelineList({ entries }: { entries: TimelineEntry[] }) {
  const grouped = groupByDate(entries);

  return (
    <div className="space-y-6">
      {Array.from(grouped.entries()).map(([date, items]) => (
        <div key={date}>
          <div className="sticky top-0 z-10 bg-background/80 backdrop-blur-sm py-2 mb-3">
            <h3 className="text-xs font-mono text-muted-foreground uppercase tracking-wider">{date}</h3>
          </div>
          <div className="space-y-1">
            {items.map((entry, i) => (
              <TimelineEntryRow key={i} entry={entry} />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

function TimelineEntryRow({ entry }: { entry: TimelineEntry }) {
  const { commit, session, link_confidence } = entry;
  const hasLink = commit && session;
  const commitOnly = commit && !session;

  return (
    <Link
      to={session ? `/session/${session.id}${commit ? `?commit=${commit.hash}` : ""}` : "#"}
      className="group flex items-start gap-3 p-3 rounded-lg hover:bg-muted/50 transition-colors relative"
    >
      {/* Timeline dot + line */}
      <div className="flex flex-col items-center pt-1.5 shrink-0">
        <div className={`w-3 h-3 rounded-full border-2 ${
          hasLink ? "border-primary bg-primary" :
          commitOnly ? "border-muted-foreground bg-transparent" :
          "border-muted-foreground/50 bg-transparent"
        }`} />
        <div className="w-px flex-1 bg-border mt-1 min-h-4" />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0 space-y-1.5">
        {/* Commit info */}
        {commit && (
          <div className="flex items-center gap-2 flex-wrap">
            <code className="text-xs text-muted-foreground font-mono">{commit.hash.slice(0, 7)}</code>
            <span className="text-sm font-medium text-foreground truncate">{commit.message}</span>
          </div>
        )}

        {/* Session info */}
        {session && (
          <div className="flex items-center gap-2 flex-wrap">
            <AgentBadge agent={session.agent} />
            {hasLink && <ConfidenceDot confidence={link_confidence} />}
            <span className="text-xs text-muted-foreground font-mono">
              {formatTime(session.started_at)}
            </span>
          </div>
        )}

        {/* Stats */}
        {commit && (
          <div className="text-xs text-muted-foreground font-mono">
            {commit.files_changed} file{commit.files_changed !== 1 ? "s" : ""}
            {commit.additions > 0 && <span className="text-green-500 ml-2">+{commit.additions}</span>}
            {commit.deletions > 0 && <span className="text-red-500 ml-1">-{commit.deletions}</span>}
          </div>
        )}

        {!commit && session && (
          <p className="text-xs text-muted-foreground italic">no commit linked</p>
        )}
      </div>
    </Link>
  );
}
