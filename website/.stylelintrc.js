module.exports = {
  ...require('@hashicorp/nextjs-scripts/.stylelintrc.js'),
  rules: {
    'selector-pseudo-class-no-unknown': [
      true,
      {
        ignorePseudoClasses: ['first', 'last'],
      },
    ],
  },
}
