import { readFileSync } from "node:fs";

// Path to the exported artifact HTML. Second arg "concurso" dumps the catalogue
// fixture instead of the golden plan.
const ART = process.argv[2] ?? process.env.ARTIFACT_HTML;
if (!ART) {
  console.error("usage: node gen_golden.mjs <artifact.html> [concurso]");
  process.exit(1);
}
const lines = readFileSync(ART, "utf8").split("\n");

// 1-indexed slices, matching grep output above.
const constants = lines.slice(437 - 1, 614 - 1).join("\n"); // const D ... PROG]
const helpers = lines.slice(617 - 1, 622 - 1).join("\n"); // iso, hojeISO, dObj, addD, difD
const engine = lines.slice(648 - 1, 769).join("\n"); // reparte ... end of construir

const harness = `
let estado = { cfg:{ inicio:"2026-09-01", prova:"2027-01-17", horas:2, dias:[1,2,3,4,5], diaRev:5, reta:28, q:{} }, overrides:{} };
for(const k in D) estado.cfg.q[k] = D[k].q;
let PLANO=[], SLOTS={}, SLOTS_R={}, PONTOS={}, SOMA_PTS=0;
${engine}
construir();
const dump = {
  somaPontos: SOMA_PTS,
  pontos: PONTOS,
  slots: SLOTS,
  slotsReta: SLOTS_R,
  dias: PLANO.map(d => ({
    n: d.n, data: d.dt, semana: d.sem, fase: d.fase, tipo: d.tipo,
    tema: d.tema || "", meta: d.meta,
    itens: (d.itens || []).map(it => ({ disciplina: it.disc, tema: it.tema, passada: it.pass })),
  })),
};
const concurso = {
  disciplinas: Object.keys(D).map((k, i) => ({
    codigo: k, nome: D[k].n, bloco: D[k].b,
    peso: D[k].b === "esp" ? 2 : 1, questoesPadrao: D[k].q, ordem: i,
    temas: TEMAS[k],
  })),
  revCiclo: REV_CICLO.map((r, i) => ({ ordem: i, titulo: r.t, questoes: r.q })),
};
globalThis.__out = { dump, concurso };
`;

const fn = new Function(constants + "\n" + helpers + "\n" + harness);
fn();
const { dump, concurso } = globalThis.__out;
const target = process.argv[3];
if (target === "concurso") {
  process.stdout.write(JSON.stringify(concurso, null, 2));
} else {
  process.stdout.write(JSON.stringify(dump, null, 2));
}
