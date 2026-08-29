import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

const apiTarget = process.env.VITE_API_PROXY ?? 'http://localhost:8080';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		port: 5173,
		// In dev, proxy the API to the Go backend so the browser stays same-origin.
		proxy: {
			'/api': { target: apiTarget, changeOrigin: true }
		}
	}
});
