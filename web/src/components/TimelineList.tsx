import { Link } from "react-router-dom";
import type { TimelineEntry } from "@/lib/types";
import { AgentBadge } from "./AgentBadge";
import { AuthorAvatar } from "./AuthorAvatar";
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
            <h3 className="text-xs font-mono text-muted-foreground uppercase tracking-wider">
              {date}
            </h3>
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
  const { commit, session, link_confidence, manual_commit } = entry;
  const hasLink = commit && session;
  const commitOnly = commit && !session;

  // Build URL with message range params for segmented sessions
  const buildSessionUrl = () => {
    if (!session) return "#";
    const params = new URLSearchParams();
    if (commit) {
      params.set("commit", commit.hash);
      params.set("author", commit.author);
    }
    if (entry.message_start_idx != null && entry.message_end_idx != null) {
      params.set("start", String(entry.message_start_idx));
      params.set("end", String(entry.message_end_idx));
    }
    const qs = params.toString();
    return `/session/${session.id}${qs ? `?${qs}` : ""}`;
  };

  return (
    <Link
      to={buildSessionUrl()}
      className="group flex items-start gap-3 p-3 rounded-lg hover:bg-muted/50 transition-colors relative"
    >
      {/* Timeline dot + line */}
      <div className="flex flex-col items-center pt-1.5 shrink-0">
        <div
          className={`w-3 h-3 rounded-full border-2 ${
            hasLink
              ? "border-primary bg-primary"
              : commitOnly
                ? "border-muted-foreground bg-transparent"
                : "border-muted-foreground/50 bg-transparent"
          }`}
        />
        <div className="w-px flex-1 bg-border mt-1 min-h-4" />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0 space-y-1.5">
        {/* Commit info */}
        {commit && (
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              <code className="text-xs text-muted-foreground font-mono">
                {commit.hash.slice(0, 7)}
              </code>
              <span className="text-sm font-medium text-foreground truncate">
                {commit.message.split("\n")[0]}
              </span>
            </div>
            {commit.message.includes("\n") && (
              <p className="mt-1 text-sm font-medium text-foreground whitespace-pre-line">
                {commit.message.split("\n").slice(1).join("\n").trim()}
              </p>
            )}
          </div>
        )}

        {/* Session info */}
        {session && (
          <div className="flex items-center gap-2 flex-wrap">
            {manual_commit ? (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-mono bg-blue-500/10 border border-blue-500/30 text-blue-400">
                Manual
              </span>
            ) : (
              <AgentBadge agent={session.agent} />
            )}
            {hasLink && <ConfidenceDot confidence={link_confidence} />}
            <span className="text-xs text-muted-foreground font-mono">
              {formatTime(session.started_at)}
            </span>
            {entry.message_start_idx != null &&
              entry.message_end_idx != null && (
                <span className="text-xs text-muted-foreground/70 font-mono">
                  ({entry.message_end_idx - entry.message_start_idx} msgs)
                </span>
              )}
            {session && (
              <Link
                to={`/session/${session.id}/full`}
                onClick={(e) => e.stopPropagation()}
                className="text-xs text-primary/60 hover:text-primary transition-colors"
              >
                full
              </Link>
            )}
          </div>
        )}

        {/* Author + Stats */}
        {commit && (
          <div className="flex items-center gap-3 text-xs text-muted-foreground font-mono">
            <AuthorAvatar
              name={commit.author}
              email={commit.author_email}
              size={20}
            />
            <span>
              {commit.files_changed} file{commit.files_changed !== 1 ? "s" : ""}
              {commit.additions > 0 && (
                <span className="text-green-500 ml-2">+{commit.additions}</span>
              )}
              {commit.deletions > 0 && (
                <span className="text-red-500 ml-1">-{commit.deletions}</span>
              )}
            </span>
          </div>
        )}

        {!commit && session && (
          <p className="text-xs text-muted-foreground italic">
            no commit linked
          </p>
        )}
      </div>
    </Link>
  );
}
