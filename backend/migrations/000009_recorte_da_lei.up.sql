-- O recorte do edital: a parte da lei que a matéria cobra. São as refs das
-- raízes ("tit3.cap7", "art37"); tudo abaixo delas está no recorte. Vazio é a
-- lei inteira — é o que os vínculos que já existem continuam sendo.
ALTER TABLE disciplinas_leis ADD COLUMN recorte text[] NOT NULL DEFAULT '{}';
