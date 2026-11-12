/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        bui: {
          canvas: 'var(--bui-bg-canvas)',
          sidebar: 'var(--bui-bg-sidebar)',
          card: 'var(--bui-bg-card)',
          inner: 'var(--bui-bg-inner)',
          hover: 'var(--bui-bg-hover)',
          track: 'var(--bui-bg-track)',
          border: 'var(--bui-border)',
          borderStrong: 'var(--bui-border-strong)',
          textPrimary: 'var(--bui-text-primary)',
          textSecondary: 'var(--bui-text-secondary)',
          textMuted: 'var(--bui-text-muted)',
          accentBlue: 'var(--bui-accent-blue)',
          accentBlueHover: 'var(--bui-accent-blue-hover)',
          lime: 'var(--bui-lime)',
          teal: 'var(--bui-teal)',
          rose: 'var(--bui-rose)',
          amber: 'var(--bui-amber)',
        }
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      }
    },
  },
  plugins: [],
}
