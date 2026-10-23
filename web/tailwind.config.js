/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{svelte,js,ts}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        garuda: {
          bg: '#0f1115',
          surface: '#15181e',
          card: '#1b1f27',
          hover: '#222731',
          border: '#2a313d',
          'border-subtle': '#1f242d',
          accent: '#2563eb',
          'accent-hover': '#1d4ed8',
          danger: '#dc2626',
          warning: '#d97706',
          success: '#16a34a',
          text: '#f1f5f9',
          muted: '#94a3b8',
        },
      },
      fontFamily: {
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace'],
        sans: ['-apple-system', 'BlinkMacSystemFont', '"Segoe UI"', 'Roboto', 'Helvetica', 'Arial', 'sans-serif'],
      },
    },
  },
  plugins: [],
};
