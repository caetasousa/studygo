<script lang="ts">
	import PageHead from '$lib/components/PageHead.svelte';
	import { planoStore } from '$lib/stores/plano.svelte';
	import { concursoStore } from '$lib/stores/concurso.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { DIAS_CURTOS, DIAS_SEMANA, ORDEM_DIAS } from '$lib/format';

	const plano = $derived(planoStore.plano);
	const cfg = $derived(plano?.config);

	function salvar(patch: Parameters<typeof planoStore.salvarConfig>[0]) {
		void planoStore.salvarConfig(patch);
	}

	function toggleDia(wd: number) {
		if (!cfg) return;
		const atual = cfg.diasEstudo;
		let novo: number[];
		if (atual.includes(wd)) {
			if (atual.length <= 2) return;
			novo = atual.filter((d) => d !== wd);
		} else {
			novo = [...atual, wd];
		}
		salvar({ diasEstudo: novo });
	}

	let baixando = $state(false);
	async function baixarCsv() {
		baixando = true;
		try {
			const res = await fetch(planoStore.csvUrl(), {
				headers: { Authorization: `Bearer ${auth.accessToken}` }
			});
			const blob = await res.blob();
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = `plano-${concursoStore.ativoSlug ?? 'concurso'}.csv`;
			a.click();
			URL.revokeObjectURL(url);
		} finally {
			baixando = false;
		}
	}

	async function limpar() {
		if (!confirm('Isso apaga todas as horas, questões, anotações de dias e marcações de prazo. Continuar?'))
			return;
		await planoStore.limparRegistros();
	}

	async function restaurar() {
		if (!confirm('Isso desfaz as trocas manuais de matérias e volta à ordem calculada pelo peso. Continuar?'))
			return;
		await planoStore.restaurarOrdem();
	}
</script>

<PageHead
	emoji="⚙️"
	titulo="Configurações"
	sub="Datas, ritmo de estudo e seus dados."
	mostrarProps={false}
/>

{#if plano && cfg}
	<div class="page">
		<div class="card">
			<div class="card-body">
				<h2 class="sec" style="margin-top:0">Datas e ritmo</h2>
				<div class="form-grid">
					<div class="field">
						<label for="inicio">Início</label>
						<input
							id="inicio"
							type="date"
							value={cfg.inicio}
							onchange={(e) => salvar({ inicio: (e.target as HTMLInputElement).value })}
						/>
					</div>
					<div class="field">
						<label for="prova">Dia da prova</label>
						<input
							id="prova"
							type="date"
							value={cfg.prova}
							onchange={(e) => salvar({ prova: (e.target as HTMLInputElement).value })}
						/>
					</div>
					<div class="field">
						<label for="horas">Horas por dia</label>
						<input
							id="horas"
							type="number"
							min="0.5"
							max="14"
							step="0.5"
							value={cfg.horasDia}
							onchange={(e) => salvar({ horasDia: parseFloat((e.target as HTMLInputElement).value) })}
						/>
					</div>
					<div class="field">
						<!-- svelte-ignore a11y_label_has_associated_control -->
						<label>Dias de estudo</label>
						<div class="day-sel">
							{#each ORDEM_DIAS as wd (wd)}
								<button
									type="button"
									class:rev={wd === cfg.diaRevisao}
									aria-pressed={cfg.diasEstudo.includes(wd)}
									title={DIAS_SEMANA[wd]}
									onclick={() => toggleDia(wd)}
								>
									{DIAS_CURTOS[wd]}
								</button>
							{/each}
						</div>
					</div>
					<div class="field">
						<label for="diarev">Dia da revisão</label>
						<select
							id="diarev"
							value={cfg.diaRevisao}
							onchange={(e) => salvar({ diaRevisao: parseInt((e.target as HTMLSelectElement).value, 10) })}
						>
							<option value={1}>segunda</option>
							<option value={2}>terça</option>
							<option value={3}>quarta</option>
							<option value={4}>quinta</option>
							<option value={5}>sexta</option>
							<option value={6}>sábado</option>
							<option value={0}>domingo</option>
						</select>
					</div>
					<div class="field">
						<label for="reta">Reta final (dias)</label>
						<input
							id="reta"
							type="number"
							min="7"
							max="120"
							step="7"
							value={cfg.retaFinalDias}
							onchange={(e) =>
								salvar({ retaFinalDias: parseInt((e.target as HTMLInputElement).value, 10) })}
						/>
					</div>
				</div>

				<h2 class="sec">Reordenação manual</h2>
				<p class="page-sub" style="margin-top:0">
					No Cronograma, use as setas ◀ ▶ de cada dia — ou arraste o bloco da matéria — para trocá-lo
					de lugar. Dias concluídos e dias fixos não podem ser movidos.
				</p>
				<div class="form-grid">
					<button class="btn" disabled={plano.reordenados.length === 0} onclick={restaurar}>
						↺ Restaurar ordem automática
					</button>
					<span class="page-sub" style="margin:0">
						{plano.reordenados.length
							? `${plano.reordenados.length} dia(s) reorganizado(s) manualmente.`
							: 'Nenhuma troca manual ainda.'}
					</span>
				</div>

				<h2 class="sec">Este concurso</h2>
				<div class="form-grid">
					<a class="btn" href="/concursos/{plano.concurso.slug}/editar">✏️ Editar disciplinas e datas</a>
					<a class="btn" href="/concursos">📁 Trocar de concurso</a>
				</div>

				<h2 class="sec">Dados</h2>
				<div class="form-grid">
					<button class="btn" onclick={baixarCsv} disabled={baixando}>⬇ Exportar CSV</button>
					<button class="btn danger" onclick={limpar}>Limpar registros</button>
				</div>
				<p class="page-sub" style="margin-top:14px">
					Seus dados ficam salvos no servidor, ligados à sua conta ({auth.usuario?.email}). O CSV é uma
					cópia de segurança que você pode abrir no Excel.
				</p>
			</div>
		</div>
	</div>
{/if}
