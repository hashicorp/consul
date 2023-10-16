/**
 * A function that parses the `tokens.css`-file from `@hashicorp/design-system-tokens`
 * and creates a map out of it that can be used to build up a color configuration
 * in `tailwind.config.js`.
 *
 * @param {string} tokensPath - The path to `tokens.css` from `@hashicorp/design-system-tokens`
 * @returns { { [string]: string } } An object that contains color names as keys and color values as values.
 */
function colorMapFromTokens(tokensPath) {
  const css = require('css');
  const path = require('path');
  const fs = require('fs');

  const hdsTokensPath = path.join(__dirname, tokensPath);

  const tokensFile = fs.readFileSync(hdsTokensPath, {
    encoding: 'utf8',
    flag: 'r',
  });

  const ast = css.parse(tokensFile);
  const rootVars = ast.stylesheet.rules.filter((r) => r.type !== 'comment')[0];

  // filter out all colors and then create a map out of them
  const vars = rootVars.declarations.filter((d) => d.type !== 'comment');
  const colorPropertyNameCleanupRegex = /^--token-color-(palette-)?/;
  const colors = vars.filter((d) => d.property.match(/^--token-color-/));

  return colors.reduce((acc, d) => {
    acc[d.property.replace(colorPropertyNameCleanupRegex, 'hds-')] = d.value;
    return acc;
  }, {});
}

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['../**/*.{html.js,hbs,mdx}'],
  theme: {
    colors: colorMapFromTokens(
      '../../node_modules/@hashicorp/design-system-tokens/dist/products/css/tokens.css'
    ),
    extend: {},
  },
  plugins: [],
};
