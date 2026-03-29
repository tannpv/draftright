/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ["'Plus Jakarta Sans'", 'ui-sans-serif', 'system-ui', 'sans-serif'],
      },
      colors: {
        dark: {
          bg:     '#202936',
          card:   '#2a3547',
          border: '#333f55',
          input:  '#333f55',
        },
        primary:      '#5d87ff',
        secondary:    '#49beff',
        success:      '#13deb9',
        warning:      '#ffae1f',
        danger:       '#fa896b',
        'text-primary': '#eaeff4',
        'text-muted':   '#7c8fac',
      },
      borderRadius: {
        DEFAULT: '7px',
      },
    },
  },
  plugins: [],
}
