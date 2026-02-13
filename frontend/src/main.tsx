import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { brand } from "./assets/brand";
import "./styles/globals.css";

// Favicon from brand assets
const link = document.querySelector("link[rel='icon']") || document.createElement("link");
link.setAttribute("rel", "icon");
link.setAttribute("href", brand.icon);
link.setAttribute("type", "image/png");
if (!document.querySelector("link[rel='icon']")) document.head.appendChild(link);

const rootElement = document.getElementById("react-app");
if (!rootElement) {
  throw new Error("Root element #react-app not found");
}

createRoot(rootElement).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>
);
