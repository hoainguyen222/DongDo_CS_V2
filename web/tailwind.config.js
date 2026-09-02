/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          navy: '#1C2D56',
          'navy-dark': '#0A0F1D',
          'navy-medium': '#162344',
          'navy-light': '#2A3F74',
          red: '#95252E',
          'red-light': '#B32D38',
          'red-dark': '#7A1E26',
        },
        bg: {
          dark: '#0A0F1D',
          card: 'rgba(22, 35, 68, 0.75)',
          surface: 'rgba(28, 45, 86, 0.6)',
        },
      },
    },
  },
  plugins: [],
};
