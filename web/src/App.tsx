import { BrowserRouter, Routes, Route } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { TimelinePage } from "@/pages/TimelinePage";
import { SessionDetailPage } from "@/pages/SessionDetailPage";
import { StatsPage } from "@/pages/StatsPage";

export default function App() {
  // Apply dark class to <html> so body bg/color use dark theme variables
  document.documentElement.classList.add("dark");

  return (
    <BrowserRouter>
      <TooltipProvider>
        <Routes>
          <Route path="/" element={<TimelinePage />} />
          <Route path="/session/:id" element={<SessionDetailPage />} />
          <Route path="/stats" element={<StatsPage />} />
        </Routes>
      </TooltipProvider>
    </BrowserRouter>
  );
}
