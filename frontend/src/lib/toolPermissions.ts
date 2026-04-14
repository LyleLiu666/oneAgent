export type SimpleToolPermissionPreset =
  | "readonly"
  | "sandbox_coding"
  | "host_full"
  | "custom";

export function simpleToolPermissionLabel(
  mode: string | null | undefined,
): string {
  switch (String(mode || "").trim().toLowerCase()) {
    case "readonly":
      return "只读查看";
    case "sandbox_coding":
      return "沙箱开发";
    case "host_full":
      return "本机执行";
    default:
      return "自定义";
  }
}

export function isToolPermissionDeniedLike(raw: unknown): boolean {
  const value = String(raw ?? "").trim().toLowerCase();
  if (!value) return false;
  return (
    value.includes("tool_permission_denied") ||
    value.includes("permission denied") ||
    value.includes("denied_by_rule") ||
    value.includes("default_deny") ||
    value.includes("操作被策略拒绝")
  );
}

export function messageLikelyNeedsElevatedPermissions(raw: unknown): boolean {
  const value = String(raw ?? "").trim().toLowerCase();
  if (!value) return false;

  const patterns = [
    "修改",
    "改代码",
    "写文件",
    "运行测试",
    "跑测试",
    "执行命令",
    "安装",
    "修复",
    "提交代码",
    "build",
    "test",
    "run",
    "npm",
    "pnpm",
    "yarn",
    "go test",
    "pytest",
    "cargo",
    "edit",
    "write",
    "patch",
    "fix",
    "implement",
    "deploy",
  ];
  return patterns.some((pattern) => value.includes(pattern));
}
