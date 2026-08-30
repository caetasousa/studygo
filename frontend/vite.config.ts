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
		},
		watch: {
			// Inside Docker the source arrives over a bind mount, and filesystem
			// events from the host do not reliably cross it (notably on WSL2 and
			// macOS). Polling costs a little CPU and is the difference between HMR
			// working and every change needing a manual rebuild. Opt-in via the
			// env var so a native `npm run dev` keeps using real inotify events.
			usePolling: process.env.VITE_USE_POLLING === '1',
			interval: 300
		}
	}
});
