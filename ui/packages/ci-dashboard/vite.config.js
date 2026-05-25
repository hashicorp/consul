import { defineConfig } from 'vite';

// Base is './' so the built dist/ works whether served from root or a subdirectory
// (e.g. GitHub Pages project page or any static host).
export default defineConfig({
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
