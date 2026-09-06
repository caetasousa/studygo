/**
 * A pergunta "tem certeza?", no lugar do confirm() do navegador.
 *
 * O confirm() nativo não tem o estilo do app, não dá para nomear o botão pelo
 * que ele faz ("Excluir", "Importar") e trava a página inteira. Aqui o pedido é
 * um estado só, e o <Confirmacao /> montado no layout o desenha.
 *
 * A promessa é o que mantém as chamadas curtas onde elas já estavam:
 *
 *     if (!(await confirmar({ titulo: '…', texto: '…' }))) return;
 */
export interface PedidoDeConfirmacao {
	titulo: string;
	texto: string;
	/** Nome do botão que confirma. Diga o que ele faz, não "OK". */
	rotulo?: string;
	/** `perigo` para o que apaga dado. */
	tom?: 'normal' | 'perigo';
}

class Confirmacao {
	pedido = $state<PedidoDeConfirmacao | null>(null);

	#responder: ((ok: boolean) => void) | null = null;

	perguntar(p: PedidoDeConfirmacao): Promise<boolean> {
		// Uma pergunta nova cancela a anterior: quem esperava resposta recebe não,
		// em vez de ficar pendurado para sempre.
		this.#responder?.(false);

		this.pedido = p;

		return new Promise<boolean>((resolve) => {
			this.#responder = resolve;
		});
	}

	responder(ok: boolean) {
		const responder = this.#responder;

		this.pedido = null;
		this.#responder = null;
		responder?.(ok);
	}
}

export const confirmacao = new Confirmacao();

/** Pergunta e devolve true quando o usuário confirma. */
export function confirmar(p: PedidoDeConfirmacao): Promise<boolean> {
	return confirmacao.perguntar(p);
}
