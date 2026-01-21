CREATE TABLE IF NOT EXISTS llm_providers (
    id varchar(36) PRIMARY KEY,
    user_id varchar(255),
    name varchar(255),
    provider_type varchar(50),
    base_url varchar(500),
    api_key text,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_llm_providers_user_id ON llm_providers (user_id);
CREATE INDEX IF NOT EXISTS idx_llm_providers_provider_type ON llm_providers (provider_type);
CREATE INDEX IF NOT EXISTS idx_llm_providers_deleted_at ON llm_providers (deleted_at);

CREATE TABLE IF NOT EXISTS llm_models (
    id varchar(36) PRIMARY KEY,
    provider_id varchar(36),
    user_id varchar(255),
    name varchar(255),
    model varchar(255),
    is_default boolean DEFAULT false,
    enable_kv_cache boolean DEFAULT true,
    options jsonb,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    CONSTRAINT fk_llm_models_provider FOREIGN KEY (provider_id) REFERENCES llm_providers(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_llm_models_provider_id ON llm_models (provider_id);
CREATE INDEX IF NOT EXISTS idx_llm_models_user_id ON llm_models (user_id);
CREATE INDEX IF NOT EXISTS idx_llm_models_deleted_at ON llm_models (deleted_at);

CREATE TABLE IF NOT EXISTS llm_calls (
    id bigserial PRIMARY KEY,
    session_id varchar(36),
    user_id varchar(255),
    provider_id varchar(36),
    model_id varchar(36),
    model_name varchar(255),
    messages text,
    response text,
    error text,
    prompt_tokens integer,
    completion_tokens integer,
    total_tokens integer,
    created_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_llm_calls_session_id ON llm_calls (session_id);
CREATE INDEX IF NOT EXISTS idx_llm_calls_user_id ON llm_calls (user_id);
CREATE INDEX IF NOT EXISTS idx_llm_calls_provider_id ON llm_calls (provider_id);
CREATE INDEX IF NOT EXISTS idx_llm_calls_model_id ON llm_calls (model_id);
