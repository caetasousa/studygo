/**
 * Salva um arquivo que veio da API, como o navegador faria com um link: o link
 * direto não serve, porque a API exige o token, que só a página tem.
 */
export function salvarArquivo(blob: Blob, nome: string) {
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = nome;
	a.click();
	URL.revokeObjectURL(url);
}
