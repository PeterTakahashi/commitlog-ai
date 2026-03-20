import type { Message } from "@/lib/types";
import { ToolCallBlock } from "./ToolCallBlock";

interface ConversationViewProps {
  messages: Message[];
  humanLabel?: string;
}

export function ConversationView({
  messages,
  humanLabel = "You",
}: ConversationViewProps) {
  // Pre-process: determine which empty human messages are tool approvals
  const approvalSet = new Set<number>();
  for (let i = 0; i < messages.length; i++) {
    const msg = messages[i];
    if (
      msg.role === "human" &&
      !msg.content &&
      (!msg.tool_calls || msg.tool_calls.length === 0)
    ) {
      // Check if previous message was an assistant with tool calls
      if (
        i > 0 &&
        messages[i - 1].role === "assistant" &&
        messages[i - 1].tool_calls &&
        messages[i - 1].tool_calls!.length > 0
      ) {
        approvalSet.add(i);
      }
    }
  }

  return (
    <div className="space-y-4 p-4">
      {messages.map((msg, i) => {
        // Skip empty messages that aren't tool approvals
        if (
          msg.role === "human" &&
          !msg.content &&
          (!msg.tool_calls || msg.tool_calls.length === 0) &&
          !approvalSet.has(i)
        ) {
          return null;
        }

        // Tool approval: show compact inline with what was approved
        if (approvalSet.has(i)) {
          const prevTools = messages[i - 1].tool_calls!;
          const toolNames = prevTools.map((tc) => tc.tool).join(", ");
          return (
            <div key={i} className="flex justify-end">
              <div className="text-xs text-muted-foreground/50 font-mono px-3 py-1">
                <span className="italic">approved</span>
                <span className="ml-1.5 text-muted-foreground/40">
                  {toolNames}
                </span>
                <span className="ml-2">
                  {new Date(msg.timestamp).toLocaleTimeString("en-US", {
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </span>
              </div>
            </div>
          );
        }

        return <MessageBubble key={i} message={msg} humanLabel={humanLabel} />;
      })}
    </div>
  );
}

function MessageBubble({
  message,
  humanLabel,
}: {
  message: Message;
  humanLabel: string;
}) {
  const isHuman = message.role === "human";

  return (
    <div className={`flex ${isHuman ? "justify-end" : "justify-start"}`}>
      <div
        className={`max-w-[85%] rounded-lg p-3 space-y-2 ${
          isHuman ? "bg-zinc-800 text-zinc-100" : "bg-muted"
        }`}
      >
        {/* Role label */}
        <div
          className={`text-xs font-mono ${isHuman ? "text-zinc-400" : "text-muted-foreground"}`}
        >
          {isHuman ? humanLabel : "Assistant"}
          <span className="ml-2">
            {new Date(message.timestamp).toLocaleTimeString("en-US", {
              hour: "2-digit",
              minute: "2-digit",
            })}
          </span>
        </div>

        {/* Content */}
        {message.content && (
          <div className="text-sm whitespace-pre-wrap break-words">
            {message.content}
          </div>
        )}

        {/* Tool calls */}
        {message.tool_calls?.map((tc, i) => (
          <ToolCallBlock key={i} toolCall={tc} />
        ))}
      </div>
    </div>
  );
}
