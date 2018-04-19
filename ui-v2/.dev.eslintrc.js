module.exports = {
  extends: ['./.eslintrc.js'],
  rules: {
    'no-console': 'warn',
    'no-unused-vars': ['error', { args: 'none' }],
    'ember/routes-segments-snake-case': 'warn',
  },
};
