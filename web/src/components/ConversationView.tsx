import type { Message } from "@/lib/types";
import { ToolCallBlock } from "./ToolCallBlock";

export function ConversationView({ messages }: { messages: Message[] }) {
  return (
    <div className="space-y-4 p-4">
      {messages.map((msg, i) => (
        <MessageBubble key={i} message={msg} />
      ))}
    </div>
  );
}

function MessageBubble({ message }: { message: Message }) {
  const isHuman = message.role === "human";

  return (
    <div className={`flex ${isHuman ? "justify-end" : "justify-start"}`}>
      <div className={`max-w-[85%] rounded-lg p-3 space-y-2 ${
        isHuman
          ? "bg-primary text-primary-foreground"
          : "bg-muted"
      }`}>
        {/* Role label */}
        <div className={`text-xs font-mono ${isHuman ? "text-primary-foreground/60" : "text-muted-foreground"}`}>
          {isHuman ? "You" : "Assistant"}
          <span className="ml-2">{new Date(message.timestamp).toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit" })}</span>
        </div>

        {/* Content */}
        {message.content && (
          <div className="text-sm whitespace-pre-wrap break-words">{message.content}</div>
        )}

        {/* Tool calls */}
        {message.tool_calls?.map((tc, i) => (
          <ToolCallBlock key={i} toolCall={tc} />
        ))}
      </div>
    </div>
  );
}
