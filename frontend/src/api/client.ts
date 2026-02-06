import { ofetch } from "ofetch";
import { useAuthStore } from "@/stores/auth";

const API_BASE = import.meta.env.VITE_API_BASE || "";

// Create base fetch instance
export const api = ofetch.create({
  baseURL: API_BASE,
  onRequest({ options }) {
    const authStore = useAuthStore();
    if (authStore.token) {
      options.headers = new Headers(options.headers);
      options.headers.set("Authorization", `Bearer ${authStore.token}`);
    }
  },
  onResponseError({ response }) {
    if (response.status === 401) {
      const authStore = useAuthStore();
      authStore.clearAuth();
      window.location.href = "/login";
    }
  },
});

// Stream event interface
export interface StreamEvent {
  type:
    | "session"
    | "content"
    | "trace"
    | "done"
    | "error"
    | "usage"
    | "msg"
    | "canceled";
  data: string;
}

function concatBytes(a: Uint8Array, b: Uint8Array): Uint8Array {
  if (a.length === 0) return b;
  if (b.length === 0) return a;
  const out = new Uint8Array(a.length + b.length);
  out.set(a, 0);
  out.set(b, a.length);
  return out;
}

function utf8ExpectedLength(firstByte: number): number {
  if ((firstByte & 0x80) === 0) return 1;
  if ((firstByte & 0xe0) === 0xc0) return 2;
  if ((firstByte & 0xf0) === 0xe0) return 3;
  if ((firstByte & 0xf8) === 0xf0) return 4;
  return 0;
}

function splitIncompleteUtf8Tail(bytes: Uint8Array): {
  complete: Uint8Array;
  remainder: Uint8Array;
} {
  if (bytes.length === 0) {
    return { complete: bytes, remainder: bytes };
  }

  const last = bytes[bytes.length - 1];
  if (last < 0x80) {
    return { complete: bytes, remainder: new Uint8Array() };
  }

  const lookback = Math.min(4, bytes.length);
  for (let i = 1; i <= lookback; i++) {
    const start = bytes.length - i;
    const b = bytes[start];

    // Continuation bytes are 10xxxxxx; skip them.
    if ((b & 0xc0) === 0x80) continue;

    const expected = utf8ExpectedLength(b);
    if (expected === 0 || expected === 1) {
      return { complete: bytes, remainder: new Uint8Array() };
    }

    const available = bytes.length - start;
    if (available >= expected) {
      return { complete: bytes, remainder: new Uint8Array() };
    }

    return {
      complete: bytes.slice(0, start),
      remainder: bytes.slice(start),
    };
  }

  return { complete: bytes, remainder: new Uint8Array() };
}

/**
 * Stream chat messages from the API
 */
async function consumeSSE(
  response: Response,
  onEvent: (event: StreamEvent) => void,
): Promise<void> {
  const reader = response.body?.getReader();
  if (!reader) {
    throw new Error("No response body");
  }

  const decoder = new TextDecoder();
  let pendingBytes: Uint8Array<ArrayBufferLike> = new Uint8Array();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    const combined = concatBytes(pendingBytes, value);
    const { complete, remainder } = splitIncompleteUtf8Tail(combined);
    pendingBytes = remainder;

    buffer += decoder.decode(complete);
    // Normalize CRLF -> LF so we can reliably split SSE events.
    buffer = buffer.replace(/\r\n/g, "\n");

    // Process complete events
    let delimiterIndex = buffer.indexOf("\n\n");
    while (delimiterIndex !== -1) {
      const rawEvent = buffer.slice(0, delimiterIndex);
      buffer = buffer.slice(delimiterIndex + 2);

      const dataLines = rawEvent
        .split("\n")
        .filter((l) => l.startsWith("data:"))
        .map((l) => l.replace(/^data:\s?/, ""));

      if (dataLines.length > 0) {
        const dataText = dataLines.join("\n");
        try {
          const data = JSON.parse(dataText);
          onEvent(data as StreamEvent);
        } catch (e) {
          console.error("Failed to parse SSE event:", e);
        }
      }

      delimiterIndex = buffer.indexOf("\n\n");
    }
  }

  if (pendingBytes.length > 0) {
    buffer += decoder.decode(pendingBytes);
  }

  // Process any remaining buffer
  buffer = buffer.replace(/\r\n/g, "\n");
  const delimiterIndex = buffer.indexOf("\n\n");
  const lastEvent = (
    delimiterIndex === -1 ? buffer : buffer.slice(0, delimiterIndex)
  ).trim();
  if (lastEvent) {
    const dataLines = lastEvent
      .split("\n")
      .filter((l) => l.startsWith("data:"))
      .map((l) => l.replace(/^data:\s?/, ""));
    if (dataLines.length > 0) {
      try {
        const data = JSON.parse(dataLines.join("\n"));
        onEvent(data as StreamEvent);
      } catch {
        // Ignore incomplete / non-JSON tail.
      }
    }
  }
}

