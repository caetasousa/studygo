-- Irreversible by design: the positional codigos carried no information, so
-- there is nothing to restore them from. Rolling back leaves the mnemonics in
-- place, which the application reads exactly the same way.
SELECT 1;
