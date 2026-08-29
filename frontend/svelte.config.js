import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		// Pure SPA: build to static files, serve index.html for every route.
		adapter: adapter({ fallback: 'index.html' }),
		alias: {
			$lib: 'src/lib'
		}
	}
};

export default config;
