import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/app";
import { FatalReport } from "@/components/fatal-report";
import { loadEmbeddedReport } from "@/lib/report";
import "@/styles.css";

async function bootstrap(): Promise<void> {
  const rootElement = document.getElementById("root");
  if (rootElement === null) {
    throw new Error("The evidence explorer root element is missing.");
  }
  const root = createRoot(rootElement);
  try {
    const report = await loadEmbeddedReport();
    root.render(
      <StrictMode>
        <App report={report} />
      </StrictMode>,
    );
  } catch (error: unknown) {
    root.render(<FatalReport error={error} />);
  }
}

void bootstrap();
