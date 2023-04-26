/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

module.exports = {
  root: true,
  parser: 'babel-eslint',
  parserOptions: {
    ecmaVersion: 2018,
    sourceType: 'module',
    ecmaFeatures: {
      legacyDecorators: true,
    },
  },
  plugins: ['ember'],
  extends: ['eslint:recommended', 'plugin:ember/recommended', 'plugin:prettier/recommended'],
  env: {
    browser: true,
  },
  rules: {
    'no-console': ['error', { allow: ['error', 'info'] }],
    'no-unused-vars': ['error', { args: 'none' }],
    'ember/no-new-mixins': ['warn'],
    'ember/no-jquery': 'warn',
    'ember/no-global-jquery': 'warn',

    // for 3.24 update
    'ember/classic-decorator-no-classic-methods': ['warn'],
    'ember/classic-decorator-hooks': ['warn'],
    'ember/no-classic-classes': ['warn'],
    'ember/no-mixins': ['warn'],
    'ember/no-computed-properties-in-native-classes': ['warn'],
    'ember/no-private-routing-service': ['warn'],
    'ember/no-test-import-export': ['warn'],
    'ember/no-actions-hash': ['warn'],
    'ember/no-classic-components': ['warn'],
    'ember/no-component-lifecycle-hooks': ['warn'],
    'ember/require-tagless-components': ['warn'],
    'ember/no-legacy-test-waiters': ['warn'],
    'ember/no-empty-glimmer-component-classes': ['warn'],
    'ember/no-get': ['off'], // be careful with autofix, might change behavior
    'ember/require-computed-property-dependencies': ['off'], // be careful with autofix
    'ember/use-ember-data-rfc-395-imports': ['off'], // be carful with autofix
    'ember/require-super-in-lifecycle-hooks': ['off'], // be careful with autofix
    'ember/require-computed-macros': ['off'], // be careful with autofix
  },
  overrides: [
    // node files
    {
      files: [
        './tailwind.config.js',
        './.docfy-config.js',
        './.eslintrc.js',
        './.prettierrc.js',
        './.template-lintrc.js',
        './ember-cli-build.js',
        './testem.js',
        './blueprints/*/index.js',
        './config/**/*.js',
        './lib/*/index.js',
        './server/**/*.js',
      ],
      parserOptions: {
        sourceType: 'script',
      },
      env: {
        browser: false,
        node: true,
      },
      plugins: ['node'],
      rules: Object.assign({}, require('eslint-plugin-node').configs.recommended.rules, {
        // add your custom rules and overrides for node files here

        // this can be removed once the following is fixed
        // https://github.com/mysticatea/eslint-plugin-node/issues/77
        'node/no-unpublished-require': 'off',
      }),
    },
    {
      // Test files:
      files: ['tests/**/*-test.{js,ts}'],
      extends: ['plugin:qunit/recommended'],
    },
  ],
};
