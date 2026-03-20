import { useState } from "react";
import type { ToolCall } from "@/lib/types";

export function ToolCallBlock({ toolCall }: { toolCall: ToolCall }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="rounded border border-border/50 bg-background/50 text-xs font-mono">
      <button
        onClick={(e) => { e.preventDefault(); setExpanded(!expanded); }}
        className="w-full flex items-center gap-2 p-2 hover:bg-muted/50 transition-colors text-left"
      >
        <span className={`transition-transform ${expanded ? "rotate-90" : ""}`}>&#9654;</span>
        <span className="text-muted-foreground">Tool:</span>
        <span className="text-foreground font-semibold">{toolCall.tool}</span>
      </button>
      {expanded && (
        <div className="border-t border-border/50 p-2 space-y-2">
          {toolCall.input && (
            <div>
              <div className="text-muted-foreground mb-1">Input:</div>
              <pre className="whitespace-pre-wrap break-all text-[11px] max-h-40 overflow-auto bg-muted/30 p-2 rounded">
                {formatInput(toolCall.input)}
              </pre>
            </div>
          )}
          {toolCall.output && (
            <div>
              <div className="text-muted-foreground mb-1">Output:</div>
              <pre className="whitespace-pre-wrap break-all text-[11px] max-h-40 overflow-auto bg-muted/30 p-2 rounded">
                {toolCall.output.slice(0, 2000)}
                {toolCall.output.length > 2000 && "..."}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function formatInput(input: string): string {
  try {
    return JSON.stringify(JSON.parse(input), null, 2);
  } catch {
    return input;
  }
}
