/** @type {import("prettier").Config} */
export default {
  printWidth: 100,
  singleQuote: true,
  semi: true,
  trailingComma: 'all',
  plugins: ['prettier-plugin-astro', 'prettier-plugin-tailwindcss'],
  overrides: [
    {
      files: '*.astro',
      options: { parser: 'astro' },
    },
  ],
};
