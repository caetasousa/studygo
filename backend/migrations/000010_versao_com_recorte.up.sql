-- A versão da lei pode guardar só a parte que o edital pede, e não a lei
-- inteira: são as refs das raízes que ela contém ("tit3.cap7", "art37"). Vazio
-- é a lei inteira — o que as versões que já existem continuam sendo. Ampliar
-- a lei (outro tópico pede mais artigos) é uma versão nova com a união.
ALTER TABLE leis_versoes ADD COLUMN recorte text[] NOT NULL DEFAULT '{}';
