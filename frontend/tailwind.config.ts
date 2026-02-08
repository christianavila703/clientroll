import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./src/**/*.{ts,tsx}",
    "./src/index.html",
    "./node_modules/@tremor/**/*.{js,ts,jsx,tsx,cjs,mjs}",
  ],
  theme: {
    // Tremor requires transparent and current colors
    transparent: "transparent",
    current: "currentColor",
    extend: {
      colors: {
        primary: {
          50: "#eef2ff",
          100: "#e0e7ff",
          200: "#c7d2fe",
          300: "#a5b4fc",
          400: "#818cf8",
          500: "#6366f1",
          600: "#4f46e5",
          700: "#4338ca",
          800: "#3730a3",
          900: "#312e81",
          950: "#1e1b4b",
        },
        // Tremor semantic tokens (light mode)
        tremor: {
          brand: {
            faint: "#eff6ff",       // blue-50
            muted: "#bfdbfe",       // blue-200
            subtle: "#60a5fa",      // blue-400
            DEFAULT: "#3b82f6",     // blue-500
            emphasis: "#1d4ed8",    // blue-700
            inverted: "#ffffff",    // white
          },
          background: {
            muted: "#f9fafb",       // gray-50
            subtle: "#f3f4f6",      // gray-100
            DEFAULT: "#ffffff",     // white
            emphasis: "#374151",    // gray-700
          },
          border: {
            DEFAULT: "#e5e7eb",     // gray-200
          },
          ring: {
            DEFAULT: "#e5e7eb",     // gray-200
          },
          content: {
            subtle: "#9ca3af",      // gray-400
            DEFAULT: "#6b7280",     // gray-500
            emphasis: "#374151",    // gray-700
            strong: "#111827",      // gray-900
            inverted: "#ffffff",    // white
          },
        },
      },
      boxShadow: {
        // Tremor shadows
        "tremor-input": "0 1px 2px 0 rgb(0 0 0 / 0.05)",
        "tremor-card": "0 1px 3px 0 rgb(0 0 0 / 0.1), 0 1px 2px -1px rgb(0 0 0 / 0.1)",
        "tremor-dropdown": "0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)",
        // App shadows
        "btn": "0 1px 2px 0 rgb(0 0 0 / 0.05)",
        "card": "0 0 0 1px rgb(0 0 0 / 0.05)",
        "dropdown": "0 4px 12px rgb(0 0 0 / 0.08), 0 0 0 1px rgb(0 0 0 / 0.05)",
      },
      borderRadius: {
        "tremor-small": "0.375rem",
        "tremor-default": "0.5rem",
        "tremor-full": "9999px",
      },
      fontSize: {
        "2xs": ["11px", { lineHeight: "16px" }],
        "tremor-label": ["0.75rem", { lineHeight: "1rem" }],
        "tremor-default": ["0.875rem", { lineHeight: "1.25rem" }],
        "tremor-title": ["1.125rem", { lineHeight: "1.75rem" }],
        "tremor-metric": ["1.875rem", { lineHeight: "2.25rem" }],
      },
    },
  },
  // Safelist Tremor classes that are dynamically generated
  safelist: [
    {
      pattern:
        /^(bg-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-(?:50|100|200|300|400|500|600|700|800|900|950))$/,
      variants: ["hover", "ui-selected"],
    },
    {
      pattern:
        /^(text-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-(?:50|100|200|300|400|500|600|700|800|900|950))$/,
      variants: ["hover", "ui-selected"],
    },
    {
      pattern:
        /^(border-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-(?:50|100|200|300|400|500|600|700|800|900|950))$/,
      variants: ["hover", "ui-selected"],
    },
    {
      pattern:
        /^(ring-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-(?:50|100|200|300|400|500|600|700|800|900|950))$/,
    },
    {
      pattern:
        /^(stroke-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-(?:50|100|200|300|400|500|600|700|800|900|950))$/,
    },
    {
      pattern:
        /^(fill-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-(?:50|100|200|300|400|500|600|700|800|900|950))$/,
    },
  ],
  plugins: [],
};

export default config;
