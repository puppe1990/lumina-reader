const { execSync } = require("node:child_process");

// #372: the kit HTML lives inside the Go module, not in web/templates, so
// Tailwind never sees its utility classes and a fresh app renders unstyled
// components. Resolve the module the same way Go does; a local replace
// (amarra-cais link) keeps working.
function shippedKitContent() {
  try {
    const out = execSync("go list -m -f {{.Dir}} github.com/puppe1990/amarra-cais", {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    });
    const dir = out.trim().replace(/\\/g, "/");
    return dir ? [dir + "/pkg/amarra/view/components/**/*.html"] : [];
  } catch {
    return [];
  }
}

/** @type {import("tailwindcss").Config} */
module.exports = {
  content: [
    "./web/templates/**/*.html",
    // o JS do leitor monta classes em runtime (toast, chat, camada de texto do
    // PDF); sem escanear esses arquivos o Tailwind poda as classes.
    "./web/static/js/*.js",
    ...shippedKitContent(),
  ],
  safelist: [
    "cais-password-wrap",
    "cais-password-toggle",
    "cais-chat-scroll-down",
    "cais-thinking",
    "cais-thinking-dots",
    "cais-select-search",
    "cais-select-search-native",
    "cais-select-search-trigger",
    "cais-select-search-panel",
    "cais-select-search-input",
    "cais-select-search-list",
    "cais-select-search-option",
    "cais-select-search-label",
    "cais-select-search-chevron",
    "is-selected",
    "is-highlighted",
    "is-hidden",
    "text-primary",
    "text-on-surface-variant",
    /* classes montadas com valores do banco (capa-nota-*) não aparecem
       inteiras nos templates; sem o safelist o Tailwind as poda. */
    { pattern: /^capa-tom-[0-5]$/ },
    { pattern: /^nota-(amarelo|verde|azul|rosa)$/ },
    { pattern: /^nota-dot-(amarelo|verde|azul|rosa)$/ },
  ],
  theme: {
    extend: {
      colors: {
        /* kit do amarra: variáveis CSS (input.css) para o tema claro de papel */
        ink: "rgb(var(--amarra-ink) / <alpha-value>)",
        foam: "rgb(var(--amarra-foam) / <alpha-value>)",
        copper: "rgb(var(--amarra-copper) / <alpha-value>)",
        tide: "rgb(var(--amarra-tide) / <alpha-value>)",
        coral: "rgb(var(--amarra-coral) / <alpha-value>)",
        /* Lumina Editorial Reader (DESIGN.md do stitch) */
        surface: "#fcf9f8",
        "surface-low": "#f6f3f2",
        "surface-container": "#f0eded",
        "surface-high": "#eae7e7",
        "surface-highest": "#e5e2e1",
        "on-surface": "#1c1b1b",
        "on-surface-variant": "#564336",
        primary: "#904900",
        "primary-container": "#b55d00",
        "primary-fixed": "#ffdcc5",
        "on-primary": "#ffffff",
        "on-primary-container": "#fffbff",
        secondary: "#615e5c",
        "secondary-container": "#e7e1df",
        "on-secondary-container": "#676462",
        outline: "#897364",
        "outline-variant": "#dcc1b0",
        "inverse-surface": "#313030",
        "inverse-on-surface": "#f3f0ef",
        error: "#ba1a1a",
        "error-container": "#ffdad6",
        "on-error": "#ffffff",
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "sans-serif"],
        serif: ["Literata", "Iowan Old Style", "Palatino Linotype", "Palatino", "Georgia", "serif"],
        mono: ["IBM Plex Mono", "ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
      boxShadow: {
        "2xs": "0 1px 2px 0 rgb(0 0 0 / 0.05)",
        xs: "0 1px 2px 0 rgb(0 0 0 / 0.05)",
      },
    },
  },
  plugins: [],
};
