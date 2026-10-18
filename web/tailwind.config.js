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
        stitch: {
          dark: '#0E1410',
          darkSurface: '#161D18',
          darkCard: '#1F2721',
          darkHover: '#28332B',
          darkBorder: '#2C362F',
          primary: '#A5D6A7',
          primaryHover: '#81C784',
          primaryDark: '#2E7D32',
          accent: '#4ADE80',
          textMuted: '#8E9E92',
          textSub: '#A3B4A7',
          textLight: '#E8EFEA',
        }
      },
      fontFamily: {
        sans: ['Manrope', 'system-ui', '-apple-system', 'sans-serif'],
      }
    },
  },
  plugins: [],
}
