import React from "react";
import ReactDOM from "react-dom/client";
import "@blocknote/core/fonts/inter.css";
import "@blocknote/mantine/style.css";
import "./index.css";
import { installExternalLinkHandling } from "./services/externalLinks";
import App from "./App";

// Before the first render: links must never reach WebView2's own window.open.
installExternalLinkHandling();

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
