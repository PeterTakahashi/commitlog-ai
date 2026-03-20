import { useEffect, useState } from "react";
import { fetchDiff } from "@/lib/api";

export function DiffViewer({ commitHash }: { commitHash: string }) {
  const [diff, setDiff] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchDiff(commitHash)
      .then(setDiff)
      .catch(() => setDiff(null))
      .finally(() => setLoading(false));
  }, [commitHash]);

  if (loading) return <div className="p-4 text-sm text-muted-foreground">Loading diff...</div>;
  if (!diff) return <div className="p-4 text-sm text-muted-foreground">No diff available</div>;

  return (
    <div className="font-mono text-xs overflow-auto">
      {diff.split("\n").map((line, i) => (
        <DiffLine key={i} line={line} lineNum={i + 1} />
      ))}
    </div>
  );
}

function DiffLine({ line, lineNum }: { line: string; lineNum: number }) {
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
    className += "text-muted-foreground font-bold border-t border-border mt-2 pt-2";
  } else {
    className += "text-muted-foreground";
  }

  return (
    <div className={className}>
      <span className="inline-block w-10 text-right mr-4 text-muted-foreground/40 select-none">{lineNum}</span>
      {line}
    </div>
  );
}
