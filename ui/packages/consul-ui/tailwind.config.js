/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['../**/*.{html.js,hbs,mdx}'],
  theme: {
    // disable all color utilities - we want to use HDS instead
    colors: {},
    extend: {},
  },
  plugins: [],
};
