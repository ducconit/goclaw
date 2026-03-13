-- New columns on team_tasks
ALTER TABLE team_tasks ADD COLUMN task_type VARCHAR(30) NOT NULL DEFAULT 'general';
ALTER TABLE team_tasks ADD COLUMN task_number INT NOT NULL DEFAULT 0;
ALTER TABLE team_tasks ADD COLUMN identifier VARCHAR(20);
ALTER TABLE team_tasks ADD COLUMN created_by_agent_id UUID REFERENCES agents(id);
ALTER TABLE team_tasks ADD COLUMN assignee_user_id TEXT;
ALTER TABLE team_tasks ADD COLUMN parent_id UUID REFERENCES team_tasks(id) ON DELETE SET NULL;
ALTER TABLE team_tasks ADD COLUMN chat_id VARCHAR(255) DEFAULT '';
ALTER TABLE team_tasks ADD COLUMN locked_at TIMESTAMPTZ;
ALTER TABLE team_tasks ADD COLUMN lock_expires_at TIMESTAMPTZ;
ALTER TABLE team_tasks ADD COLUMN progress_percent INT DEFAULT 0 CHECK (progress_percent BETWEEN 0 AND 100);
ALTER TABLE team_tasks ADD COLUMN progress_step TEXT;

-- Indexes
CREATE INDEX idx_tt_parent ON team_tasks(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_tt_scope ON team_tasks(team_id, channel, chat_id);
CREATE INDEX idx_tt_type ON team_tasks(team_id, task_type);
CREATE INDEX idx_tt_lock ON team_tasks(lock_expires_at) WHERE lock_expires_at IS NOT NULL AND status = 'in_progress';
CREATE UNIQUE INDEX idx_tt_identifier ON team_tasks(team_id, identifier) WHERE identifier IS NOT NULL;

-- Task comments
CREATE TABLE team_task_comments (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    task_id    UUID NOT NULL REFERENCES team_tasks(id) ON DELETE CASCADE,
    agent_id   UUID REFERENCES agents(id),
    user_id    TEXT,
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ttc_task ON team_task_comments(task_id);

-- Audit history
CREATE TABLE team_task_events (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    task_id    UUID NOT NULL REFERENCES team_tasks(id) ON DELETE CASCADE,
    event_type VARCHAR(30) NOT NULL,
    actor_type VARCHAR(10) NOT NULL,
    actor_id   TEXT NOT NULL,
    data       JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tte_task ON team_task_events(task_id);

-- Task-workspace attachments
CREATE TABLE team_task_attachments (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    task_id    UUID NOT NULL REFERENCES team_tasks(id) ON DELETE CASCADE,
    file_id    UUID NOT NULL REFERENCES team_workspace_files(id) ON DELETE CASCADE,
    added_by   UUID REFERENCES agents(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(task_id, file_id)
);
CREATE INDEX idx_tta_task ON team_task_attachments(task_id);

-- Backfill task_number (per-team sequential) and identifiers for existing tasks
DO $$
DECLARE
    r RECORD;
    seq INT;
    prev_team UUID := '00000000-0000-0000-0000-000000000000';
BEGIN
    FOR r IN
        SELECT t.id, t.team_id,
               UPPER(LEFT(COALESCE(tm.name, 'TSK'), 3)) AS team_prefix
        FROM team_tasks t
        JOIN agent_teams tm ON tm.id = t.team_id
        WHERE t.identifier IS NULL
        ORDER BY t.team_id, t.created_at
    LOOP
        IF r.team_id != prev_team THEN
            seq := 0;
            prev_team := r.team_id;
        END IF;
        seq := seq + 1;
        UPDATE team_tasks SET task_number = seq, identifier = r.team_prefix || '-' || seq WHERE id = r.id;
    END LOOP;
END $$;
