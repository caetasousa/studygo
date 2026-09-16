import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		// Pure SPA: build to static files, serve index.html for every route.
		adapter: adapter({ fallback: 'index.html' }),
		// O SvelteKit liga o app com um <script> embutido, e por isso a CSP da
		// borda precisa de 'unsafe-inline' — ela é a mesma para toda versão e não
		// tem como saber o hash de cada build. Aqui o build calcula o hash e o
		// põe num <meta> da página: o navegador exige as duas políticas, então
		// vale a mais estrita, e injeção de HTML deixa de virar execução.
		csp: {
			mode: 'hash',
			directives: {
				'script-src': ['self']
			}
		},
		alias: {
			$lib: 'src/lib'
		}
	}
};

export default config;
