-- Weekly review as an opt-in, not a rule.
--
-- The plan reserved one whole day of every week for review, taken from the
-- content days. But review is not an event on a fixed day: it is a slice of
-- EVERY day (see the plan's review tail), drilling the error notebook of the
-- subjects that day covers. Surrendering a whole day to it cost a day of
-- content each week for something the daily tail already does.
--
-- Existing plans keep the day they were built with, so nobody's schedule
-- silently reshuffles under them; new plans default to off.
ALTER TABLE planos
    ADD COLUMN revisao_semanal boolean NOT NULL DEFAULT true;

ALTER TABLE planos
    ALTER COLUMN revisao_semanal SET DEFAULT false;
