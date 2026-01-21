CREATE TABLE IF NOT EXISTS users (
    id varchar(255) PRIMARY KEY,
    email varchar(255),
    username varchar(255),
    name varchar(255),
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

CREATE TABLE IF NOT EXISTS chat_sessions (
    id varchar(36) PRIMARY KEY,
    user_id varchar(255),
    title varchar(500),
    module varchar(50) DEFAULT 'assistant',
    metadata jsonb,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_id ON chat_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_module ON chat_sessions (module);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_deleted_at ON chat_sessions (deleted_at);

CREATE TABLE IF NOT EXISTS chat_messages (
    id bigserial PRIMARY KEY,
    session_id varchar(36),
    parent_id bigint,
    role varchar(20),
    type varchar(20) DEFAULT 'text',
    content text,
    trace jsonb,
    created_at timestamptz,
    deleted_at timestamptz,
    CONSTRAINT fk_chat_messages_session FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE,
    CONSTRAINT fk_chat_messages_parent FOREIGN KEY (parent_id) REFERENCES chat_messages(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages (session_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_parent_id ON chat_messages (parent_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_deleted_at ON chat_messages (deleted_at);