export async function streamChat(
  message: string,
  sessionId: string = "",
  modelId: string = "",
  toolIds: string[] = [],
  toolProtocol: string = "json",
  workspace: string = "",
  onEvent: (event: StreamEvent) => void,
  onError: (error: Error) => void,
  signal?: AbortSignal,
): Promise<void> {
  const authStore = useAuthStore();

  try {
    const response = await fetch(`${API_BASE}/api/chat`, {
      method: "POST",
      signal,
      headers: {
        "Content-Type": "application/json",
        Accept: "text/event-stream",
        Authorization: `Bearer ${authStore.token}`,
      },
      body: JSON.stringify({
        message,
        session_id: sessionId,
        model_id: modelId,
        tool_ids: toolIds,
        tool_protocol: toolProtocol,
        workspace,
      }),
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    await consumeSSE(response, onEvent);
  } catch (error) {
    const e = error as any;
    if (e?.name === "AbortError") return;
    onError(error as Error);
  }
}

/**
 * Attach to an in-flight stream for an existing session (best-effort).
 */
export async function attachChatStream(
  sessionId: string,
  onEvent: (event: StreamEvent) => void,
  onError: (error: Error) => void,
  signal?: AbortSignal,
): Promise<void> {
  const authStore = useAuthStore();
  const id = String(sessionId || "").trim();
  if (!id) {
    onError(new Error("sessionId is required"));
    return;
  }

  try {
    const response = await fetch(
      `${API_BASE}/api/sessions/${encodeURIComponent(id)}/stream`,
      {
        method: "GET",
        signal,
        headers: {
          Accept: "text/event-stream",
          Authorization: `Bearer ${authStore.token}`,
        },
      },
    );

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    await consumeSSE(response, onEvent);
  } catch (error) {
    const e = error as any;
    if (e?.name === "AbortError") return;
    onError(error as Error);
  }
}

export async function stopSessionStream(sessionId: string) {
  const id = String(sessionId || "").trim();
  if (!id) throw new Error("sessionId is required");
  return api(`/api/sessions/${encodeURIComponent(id)}/stop`, { method: "POST" });
}

/**
 * Get user info
 */
export async function getUserInfo() {
  return api("/api/me");
}

/**
 * Get chat sessions
 */
export async function getSessions() {
  return api("/api/sessions");
}

/**
 * Get a specific session with messages
 */
export async function getSession(sessionId: string) {
  return api(`/api/sessions/${sessionId}`);
}

/**
 * Delete a session
 */
export async function deleteSession(sessionId: string) {
  return api(`/api/sessions/${sessionId}`, { method: "DELETE" });
}

/**
 * Truncate a session's messages from a given message id (inclusive).
 * Used for "retry/regenerate" flows.
 */
export async function truncateSession(
  sessionId: string,
  fromMessageId: number,
) {
  return api(`/api/sessions/${sessionId}/truncate`, {
    method: "POST",
    body: { from_message_id: fromMessageId },
  });
}

// ============================================================================
// Secretary (WeChat-style inbox + triage)
// ============================================================================

export interface SecretaryInboxAppendResponse {
  session_id: string;
  message_id: number;
  ack_message_id: number;
  ack_text: string;
}

export async function appendSecretaryInboxMessage(payload: {
  content: string;
  workspace?: string;
}): Promise<SecretaryInboxAppendResponse> {
  return api("/api/secretary/inbox/messages", { method: "POST", body: payload });
}

export interface SecretaryTriageResponse {
  session_id: string;
  summary_message: string;
  summary_message_id: number;
  cursor_message_id: number;
  created_task_ids: string[];
  questions?: string[];
  workspaces_created?: string[];
}

export async function secretaryTriage(payload: {
  cursor_message_id?: number;
}): Promise<SecretaryTriageResponse> {
  return api("/api/secretary/triage", { method: "POST", body: payload });
}

export interface SecretaryStateResponse {
  session_id: string;
  cursor_message_id: number;
  recovery_focus?: {
    task_id: string;
    attempt_id: string;
  };
  triage_runs?: Array<{
    from_cursor: number;
    to_message_id: number;
    summary_message_id?: number;
    summary_message?: string;
    created_task_ids?: string[];
    questions?: string[];
    workspaces_created?: string[];
    created_at?: string;
  }>;
}

export async function getSecretaryState(): Promise<SecretaryStateResponse> {
  return api(`/api/secretary/state`);
}

export async function setSecretaryRecoveryFocus(payload: {
  task_id?: string;
  attempt_id?: string;
}): Promise<{ session_id: string; recovery_focus?: { task_id: string; attempt_id: string } }> {
  return api("/api/secretary/recovery/focus", { method: "POST", body: payload });
}

export async function getSecretarySession() {
  return api("/api/secretary/session");
}

export async function approveToolApproval(approvalId: string, reason: string = "") {
  const id = String(approvalId || "").trim();
  if (!id) throw new Error("approvalId is required");
  const body = String(reason || "").trim() ? { reason } : undefined;
  return api(`/api/approvals/${encodeURIComponent(id)}/approve`, {
    method: "POST",
    body,
  });
}

export async function denyToolApproval(approvalId: string, reason: string = "") {
  const id = String(approvalId || "").trim();
  if (!id) throw new Error("approvalId is required");
  const body = String(reason || "").trim() ? { reason } : undefined;
  return api(`/api/approvals/${encodeURIComponent(id)}/deny`, {
    method: "POST",
    body,
  });
}

// ============================================================================
// Command approval settings
// ============================================================================

export type CommandApprovalMode = "auto" | "manual";

export interface CommandApprovalSettings {
  command_approval_mode: CommandApprovalMode;
}

export async function getCommandApprovalSettings(): Promise<CommandApprovalSettings> {
  return api("/api/command_approvals/settings");
}

export async function updateCommandApprovalSettings(
  payload: CommandApprovalSettings,
): Promise<CommandApprovalSettings> {
  return api("/api/command_approvals/settings", { method: "PUT", body: payload });
}

export async function getProviders() {
  return api("/api/llm/providers");
}

export async function createProvider(payload: {
  name: string;
  provider_type: string;
  base_url: string;
  api_key: string;
}) {
  return api("/api/llm/providers", { method: "POST", body: payload });
}

export async function updateProvider(
  providerId: string,
  payload: {
    name?: string;
    provider_type?: string;
    base_url?: string;
    api_key?: string;
  },
) {
  return api(`/api/llm/providers/${providerId}`, {
    method: "PUT",
    body: payload,
  });
}

export async function deleteProvider(providerId: string) {
  return api(`/api/llm/providers/${providerId}`, { method: "DELETE" });
}

export async function getModels(providerId?: string) {
  const query = providerId
    ? `?provider_id=${encodeURIComponent(providerId)}`
    : "";
  return api(`/api/llm/models${query}`);
}

export async function getTools() {
  return api("/api/tools");
}

export interface RuntimeConfig {
  default_workspace?: string;
  base_url?: string;
  warnings?: string[];
}

export async function getConfig(): Promise<RuntimeConfig> {
  return api("/api/config");
}

export async function chooseWorkspaceDir() {
  return api("/api/workspace/choose", { method: "POST" });
}

// ============================================================================
// Document Export
// ============================================================================

export type DocumentExportFormat = "docx" | "pptx";

export interface DocumentExportRequest {
  workspace: string;
  input_path: string;
  format: DocumentExportFormat;
  output_path?: string;
  template_path?: string;
}

export async function exportDocument(payload: DocumentExportRequest) {
  return api("/api/documents/export", { method: "POST", body: payload });
}

export async function createModel(payload: {
  provider_id: string;
  name: string;
  model: string;
  is_default?: boolean;
  enable_kv_cache?: boolean;
}) {
  return api("/api/llm/models", { method: "POST", body: payload });
}

export async function updateModel(
  modelId: string,
  payload: {
    name?: string;
    model?: string;
    is_default?: boolean;
    enable_kv_cache?: boolean;
  },
) {
  return api(`/api/llm/models/${modelId}`, { method: "PUT", body: payload });
}

export async function deleteModel(modelId: string) {
  return api(`/api/llm/models/${modelId}`, { method: "DELETE" });
}

// ============================================================================
// Task Queue
// ============================================================================

export type AttemptStatus =
  | "queued"
  | "running"
  | "succeeded"
  | "failed"
  | "limit_exceeded"
  | "canceled"
  | "timed_out"
  | "interrupted";

export interface TaskLimits {
  max_steps?: number;
  max_runtime_seconds?: number;
  max_total_tokens?: number;
  max_cost_usd?: number;
  max_auto_attempts?: number;
}

export interface TaskObserverDecision {
  pass: boolean;
  reason?: string;
  evidence?: string[];
  next_steps?: string;
  questions_for_user?: string[];
}

export type ToolPolicyEffect = "allow" | "deny";

export interface ToolPolicyConstraints {
  file_scope?: string[];
  read_outside_workspace?: boolean;
  command_profile?: string;
  allowlist?: string[];
}

export interface ToolPolicyRule {
  id: string;
  effect: ToolPolicyEffect;
  tool_id: string;
  constraints?: ToolPolicyConstraints;
}

export interface ToolPolicy {
  id: string;
  default_effect?: ToolPolicyEffect;
  default_command_profile?: string;
  rules?: ToolPolicyRule[];
}

export interface ToolPolicySnapshot {
  policy: ToolPolicy;
  policy_hash: string;
  resolved_at: string;
  principal_id: string;
}

export interface TaskAttempt {
  id: string;
  status: AttemptStatus;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  resumed_from_attempt_id?: string;
  auto?: boolean;
  principal_id?: string;
  policy_snapshot?: ToolPolicySnapshot;
  run_id?: string;
  summary?: string;
  artifact_manifest_version?: string;
  artifact_manifest_path?: string;
  findings_path?: string;
  trace_log_path?: string;
  test_report_path?: string;
  diff_patch_path?: string;
  changed_files_path?: string;
  review_comments_path?: string;
  review_notes?: string;
  project_config_path?: string;
  worktree_root?: string;
  base_commit_sha?: string;
  base_ref?: string;
  copy_files_log_path?: string;
  setup_script_log_path?: string;
  test_script_log_path?: string;
  cleanup_script_log_path?: string;
  observer?: TaskObserverDecision;
  usage?: {
    calls?: number;
    prompt_tokens?: number;
    completion_tokens?: number;
    total_tokens?: number;
    cost_usd?: number;
  };
  error?: string;
}

export interface Task {
  id: string;
  user_id: string;
  workspace: string;
  title: string;
  prompt: string;
  model_id?: string;
  limits?: TaskLimits;
  created_at: string;
  updated_at: string;
  attempts: TaskAttempt[];
}

export interface TaskEvent {
  ts: string;
  task_id: string;
  attempt_id?: string;
  type: string;
  message?: string;
  data?: Record<string, any>;
}

export async function createTask(payload: {
  workspace: string;
  title?: string;
  prompt: string;
  model_id?: string;
  limits?: TaskLimits;
}): Promise<Task> {
  return api("/api/tasks", { method: "POST", body: payload });
}

export async function listTasks(workspace?: string): Promise<Task[]> {
  const query = workspace ? `?workspace=${encodeURIComponent(workspace)}` : "";
  return api(`/api/tasks${query}`);
}

export async function getTask(taskId: string): Promise<Task> {
  return api(`/api/tasks/${taskId}`);
}

export async function cancelTask(taskId: string): Promise<Task> {
  return api(`/api/tasks/${taskId}/cancel`, { method: "POST" });
}

export async function resumeTask(
  taskId: string,
  payload?: { review_notes?: string; source?: string },
): Promise<Task> {
  return api(`/api/tasks/${taskId}/resume`, { method: "POST", body: payload });
}

export async function getTaskEvents(taskId: string): Promise<TaskEvent[]> {
  return api(`/api/tasks/${taskId}/events`);
}

export interface TaskAttemptArtifactContent {
  path: string;
  content: string;
  truncated: boolean;
}

export async function getTaskAttemptArtifact(
  taskId: string,
  attemptId: string,
  kind: string,
): Promise<TaskAttemptArtifactContent> {
  return api(
    `/api/tasks/${encodeURIComponent(taskId)}/attempts/${encodeURIComponent(attemptId)}/artifacts/${encodeURIComponent(kind)}`,
  );
}

export async function getWorkflowNodeArtifact(
  workflowId: string,
  runId: string,
  nodeId: string,
  kind: string,
  workspace: string,
): Promise<TaskAttemptArtifactContent> {
  const qs = new URLSearchParams({ workspace });
  return api(
    `/api/workflows/${encodeURIComponent(workflowId)}/runs/${encodeURIComponent(runId)}/nodes/${encodeURIComponent(nodeId)}/artifacts/${encodeURIComponent(kind)}?${qs.toString()}`,
  );
}

export async function getSubagentRunArtifact(
  sessionId: string,
  runId: string,
  kind: string,
  opts?: { tail?: boolean },
): Promise<TaskAttemptArtifactContent> {
  const q = opts?.tail ? "?tail=1" : "";
  return api(
    `/api/subagent/sessions/${encodeURIComponent(sessionId)}/runs/${encodeURIComponent(runId)}/artifacts/${encodeURIComponent(kind)}${q}`,
  );
}

export async function getTaskAttemptDiffPatch(
  taskId: string,
  attemptId: string,
): Promise<TaskAttemptArtifactContent> {
  return api(
    `/api/tasks/${encodeURIComponent(taskId)}/attempts/${encodeURIComponent(attemptId)}/diff_patch`,
  );
}

export async function getTaskAttemptChangedFiles(
  taskId: string,
  attemptId: string,
): Promise<TaskAttemptArtifactContent> {
  return api(
    `/api/tasks/${encodeURIComponent(taskId)}/attempts/${encodeURIComponent(attemptId)}/changed_files`,
  );
}

export interface TaskAttemptReviewComment {
  ts: string;
  principal_id: string;
  task_id: string;
  attempt_id: string;
  comment: string;
}

export async function listTaskAttemptReviewComments(
  taskId: string,
  attemptId: string,
): Promise<TaskAttemptReviewComment[]> {
  return api(
    `/api/tasks/${encodeURIComponent(taskId)}/attempts/${encodeURIComponent(attemptId)}/review_comments`,
  );
}

export async function postTaskAttemptReviewComment(
  taskId: string,
  attemptId: string,
  payload: { comment: string },
): Promise<TaskAttemptReviewComment> {
  return api(
    `/api/tasks/${encodeURIComponent(taskId)}/attempts/${encodeURIComponent(attemptId)}/review_comments`,
    { method: "POST", body: payload },
  );
}

// ============================================================================
// Workflows (orchestration graph, workspace-scoped)
// ============================================================================

export interface Workflow {
  workflow_id: string;
  workspace_root: string;
  name: string;
  created_at: string;
  updated_at: string;
}

export interface WorkflowGraphNode {
  node_id: string;
  title?: string;
  prompt?: string;
  principal_id?: string;
  model_id?: string;
  skills?: string[];
}

export interface WorkflowGraphEdge {
  from: string;
  to: string;
}

export interface WorkflowGraph {
  nodes: WorkflowGraphNode[];
  edges: WorkflowGraphEdge[];
}

export interface WorkflowVersion {
  version_id: string;
  workflow_id: string;
  published: boolean;
  published_at?: string;
  graph: WorkflowGraph;
  created_at: string;
}

export interface WorkflowArtifact {
  path: string;
  kind?: string;
}

export interface WorkflowArtifactManifest {
  artifacts: WorkflowArtifact[];
}

export interface WorkflowNodeRun {
  node_id: string;
  status: string;
  started_at?: string;
  finished_at?: string;
  error?: string;
  artifacts?: WorkflowArtifactManifest;
  hard_gate_report_path?: string;
  soft_gate_report_path?: string;
}

export interface WorkflowRun {
  run_id: string;
  workflow_id: string;
  version_id: string;
  workspace_root: string;
  graph_snapshot: WorkflowGraph;
  inputs?: Record<string, any>;
  node_runs?: Record<string, WorkflowNodeRun>;
  status: string;
  started_at?: string;
  finished_at?: string;
  error?: string;
  created_at: string;
  updated_at: string;
}

export async function listWorkflows(workspace: string): Promise<Workflow[]> {
  const qs = new URLSearchParams({ workspace });
  return api(`/api/workflows?${qs.toString()}`);
}

export async function createWorkflow(payload: {
  workspace_root: string;
  name: string;
}): Promise<Workflow> {
  return api("/api/workflows", { method: "POST", body: payload });
}

export async function renameWorkflow(
  workflowId: string,
  payload: { workspace_root: string; name: string },
): Promise<Workflow> {
  return api(`/api/workflows/${encodeURIComponent(workflowId)}`, {
    method: "PATCH",
    body: payload,
  });
}

export async function deleteWorkflow(
  workflowId: string,
  workspace: string,
): Promise<{ ok: boolean }> {
  const qs = new URLSearchParams({ workspace });
  return api(`/api/workflows/${encodeURIComponent(workflowId)}?${qs.toString()}`, {
    method: "DELETE",
  });
}

export async function publishWorkflowVersion(
  workflowId: string,
  payload: { workspace_root: string; graph: WorkflowGraph },
): Promise<WorkflowVersion> {
  return api(`/api/workflows/${encodeURIComponent(workflowId)}/publish`, {
    method: "POST",
    body: payload,
  });
}

export async function createWorkflowRun(
  workflowId: string,
  payload: { workspace_root: string; version_id: string; inputs?: Record<string, any> },
): Promise<WorkflowRun> {
  return api(`/api/workflows/${encodeURIComponent(workflowId)}/runs`, {
    method: "POST",
    body: payload,
  });
}

export async function getWorkflowRun(
  workflowId: string,
  runId: string,
  workspace: string,
): Promise<WorkflowRun> {
  const qs = new URLSearchParams({ workspace });
  return api(
    `/api/workflows/${encodeURIComponent(workflowId)}/runs/${encodeURIComponent(runId)}?${qs.toString()}`,
  );
}

export async function executeWorkflowRun(
  workflowId: string,
  runId: string,
  payload: { workspace_root: string; concurrency?: number },
): Promise<WorkflowRun> {
  return api(
    `/api/workflows/${encodeURIComponent(workflowId)}/runs/${encodeURIComponent(runId)}/execute`,
    { method: "POST", body: payload },
  );
}

export async function cancelWorkflowRun(
  workflowId: string,
  runId: string,
  payload: { workspace_root: string; reason?: string },
): Promise<WorkflowRun> {
  return api(
    `/api/workflows/${encodeURIComponent(workflowId)}/runs/${encodeURIComponent(runId)}/cancel`,
    { method: "POST", body: payload },
  );
}

// ============================================================================
// Tool permissions (admin)
// ============================================================================

export interface AuthTokenResponse {
  token: string;
  principal_id: string;
  created_at: string;
  revoked_at?: string;
}

export interface ToolPolicyResponse {
  principal_id: string;
  exists: boolean;
  policy: ToolPolicy;
  snapshot: ToolPolicySnapshot;
}

export async function createAuthToken(
  principalId: string,
): Promise<AuthTokenResponse> {
  return api("/api/admin/tokens", {
    method: "POST",
    body: { principal_id: principalId },
  });
}

export async function listAuthTokens(): Promise<AuthTokenResponse[]> {
  return api("/api/admin/tokens");
}

export async function revokeAuthToken(token: string): Promise<{ ok: boolean }> {
  return api("/api/admin/tokens/revoke", { method: "POST", body: { token } });
}

export async function getToolPolicy(
  principalId: string,
): Promise<ToolPolicyResponse> {
  return api(`/api/admin/tool_policies/${encodeURIComponent(principalId)}`);
}

export async function setToolPolicy(
  principalId: string,
  policy: ToolPolicy,
): Promise<ToolPolicyResponse> {
  return api(`/api/admin/tool_policies/${encodeURIComponent(principalId)}`, {
    method: "PUT",
    body: policy,
  });
}

// ============================================================================
// Skills (governance)
// ============================================================================

export interface SkillInfo {
  skill_id: string;
  name: string;
  description: string;
  source: string;
  path: string;
  archivable: boolean;
}

export interface SkillCandidateInfo extends SkillInfo {
  effective: boolean;
  precedence_rank: number;
}

export interface SkillDuplicateGroup {
  skill_id: string;
  candidates: SkillCandidateInfo[];
}

export async function listSkills(): Promise<SkillInfo[]> {
  return api("/api/skills");
}

export async function listSkillDuplicates(params?: {
  workspace?: string;
}): Promise<SkillDuplicateGroup[]> {
  const qs = new URLSearchParams();
  if (params?.workspace) qs.set("workspace", params.workspace);
  const q = qs.toString();
  return api(`/api/skills/duplicates${q ? `?${q}` : ""}`);
}

export async function archiveSkill(
  skillId: string,
  payload?: { reason?: string },
): Promise<{ ok: boolean; skill_id: string; archived_path: string; reason?: string }> {
  return api(`/api/skills/${encodeURIComponent(skillId)}/archive`, {
    method: "POST",
    body: payload || {},
  });
}

export interface StaleSkillInfo extends SkillInfo {
  used_count: number;
  last_used_at?: string;
  last_activity_at?: string;
  stale_reason?: string;
  recommended_action?: string;
  stale_threshold_days?: number;
}

export async function listStaleSkills(params?: {
  days?: number;
}): Promise<StaleSkillInfo[]> {
  const qs = new URLSearchParams();
  if (typeof params?.days === "number" && Number.isFinite(params.days)) {
    qs.set("days", String(Math.floor(params.days)));
  }
  const q = qs.toString();
  return api(`/api/skills/stale${q ? `?${q}` : ""}`);
}

export interface PinSkillResult {
  ok: boolean;
  skill_id: string;
  canonical_path: string;
  archived_paths?: string[];
  shadowed_candidates?: SkillCandidateInfo[];
}

export async function pinSkillCandidate(
  skillId: string,
  payload: {
    source: string;
    path: string;
    workspace_root?: string;
    archive_shadowed_personal?: boolean;
  },
): Promise<PinSkillResult> {
  return api(`/api/skills/${encodeURIComponent(skillId)}/pin`, {
    method: "POST",
    body: payload,
  });
}

export async function archiveShadowedPersonalDuplicates(
  skillId: string,
): Promise<{
  ok: boolean;
  skill_id: string;
  archived_paths: string[];
}> {
  return api(`/api/skills/${encodeURIComponent(skillId)}/archive_shadowed`, {
    method: "POST",
    body: {},
  });
}

export async function getSkill(
  skillId: string,
): Promise<
  SkillInfo & { sha256?: string; skill_md?: string; files?: string[] }
> {
  return api(`/api/skills/${encodeURIComponent(skillId)}`);
}

export async function readSkillFile(
  skillId: string,
  path: string,
): Promise<{ path: string; sha256: string; content: string }> {
  const qs = new URLSearchParams();
  if (path) qs.set("path", path);
  const q = qs.toString();
  return api(
    `/api/skills/${encodeURIComponent(skillId)}/file${q ? `?${q}` : ""}`,
  );
}

export async function updateSkill(payload: {
  skill_id: string;
  skill_md: string;
  expected_sha256?: string;
}): Promise<SkillInfo & { sha256?: string; skill_md?: string }> {
  return api(`/api/skills/${encodeURIComponent(payload.skill_id)}`, {
    method: "PUT",
    body: {
      skill_md: payload.skill_md,
      expected_sha256: payload.expected_sha256,
    },
  });
}

// ============================================================================
// Work Ledger (Receipts / Digest)
// ============================================================================

export interface Digest {
  principal_id: string;
  day_key: string;
  generated_at?: string;
  markdown: string;
}

export interface DigestItem {
  receipt_id: string;
  status: string;
  workspace_root?: string;
  kind?: string;
  summary: string;
  finished_at?: string;
  artifacts?: Record<string, any>;
}

export interface DigestCluster {
  key: string;
  count: number;
  receipt_ids: string[];
}

export interface StructuredDigest {
  principal_id: string;
  day_key: string;
  generated_at?: string;
  items: DigestItem[];
  clusters?: DigestCluster[];
}

export async function getTodayDigest(
  refresh: boolean = false,
): Promise<Digest> {
  const q = refresh ? "?refresh=1" : "";
  return api(`/api/ledger/digests/today${q}`);
}

export async function getDigest(
  dayKey: string,
  refresh: boolean = false,
): Promise<Digest> {
  const q = refresh ? "?refresh=1" : "";
  return api(`/api/ledger/digests/${encodeURIComponent(dayKey)}${q}`);
}

export async function getTodayStructuredDigest(): Promise<StructuredDigest> {
  return api(`/api/ledger/digests/today/structured`);
}

export async function getStructuredDigest(
  dayKey: string,
): Promise<StructuredDigest> {
  return api(`/api/ledger/digests/${encodeURIComponent(dayKey)}/structured`);
}

export async function createLedgerFollowUpTask(payload: {
  receipt_ids: string[];
  instruction?: string;
  model_id?: string;
  limits?: Record<string, any>;
}): Promise<any> {
  return api(`/api/ledger/followups`, {
    method: "POST",
    body: payload,
  });
}

export interface LedgerStatusToday {
  day_key: string;
  digest_exists: boolean;
  learning_job_status: string;
  sop_proposed_count: number;
}

export async function getLedgerStatusToday(): Promise<LedgerStatusToday> {
  return api("/api/ledger/status/today");
}

// ============================================================================
// Task queue governance (multi-workspace policies / schedules)
// ============================================================================

export interface TaskQueueGovernance {
  global: {
    max_running_workspaces?: number;
  };
  workspaces?: Record<
    string,
    {
      paused?: boolean;
      priority?: number;
    }
  >;
  schedules?: Array<{
    id: string;
    enabled: boolean;
    user_id?: string;
    workspace: string;
    title?: string;
    prompt: string;
    model_id?: string;
    every_seconds: number;
    next_run_at?: string;
    misfire_policy?: string;
    last_trigger_key?: string;
    last_enqueue_at?: string;
    last_enqueue_error?: string;
    last_enqueue_error_at?: string;
    created_at?: string;
    updated_at?: string;
  }>;
  updated_at?: string;
}

export async function getTaskQueueGovernance(): Promise<TaskQueueGovernance> {
  return api("/api/tasks/governance");
}

export interface TaskQueueGovernanceSnapshotWorkspace {
  queued_tasks?: number;
  running?: boolean;
  paused?: boolean;
  priority?: number;
  age?: number;
  effective_priority?: number;
  decision?: string;
  reason_code?: string;
  selected_task_id?: string;
}

export interface TaskQueueGovernanceSnapshot {
  ts: string;
  global: {
    max_running_workspaces?: number;
  };
  running_workspaces?: string[];
  deferred_workspaces?: number;
  paused_workspaces?: number;
  workspaces?: Record<string, TaskQueueGovernanceSnapshotWorkspace>;
}

export async function getTaskQueueGovernanceSnapshot(): Promise<TaskQueueGovernanceSnapshot> {
  return api("/api/tasks/governance/snapshot");
}

export async function updateTaskQueueGlobalPolicy(payload: {
  max_running_workspaces: number;
}): Promise<TaskQueueGovernance> {
  return api("/api/tasks/governance/global", { method: "POST", body: payload });
}

export async function updateTaskQueueWorkspacePolicy(payload: {
  workspace: string;
  paused?: boolean;
  priority?: number;
}): Promise<TaskQueueGovernance> {
  return api("/api/tasks/governance/workspace", {
    method: "POST",
    body: payload,
  });
}

export async function createTaskQueueSchedule(payload: {
  workspace: string;
  title?: string;
  prompt: string;
  model_id?: string;
  misfire_policy?: string;
  every_seconds: number;
  enabled: boolean;
  limits?: Record<string, any>;
}): Promise<TaskQueueGovernance> {
  return api("/api/tasks/governance/schedules", { method: "POST", body: payload });
}

export type ReceiptKind = "subagent_run";

export type ReceiptStatus =
  | "succeeded"
  | "failed"
  | "canceled"
  | "timed_out"
  | "interrupted";

export type EvidenceCompleteness = "complete" | "partial" | "insufficient";

export interface ReceiptArtifacts {
  findings_path?: string;
  trace_log_path?: string;
  test_report_path?: string;
  diff_patch_path?: string;
  changed_files_path?: string;
  review_comments_path?: string;
  diff_ref?: string;
  worktree_root?: string;
  base_commit_sha?: string;
  base_ref?: string;
}

export interface ReceiptSignals {
  duration_ms?: number;
  calls?: number;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
  cost_usd?: number;
}

export interface Receipt {
  receipt_id: string;
  principal_id: string;
  workspace_root?: string;
  kind: ReceiptKind;
  status: ReceiptStatus;
  started_at: string;
  finished_at: string;
  summary: string;
  artifact_manifest_version?: string;
  artifact_manifest_path?: string;
  evidence_completeness?: EvidenceCompleteness;
  artifacts?: ReceiptArtifacts;
  signals?: ReceiptSignals;
}

export async function listReceipts(params?: {
  workspace?: string;
  status?: ReceiptStatus;
  q?: string;
  limit?: number;
}): Promise<Receipt[]> {
  const qs = new URLSearchParams();
  if (params?.workspace) qs.set("workspace", params.workspace);
  if (params?.status) qs.set("status", params.status);
  if (params?.q) qs.set("q", params.q);
  if (params?.limit) qs.set("limit", String(params.limit));
  const q = qs.toString();
  return api(`/api/ledger/receipts${q ? `?${q}` : ""}`);
}

export async function getReceipt(receiptId: string): Promise<Receipt> {
  return api(`/api/ledger/receipts/${encodeURIComponent(receiptId)}`);
}

export type SuggestionStatus =
  | "proposed"
  | "parked"
  | "approved"
  | "rejected"
  | "merged"
  | "deprecated"
  | "archived";

export interface SuggestionScores {
  scarcity_score: number;
  depth_score: number;
  evidence_score: number;
  total_score: number;
}

export interface SuggestionMeta {
  day_key?: string;
  compression_prompt?: string;
  compression_verdict?: string;
  compression_reason?: string;
  compression_evaluated_at?: string;
  similar_skill_ids?: string[];
  delta_vs_top1?: string;
  similar_suggestion_ids?: string[];
  recommended_merge_target_id?: string;
  materialized_skill_id?: string;
  materialized_skill_path?: string;
}

export interface Suggestion {
  suggestion_id: string;
  principal_id: string;
  workspace_root?: string;
  title: string;
  description?: string;
  risk_notes?: string;
  status: SuggestionStatus;
  evidence_receipt_ids: string[];
  evidence_count: number;
  draft_skill: string;
  merged_into_suggestion_id?: string;
  scores?: SuggestionScores;
  meta?: SuggestionMeta;
  created_at: string;
  updated_at: string;
}

export async function listSopSuggestions(params?: {
  day?: string;
  status?: SuggestionStatus;
  include_parked?: boolean;
  limit?: number;
}): Promise<Suggestion[]> {
  const qs = new URLSearchParams();
  if (params?.day) qs.set("day", params.day);
  if (params?.status) qs.set("status", params.status);
  if (params?.include_parked) qs.set("include_parked", "1");
  if (params?.limit) qs.set("limit", String(params.limit));
  const q = qs.toString();
  return api(`/api/ledger/sop_suggestions${q ? `?${q}` : ""}`);
}

export async function createSopSuggestion(payload: {
  workspace_root?: string;
  title: string;
  description?: string;
  risk_notes?: string;
  draft_skill: string;
  evidence_receipt_ids: string[];
  day_key?: string;
}): Promise<Suggestion> {
  return api("/api/ledger/sop_suggestions", { method: "POST", body: payload });
}

export async function generateSopSuggestions(payload?: {
  lookback_days?: number;
  count?: number;
}): Promise<Suggestion[]> {
  return api("/api/ledger/sop_suggestions/generate", {
    method: "POST",
    body: payload || {},
  });
}

export async function updateSopSuggestionStatus(
  suggestionId: string,
  payload: { status: SuggestionStatus; merged_into_suggestion_id?: string },
): Promise<Suggestion> {
  return api(
    `/api/ledger/sop_suggestions/${encodeURIComponent(suggestionId)}/status`,
    {
      method: "POST",
      body: payload,
    },
  );
}

export async function updateSopSuggestion(
  suggestionId: string,
  payload: {
    title?: string;
    description?: string;
    risk_notes?: string;
    draft_skill?: string;
  },
): Promise<Suggestion> {
  return api(
    `/api/ledger/sop_suggestions/${encodeURIComponent(suggestionId)}`,
    {
      method: "PUT",
      body: payload,
    },
  );
}

export async function loadMoreSopSuggestions(payload?: {
  day_key?: string;
  count?: number;
}): Promise<Suggestion[]> {
  return api("/api/ledger/sop_suggestions/load_more", {
    method: "POST",
    body: payload || {},
  });
}

export interface SimilarSuggestion {
  suggestion_id: string;
  title: string;
  status: SuggestionStatus;
  similarity: number;
}

export async function getSimilarSopSuggestions(
  suggestionId: string,
  limit: number = 5,
): Promise<SimilarSuggestion[]> {
  const qs = new URLSearchParams();
  if (limit > 0) qs.set("limit", String(limit));
  const q = qs.toString();
  return api(
    `/api/ledger/sop_suggestions/${encodeURIComponent(suggestionId)}/similar${q ? `?${q}` : ""}`,
  );
}

// ============================================================================
// Bocha Search Services
// ============================================================================

export interface BochaSettings {
  has_bocha_api_key: boolean;
}

export async function getBochaSettings(): Promise<BochaSettings> {
  return api("/api/bocha/settings");
}

export async function updateBochaSettings(payload: { bocha_api_key?: string }) {
  return api("/api/bocha/settings", { method: "PUT", body: payload });
}

export interface BochaSearchOptions {
  summary?: boolean;
  freshness?: "noLimit" | "oneDay" | "oneWeek" | "oneMonth" | "oneYear";
  count?: number;
}

export async function bochaSearch(query: string, options?: BochaSearchOptions) {
  return api("/api/bocha/search", {
    method: "POST",
    body: { query, ...options },
  });
}
