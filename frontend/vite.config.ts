import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';
export default defineConfig({ plugins: [tailwindcss()], base: './', server: { host: '127.0.0.1', proxy: {'/api': {target:'http://127.0.0.1:8080',changeOrigin:false}} }, build: {target:'es2022'} });
