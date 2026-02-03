package handler

import (
	"net/http"
	"strings"
)

const sessionModuleMismatchCode = "session_module_mismatch"

func sessionModuleMismatchError(expectedModule, actualModule string) *PublicError {
	expected := strings.TrimSpace(expectedModule)
	_ = strings.TrimSpace(actualModule)

	public := "会话模式不匹配"
	hint := "请切换到正确的模式/入口后重试"

	switch expected {
	case "assistant":
		public = "这个会话属于秘书模式，不能在完全模式里继续。"
		hint = "请切换到“秘书模式”（/secretary）；或在完全模式新建一个对话。"
	case "secretary":
		public = "这个会话属于完全模式，不能在秘书模式里继续。"
		hint = "请切换到“完全模式”（/chat）后再试。"
	}

	return &PublicError{
		Status: http.StatusConflict,
		Code:   sessionModuleMismatchCode,
		Public: public,
		Hint:   hint,
	}
}
