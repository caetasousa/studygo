-- Dark is the app's default look. Only the column default changes: plans that
-- already carry a choice (including an explicit 'system') keep it.
ALTER TABLE planos ALTER COLUMN tema_ui SET DEFAULT 'dark';
