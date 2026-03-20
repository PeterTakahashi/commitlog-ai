import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export function ConfidenceDot({ confidence }: { confidence: number }) {
  const pct = Math.round(confidence * 100);
  let color = "bg-red-500";
  if (pct >= 80) color = "bg-green-500";
  else if (pct >= 50) color = "bg-yellow-500";

  return (
    <Tooltip>
      <TooltipTrigger>
        <span className={`inline-block w-2 h-2 rounded-full ${color}`} />
      </TooltipTrigger>
      <TooltipContent>
        <p>Match confidence: {pct}%</p>
      </TooltipContent>
    </Tooltip>
  );
}
