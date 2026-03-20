import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { fetchSession, fetchSessionCommits } from "@/lib/api";
import type { CommitWithDiff } from "@/lib/api";
import type { Session } from "@/lib/types";
import { AgentBadge } from "@/components/AgentBadge";
import { ConversationView } from "@/components/ConversationView";

export function SessionFullPage() {
  const { id } = useParams<{ id: string }>();
  const [session, setSession] = useState<Session | null>(null);
  const [commits, setCommits] = useState<CommitWithDiff[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<"conversation" | "changes">(
    "conversation",
  );

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    Promise.all([fetchSession(id), fetchSessionCommits(id)])
      .then(([s, c]) => {
        setSession(s);
        setCommits(c);
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen text-muted-foreground">
        Loading...
      </div>
    );
  }

  if (!session) {
    return (
      <div className="flex items-center justify-center h-screen text-muted-foreground">
        Session not found
      </div>
    );
  }

  // Token totals
  const tokenTotals = (session.messages ?? []).reduce(
    (acc, msg) => {
      if (msg.usage) {
        acc.input += msg.usage.input_tokens;
        acc.output += msg.usage.output_tokens;
        acc.cacheCreation += msg.usage.cache_creation_input_tokens ?? 0;
        acc.cacheRead += msg.usage.cache_read_input_tokens ?? 0;
      }
      return acc;
    },
    { input: 0, output: 0, cacheCreation: 0, cacheRead: 0 },
  );
  const totalTokens =
    tokenTotals.input +
    tokenTotals.output +
    tokenTotals.cacheCreation +
    tokenTotals.cacheRead;

  const modelsUsed = [
    ...new Set(
      (session.messages ?? []).map((m) => m.model).filter(Boolean) as string[],
    ),
  ];

  const totalAdditions = commits.reduce((s, c) => s + c.additions, 0);
  const totalDeletions = commits.reduce((s, c) => s + c.deletions, 0);

  return (
    <div className="flex flex-col h-screen">
      {/* Header */}
      <header className="border-b border-border p-4 flex items-center gap-4 shrink-0">
        <Link
          to="/"
          className="text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          &larr; Back
        </Link>
        <div className="flex items-center gap-2 flex-1 min-w-0">
          <AgentBadge agent={session.agent} />
          <span className="text-sm font-mono text-muted-foreground truncate">
            {session.project}
          </span>
          <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
            Full Session
          </span>
        </div>
        <div className="text-xs text-muted-foreground font-mono shrink-0">
          {new Date(session.started_at).toLocaleString()}
          {" - "}
          {new Date(session.ended_at).toLocaleTimeString("en-US", {
            hour: "2-digit",
            minute: "2-digit",
          })}
        </div>
      </header>

      {/* Stats bar */}
      <div className="border-b border-border px-4 py-2 flex items-center gap-4 text-xs font-mono text-muted-foreground shrink-0 bg-muted/30 flex-wrap">
        {modelsUsed.map((m) => (
          <span key={m} className="text-foreground font-medium">
            {m}
          </span>
        ))}
        <span>{session.messages?.length ?? 0} messages</span>
        {totalTokens > 0 && (
          <>
            <span>
              Tokens:{" "}
              <span className="text-foreground">
                {formatTokenCount(totalTokens)}
              </span>
            </span>
            <span>
              In:{" "}
              <span className="text-blue-400">
                {formatTokenCount(tokenTotals.input)}
              </span>
            </span>
            <span>
              Out:{" "}
              <span className="text-green-400">
                {formatTokenCount(tokenTotals.output)}
              </span>
            </span>
          </>
        )}
        {commits.length > 0 && (
          <span>
            {commits.length} commit{commits.length !== 1 ? "s" : ""}
            <span className="text-green-500 ml-2">+{totalAdditions}</span>
            <span className="text-red-500 ml-1">-{totalDeletions}</span>
          </span>
        )}
      </div>

      {/* Tab bar */}
      <div className="border-b border-border px-4 flex gap-0 shrink-0">
        <button
          onClick={() => setActiveTab("conversation")}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === "conversation"
              ? "border-primary text-foreground"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          Conversation
        </button>
        <button
          onClick={() => setActiveTab("changes")}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === "changes"
              ? "border-primary text-foreground"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          Changes
          {commits.length > 0 && (
            <span className="ml-1.5 text-xs text-muted-foreground">
              ({commits.length})
            </span>
          )}
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {activeTab === "conversation" && (
          <ConversationView
            messages={session.messages ?? []}
            humanLabel={commits[0]?.author ?? "You"}
          />
        )}
        {activeTab === "changes" && <SessionChanges commits={commits} />}
      </div>
    </div>
  );
}

function SessionChanges({ commits }: { commits: CommitWithDiff[] }) {
  const [expandedCommit, setExpandedCommit] = useState<string | null>(
    commits.length === 1 ? commits[0]?.hash : null,
  );

  if (commits.length === 0) {
    return (
      <div className="p-8 text-center text-muted-foreground text-sm">
        No linked commits found for this session.
      </div>
    );
  }

  return (
    <div className="divide-y divide-border">
      {commits.map((commit) => (
        <div key={commit.hash}>
          {/* Commit header */}
          <button
            onClick={() =>
              setExpandedCommit(
                expandedCommit === commit.hash ? null : commit.hash,
              )
            }
            className="w-full px-4 py-3 flex items-center gap-3 hover:bg-muted/50 transition-colors text-left"
          >
            <span
              className={`text-xs transition-transform ${expandedCommit === commit.hash ? "rotate-90" : ""}`}
            >
              &#9654;
            </span>
            <code className="text-xs text-muted-foreground font-mono shrink-0">
              {commit.hash.slice(0, 7)}
            </code>
            <span className="text-sm font-medium text-foreground truncate flex-1">
              {commit.message}
            </span>
            <span className="text-xs text-muted-foreground font-mono shrink-0">
              {new Date(commit.timestamp).toLocaleTimeString("en-US", {
                hour: "2-digit",
                minute: "2-digit",
              })}
            </span>
            <span className="text-xs font-mono shrink-0">
              {commit.files_changed} file
              {commit.files_changed !== 1 ? "s" : ""}
              {commit.additions > 0 && (
                <span className="text-green-500 ml-1">+{commit.additions}</span>
              )}
              {commit.deletions > 0 && (
                <span className="text-red-500 ml-1">-{commit.deletions}</span>
              )}
            </span>
          </button>

          {/* Expanded diff */}
          {expandedCommit === commit.hash && (
            <div className="border-t border-border bg-zinc-950/50">
              <InlineDiff diff={commit.diff} />
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

function InlineDiff({ diff }: { diff: string }) {
  if (!diff) {
    return (
      <div className="p-4 text-sm text-muted-foreground">No diff available</div>
    );
  }

  return (
    <div className="font-mono text-xs overflow-auto">
      {diff.split("\n").map((line, i) => {
        let className = "px-4 py-0.5 whitespace-pre ";
        if (line.startsWith("+++") || line.startsWith("---")) {
          className += "text-muted-foreground font-bold";
        } else if (line.startsWith("@@")) {
          className += "text-blue-400 bg-blue-500/10";
        } else if (line.startsWith("+")) {
          className += "text-green-400 bg-green-500/10";
        } else if (line.startsWith("-")) {
          className += "text-red-400 bg-red-500/10";
        } else if (line.startsWith("diff ")) {
          className +=
            "text-muted-foreground font-bold border-t border-border mt-2 pt-2";
        } else {
          className += "text-muted-foreground";
        }

        return (
          <div key={i} className={className}>
            {line}
          </div>
        );
      })}
    </div>
  );
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}
