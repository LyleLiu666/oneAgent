import { ref } from "vue";

import {
  getSimpleToolPermissions,
  updateSimpleToolPermissions,
  type CommandApprovalMode,
  type SimpleToolPermissionResponse,
  type UpdateSimpleToolPermissionRequest,
} from "@/api/client";
import { parseApiError, type ParsedApiError } from "@/lib/apiError";

export function useSimpleToolPermissions() {
  const state = ref<SimpleToolPermissionResponse | null>(null);
  const loading = ref(false);
  const saving = ref(false);
  const error = ref<ParsedApiError | null>(null);
  const successMessage = ref("");

  const clearSuccessMessage = () => {
    successMessage.value = "";
  };

  const refresh = async () => {
    loading.value = true;
    error.value = null;
    try {
      state.value = await getSimpleToolPermissions();
    } catch (err) {
      error.value = parseApiError(err, "加载执行权限失败");
    } finally {
      loading.value = false;
    }
    return state.value;
  };

  const applyChange = async (
    payload: UpdateSimpleToolPermissionRequest,
    successMessageText: string,
  ) => {
    saving.value = true;
    error.value = null;
    successMessage.value = "";
    try {
      state.value = await updateSimpleToolPermissions(payload);
      successMessage.value = successMessageText;
      return state.value;
    } catch (err) {
      error.value = parseApiError(err, "保存执行权限失败");
      throw err;
    } finally {
      saving.value = false;
    }
  };

  const applyMode = async (
    mode: NonNullable<UpdateSimpleToolPermissionRequest["mode"]>,
    source: NonNullable<UpdateSimpleToolPermissionRequest["source"]>,
  ) => {
    const label =
      mode === "readonly"
        ? "只读查看"
        : mode === "sandbox_coding"
          ? "沙箱开发"
          : "本机执行";
    return applyChange(
      { mode, source },
      `已切换为${label}。正在运行的任务保持原权限；新任务或后续重试会使用新配置。`,
    );
  };

  const updateApprovalMode = async (
    commandApprovalMode: CommandApprovalMode,
    source: NonNullable<UpdateSimpleToolPermissionRequest["source"]>,
  ) => {
    const label = commandApprovalMode === "manual" ? "手动审批" : "自动审批";
    return applyChange(
      { command_approval_mode: commandApprovalMode, source },
      `已切换为${label}。`,
    );
  };

  return {
    state,
    loading,
    saving,
    error,
    successMessage,
    clearSuccessMessage,
    refresh,
    applyMode,
    updateApprovalMode,
  };
}
