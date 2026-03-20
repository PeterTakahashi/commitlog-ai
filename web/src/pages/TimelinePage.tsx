import { useEffect, useState, useRef, useCallback } from "react";
import { fetchTimeline, fetchBranches } from "@/lib/api";
import type { TimelineEntry } from "@/lib/types";
import { TimelineList } from "@/components/TimelineList";
import { Link } from "react-router-dom";

const PAGE_SIZE = 50;

const AGENT_FILTERS = [
  { value: "", label: "All", icon: "" },
  { value: "claude_code", label: "Claude", icon: "/icons/claude.svg" },
  { value: "gemini_cli", label: "Gemini", icon: "/icons/gemini.svg" },
  { value: "codex_cli", label: "Codex", icon: "/icons/codex.svg" },
];

export function TimelinePage() {
  const [entries, setEntries] = useState<TimelineEntry[]>([]);
  const [gitRepo, setGitRepo] = useState("");
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [agent, setAgent] = useState("");
  const [branch, setBranch] = useState("");
  const [branches, setBranches] = useState<string[]>([]);
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(null);

  const sentinelRef = useRef<HTMLDivElement>(null);

  // Fetch branches once on mount
  useEffect(() => {
    fetchBranches().then(setBranches).catch(console.error);
  }, []);

  // Reset and fetch page 1 when filters change
  useEffect(() => {
    setEntries([]);
    setPage(1);
    setLoading(true);

    fetchTimeline({
      agent: agent || undefined,
      page: 1,
      pageSize: PAGE_SIZE,
      search: search || undefined,
      branch: branch || undefined,
    })
      .then((data) => {
        setEntries(data.entries ?? []);
        setGitRepo(data.git_repo);
        setTotal(data.total);
        setHasMore(data.has_more);
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [agent, search, branch]);

  // Load next page
  const loadMore = useCallback(() => {
    if (loadingMore || !hasMore) return;
    const nextPage = page + 1;
    setLoadingMore(true);

    fetchTimeline({
      agent: agent || undefined,
      page: nextPage,
      pageSize: PAGE_SIZE,
      search: search || undefined,
      branch: branch || undefined,
    })
      .then((data) => {
        setEntries((prev) => [...prev, ...(data.entries ?? [])]);
        setPage(nextPage);
        setHasMore(data.has_more);
      })
      .catch(console.error)
      .finally(() => setLoadingMore(false));
  }, [page, agent, search, branch, hasMore, loadingMore]);

  // Intersection observer for infinite scroll
  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          loadMore();
        }
      },
      { rootMargin: "200px" },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [loadMore]);

  const handleSearchInput = (value: string) => {
    setSearchInput(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => setSearch(value), 300);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (debounceRef.current) clearTimeout(debounceRef.current);
    setSearch(searchInput);
  };

  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="w-56 shrink-0 border-r border-border p-4 space-y-6 overflow-auto">
        <div>
          <Link to="/" className="text-lg font-bold font-mono text-foreground">
            commitlog-ai
          </Link>
          {gitRepo && (
            <p className="text-xs text-muted-foreground font-mono mt-1 truncate">
              {gitRepo.split("/").pop()}
            </p>
          )}
        </div>

        {/* Search */}
        <form onSubmit={handleSearch} className="space-y-1">
          <h4 className="text-xs font-mono text-muted-foreground uppercase tracking-wider">
            Search
          </h4>
          <input
            type="text"
            value={searchInput}
            onChange={(e) => handleSearchInput(e.target.value)}
            placeholder="commit hash or message..."
            className="w-full text-sm bg-muted border border-border rounded px-2 py-1.5 text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </form>

        <div className="space-y-2">
          <h4 className="text-xs font-mono text-muted-foreground uppercase tracking-wider">
            Filter
          </h4>
          {AGENT_FILTERS.map((f) => (
            <button
              key={f.value}
              onClick={() => setAgent(f.value)}
              className={`flex items-center gap-2 w-full text-left px-2 py-1.5 rounded text-sm transition-colors ${
                agent === f.value
                  ? "bg-primary text-primary-foreground"
                  : "hover:bg-muted text-foreground"
              }`}
            >
              {f.icon && <img src={f.icon} alt={f.label} className="w-4 h-4" />}
              {f.label}
            </button>
          ))}
        </div>

        {branches.length > 0 && (
          <div className="space-y-2">
            <h4 className="text-xs font-mono text-muted-foreground uppercase tracking-wider">
              Branch
            </h4>
            <select
              value={branch}
              onChange={(e) => setBranch(e.target.value)}
              className="w-full text-sm bg-muted border border-border rounded px-2 py-1.5 text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            >
              <option value="">All branches</option>
              {branches.map((b) => (
                <option key={b} value={b}>
                  {b}
                </option>
              ))}
            </select>
          </div>
        )}

        <div className="pt-4 border-t border-border">
          <Link
            to="/stats"
            className="text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            Stats
          </Link>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto p-6">
        {loading ? (
          <div className="flex items-center justify-center h-full text-muted-foreground">
            Loading...
          </div>
        ) : entries.length > 0 ? (
          <>
            <div className="mb-4 text-sm text-muted-foreground font-mono">
              {entries.length} of {total} entries
              {search && (
                <span className="ml-2">matching &quot;{search}&quot;</span>
              )}
            </div>
            <TimelineList entries={entries} />

            {/* Infinite scroll sentinel */}
            <div
              ref={sentinelRef}
              className="h-10 flex items-center justify-center"
            >
              {loadingMore && (
                <span className="text-sm text-muted-foreground">
                  Loading more...
                </span>
              )}
            </div>
          </>
        ) : (
          <div className="flex items-center justify-center h-full text-muted-foreground">
            <div className="text-center space-y-2">
              <p className="text-lg">No timeline data</p>
              <p className="text-sm">
                Run{" "}
                <code className="bg-muted px-2 py-0.5 rounded">
                  commitlog-ai parse && commitlog-ai link
                </code>{" "}
                first
              </p>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
