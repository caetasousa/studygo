-- Per-discipline completion. Until now `concluido` lived only on registros_dia,
-- so ticking a day marked every subject in it at once and there was no way to
-- say "I finished Português today, Direito is still open".
--
-- The day-level flag stays: it is what the engine and the stats read, and it is
-- still the right unit for the special days (simulado, revisão, véspera), which
-- have no disciplines to split by.
ALTER TABLE registros_bloco
    ADD COLUMN concluido boolean NOT NULL DEFAULT false;
