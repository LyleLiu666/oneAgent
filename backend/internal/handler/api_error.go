package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
)

type apiErrorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

// PublicError is a user-safe, actionable API error that can be returned to clients.
// The wrapped Err is considered internal and should not be surfaced directly.
type PublicError struct {
	Status int
	Code   string
	Public string
	Hint   string
	Err    error
}

func (e *PublicError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Public) != "" {
		return strings.TrimSpace(e.Public)
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "error"
}

func (e *PublicError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func RespondError(c *gin.Context, status int, err error) {
	code, msg, hint := classifyAPIError(status, err)
	if strings.TrimSpace(msg) == "" {
		msg = "发生错误"
	}

	c.JSON(status, apiErrorResponse{
		Error:     msg,
		Code:      code,
		RequestID: strings.TrimSpace(middleware.GetRequestID(c)),
		Hint:      hint,
	})
}

func classifyAPIError(status int, err error) (code string, msg string, hint string) {
	var pe *PublicError
	if err != nil && errors.As(err, &pe) {
		msg = strings.TrimSpace(pe.Public)
		code = strings.TrimSpace(pe.Code)
		hint = strings.TrimSpace(pe.Hint)
		if msg == "" {
			msg = redactErrorString(pe.Error())
		}
		return code, msg, hint
	}

	raw := ""
	if err != nil {
		raw = redactErrorString(err.Error())
	}

	// Heuristic mapping for tool permission errors (avoid leaking internal policy strings).
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "not allowed") ||
		strings.Contains(lower, "denied_by_rule") ||
		strings.Contains(lower, "default_deny") {
		return "tool_permission_denied", "操作被策略拒绝", "请前往“工具权限”页面调整 policy/profile 后重试"
	}
	if strings.Contains(lower, "no llm model configured") {
		return "llm_model_missing", "还没有配置可用的大模型", "请前往“设置”添加 Provider，并至少设置一个默认 Model"
	}
	if strings.Contains(lower, "model not found") {
		return "llm_model_not_found", "找不到当前模型配置", "请前往“设置”重新选择，或重新设置默认 Model"
	}
	if strings.Contains(lower, "provider not found") {
		return "llm_provider_not_found", "找不到当前供应商配置", "请前往“设置”重新配置 Provider 和 Model"
	}
	if strings.Contains(lower, "provider base_url or api_key is missing") {
		return "llm_provider_incomplete", "当前供应商配置不完整", "请前往“设置”补全 Base URL 和 API Key"
	}
	if errors.Is(err, ErrWorkspaceChooserNotSupported) {
		return "workspace_chooser_unsupported", "当前服务端环境不支持原生文件夹选择", "请手动填写服务端工作区路径"
	}

	switch status {
	case http.StatusBadRequest:
		if raw == "" {
			raw = "请求无效"
		}
		return "bad_request", raw, ""
	case http.StatusUnauthorized:
		return "unauthorized", "未授权", "请登录或提供有效 token"
	case http.StatusForbidden:
		return "forbidden", "无权限", ""
	case http.StatusNotFound:
		return "not_found", "未找到", ""
	case http.StatusServiceUnavailable:
		return "service_unavailable", "服务未就绪", "请稍后重试"
	default:
		return "internal_error", "服务器错误", "请稍后重试；必要时使用 request_id 定位日志"
	}
}

var (
	reWindowsAbsPath = regexp.MustCompile(`[A-Za-z]:\\[^\s"'<>]+`)
	reUnixAbsPath    = regexp.MustCompile(`(?m)(?:^|\s)(/[^\s"'<>]+)+`)
	reBearerToken    = regexp.MustCompile(`(?i)bearer\s+[^\s]+`)
)

func redactErrorString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	s = reBearerToken.ReplaceAllString(s, "Bearer <redacted>")
	s = reWindowsAbsPath.ReplaceAllString(s, "<path>")
	s = reUnixAbsPath.ReplaceAllStringFunc(s, func(m string) string {
		prefix := ""
		if strings.HasPrefix(m, " ") || strings.HasPrefix(m, "\t") {
			prefix = m[:1]
		}
		return prefix + "<path>"
	})
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}

	const maxLen = 240
	if len(s) > maxLen {
		s = s[:maxLen] + "…"
	}
	return s
}
