module.exports = {
  ...require('@hashicorp/nextjs-scripts/.stylelintrc.js'),
  /* Specify overrides here */
  ignoreFiles: ['public/**/*.css'],
  rules: {
    'selector-pseudo-class-no-unknown': [
      true,
      {
        ignorePseudoClasses: ['first', 'last'],
      },
    ],
  },
}
