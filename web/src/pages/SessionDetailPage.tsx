import { useEffect, useState } from "react";
import { useParams, useSearchParams, Link } from "react-router-dom";
import { fetchSession } from "@/lib/api";
import type { Session } from "@/lib/types";
import { AgentBadge } from "@/components/AgentBadge";
import { ConversationView } from "@/components/ConversationView";
import { DiffViewer } from "@/components/DiffViewer";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export function SessionDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const commitHash = searchParams.get("commit");

  const authorName = searchParams.get("author") || "You";
  const msgStart = searchParams.get("start");
  const msgEnd = searchParams.get("end");

  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    const opts =
      msgStart != null && msgEnd != null
        ? { start: Number(msgStart), end: Number(msgEnd) }
        : undefined;
    fetchSession(id, opts)
      .then(setSession)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [id, msgStart, msgEnd]);

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

  const hasDiff = !!commitHash;

  // Compute token usage totals
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

  // Find models used in this session
  const modelsUsed = [
    ...new Set(
      (session.messages ?? []).map((m) => m.model).filter(Boolean) as string[],
    ),
  ];

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
          {commitHash && (
            <code className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
              {commitHash.slice(0, 7)}
            </code>
          )}
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

      {/* Token usage + model bar */}
      {totalTokens > 0 && (
        <div className="border-b border-border px-4 py-2 flex items-center gap-4 text-xs font-mono text-muted-foreground shrink-0 bg-muted/30">
          {modelsUsed.map((m) => (
            <span key={m} className="text-foreground font-medium">
              {m}
            </span>
          ))}
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
          {tokenTotals.cacheRead > 0 && (
            <span>
              Cache read:{" "}
              <span className="text-yellow-400">
                {formatTokenCount(tokenTotals.cacheRead)}
              </span>
            </span>
          )}
          {tokenTotals.cacheCreation > 0 && (
            <span>
              Cache write:{" "}
              <span className="text-orange-400">
                {formatTokenCount(tokenTotals.cacheCreation)}
              </span>
            </span>
          )}
        </div>
      )}

      {/* Content */}
      {hasDiff ? (
        // Split view: conversation + diff
        <div className="flex flex-1 overflow-hidden">
          <div className="flex-1 overflow-auto border-r border-border">
            <div className="p-2 border-b border-border">
              <h3 className="text-xs font-mono text-muted-foreground uppercase tracking-wider px-2">
                Conversation
              </h3>
            </div>
            <ConversationView
              messages={session.messages ?? []}
              humanLabel={authorName}
            />
          </div>
          <div className="flex-1 overflow-auto">
            <div className="p-2 border-b border-border">
              <h3 className="text-xs font-mono text-muted-foreground uppercase tracking-wider px-2">
                Diff
              </h3>
            </div>
            <DiffViewer commitHash={commitHash} />
          </div>
        </div>
      ) : (
        // Conversation only with tabs
        <Tabs
          defaultValue="conversation"
          className="flex-1 flex flex-col overflow-hidden"
        >
          <TabsList className="mx-4 mt-2 w-fit">
            <TabsTrigger value="conversation">Conversation</TabsTrigger>
            <TabsTrigger value="info">Info</TabsTrigger>
          </TabsList>
          <TabsContent
            value="conversation"
            className="flex-1 overflow-auto mt-0"
          >
            <ConversationView
              messages={session.messages ?? []}
              humanLabel={authorName}
            />
          </TabsContent>
          <TabsContent value="info" className="flex-1 overflow-auto mt-0 p-4">
            <div className="space-y-3 text-sm font-mono">
              <div>
                <span className="text-muted-foreground">Session ID:</span>{" "}
                {session.id}
              </div>
              <div>
                <span className="text-muted-foreground">Agent:</span>{" "}
                {session.agent.tool} / {session.agent.model}
              </div>
              <div>
                <span className="text-muted-foreground">Project:</span>{" "}
                {session.project}
              </div>
              <div>
                <span className="text-muted-foreground">CWD:</span>{" "}
                {session.cwd}
              </div>
              {session.git_branch && (
                <div>
                  <span className="text-muted-foreground">Branch:</span>{" "}
                  {session.git_branch}
                </div>
              )}
              <div>
                <span className="text-muted-foreground">Messages:</span>{" "}
                {session.messages?.length ?? 0}
              </div>
              <div>
                <span className="text-muted-foreground">Duration:</span>{" "}
                {formatDuration(session.started_at, session.ended_at)}
              </div>
            </div>
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}

function formatDuration(start: string, end: string): string {
  const ms = new Date(end).getTime() - new Date(start).getTime();
  const mins = Math.floor(ms / 60000);
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  return `${hrs}h ${mins % 60}m`;
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}
