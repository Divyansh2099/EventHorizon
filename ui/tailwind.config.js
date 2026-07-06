/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        bg: '#050505',
        surface: '#0a0a0a',
        line: '#1f1f1f',
        accent: '#ff2a85',
        accent2: '#00d4ff',
        accent3: '#00ff9d',
        textMain: '#ffffff',
        sub: '#888888',
      },
      fontFamily: {
        display: ['Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
      animation: {
        'slideDownSpring': 'slideDownSpring 1.2s cubic-bezier(0.175, 0.885, 0.32, 1.1) forwards',
        'popInSpring': 'popInSpring 0.8s cubic-bezier(0.34, 1.56, 0.64, 1) forwards',
        'breathe': 'breathe 2s ease-in-out infinite',
        'fadeIn': 'fadeIn 0.4s ease forwards',
        'slideRight': 'slideRight linear infinite',
      },
      keyframes: {
        slideDownSpring: {
          '0%': { opacity: '0', transform: 'translateY(-40px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        popInSpring: {
          '0%': { opacity: '0', transform: 'scale(0.95) translateY(10px)' },
          '100%': { opacity: '1', transform: 'scale(1) translateY(0)' },
        },
        breathe: {
          '0%, 100%': { opacity: '0.5', transform: 'scale(0.9)' },
          '50%': { opacity: '1', transform: 'scale(1.1)', boxShadow: '0 0 16px currentColor' },
        },
        fadeIn: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        slideRight: {
          '0%': { left: '-40px' },
          '100%': { left: '100%' },
        }
      }
    },
  },
  plugins: [],
}
