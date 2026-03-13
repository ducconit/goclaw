DROP INDEX IF EXISTS idx_tt_followup;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS followup_chat_id;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS followup_channel;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS followup_message;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS followup_max;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS followup_count;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS followup_at;
