module.exports = {
  content: [
    './client/pages/**/*.{js,ts,jsx,tsx}',
    './client/components/**/*.{js,ts,jsx,tsx}',
  ],
  darkMode: 'media',
  theme: {
    extend: {},
  },
  variants: {
    extend: {},
  },
  plugins: [
    require('@tailwindcss/forms'),
  ],
}
