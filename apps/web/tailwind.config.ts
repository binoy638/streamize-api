import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        background: "oklch(var(--tw-bg) / <alpha-value>)",
        surface: "oklch(var(--tw-surface) / <alpha-value>)",
        accent: "oklch(var(--tw-accent) / <alpha-value>)",
      },
      fontFamily: {
        display: ["var(--font-display)"],
        body: ["var(--font-body)"],
        mono: ["var(--font-mono)"],
      },
      borderRadius: {
        streamize: "var(--radius)",
      },
    },
  },
  plugins: [],
} satisfies Config;
