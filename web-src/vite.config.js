import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';
import path from 'path';

export default defineConfig({
  plugins: [preact()],

  resolve: {
    alias: {
      // Page-specific
      '@pages': path.resolve(__dirname, 'src/pages'),
      '@store': path.resolve(__dirname, 'src/store'),
      '@services': path.resolve(__dirname, 'src/services'),
      '@query': path.resolve(__dirname, 'src/components/query'),

      // Shared UI components and styles
      '@components': path.resolve(__dirname, 'src/components'),
      '@styles': path.resolve(__dirname, 'src/styles'),
      '@constants': path.resolve(__dirname, 'src/constants'),
      '@utils': path.resolve(__dirname, 'src/utils'),
    },
  },

  optimizeDeps: {
    // Force prebundle these to ensure single instance
    include: ['preact', 'preact/hooks', '@preact/signals', 'lucide-preact'],
  },

  build: {
    outDir: '../web/dist',
    emptyOutDir: true,
    // Single bundle for embedding
    rollupOptions: {
      output: {
        manualChunks: undefined,
      },
    },
  },

  server: {
    // Proxy API calls to Go backend during development
    proxy: {
      '/api': {
        target: 'http://localhost:2369',
        changeOrigin: true,
      },
    },
  },
});
