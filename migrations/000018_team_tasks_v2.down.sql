DROP TABLE IF EXISTS team_task_attachments;
DROP TABLE IF EXISTS team_task_events;
DROP TABLE IF EXISTS team_task_comments;

DROP INDEX IF EXISTS idx_tt_parent;
DROP INDEX IF EXISTS idx_tt_scope;
DROP INDEX IF EXISTS idx_tt_type;
DROP INDEX IF EXISTS idx_tt_lock;
DROP INDEX IF EXISTS idx_tt_identifier;

ALTER TABLE team_tasks DROP COLUMN IF EXISTS task_type;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS task_number;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS identifier;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS created_by_agent_id;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS assignee_user_id;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS parent_id;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS chat_id;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS locked_at;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS lock_expires_at;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS progress_percent;
ALTER TABLE team_tasks DROP COLUMN IF EXISTS progress_step;
