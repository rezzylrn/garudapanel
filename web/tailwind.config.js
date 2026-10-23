/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{svelte,js,ts}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        garuda: {
          bg: '#0b0f14',
          surface: '#121820',
          card: '#161e29',
          border: '#233042',
          hover: '#1b2636',
          primary: '#0ea5e9',
          accent: '#10b981',
          danger: '#ef4444',
          warning: '#f59e0b',
          muted: '#8599b3',
          text: '#f1f5f9',
        },
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'Fira Code', 'Courier New', 'monospace'],
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
};
