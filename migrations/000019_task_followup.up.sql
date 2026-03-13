ALTER TABLE team_tasks ADD COLUMN followup_at       TIMESTAMPTZ;
ALTER TABLE team_tasks ADD COLUMN followup_count    INT NOT NULL DEFAULT 0;
ALTER TABLE team_tasks ADD COLUMN followup_max      INT NOT NULL DEFAULT 0;
ALTER TABLE team_tasks ADD COLUMN followup_message  TEXT;
ALTER TABLE team_tasks ADD COLUMN followup_channel  VARCHAR(60);
ALTER TABLE team_tasks ADD COLUMN followup_chat_id  VARCHAR(255);

CREATE INDEX idx_tt_followup ON team_tasks(followup_at)
  WHERE followup_at IS NOT NULL AND status = 'in_progress';
