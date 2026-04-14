//go:build !darwin && !windows

package handler

import (
	"context"
	"fmt"
	"runtime"
)

func defaultChooseWorkspaceDir(ctx context.Context) (string, bool, error) {
	_ = ctx
	return "", false, fmt.Errorf("%w: %s", ErrWorkspaceChooserNotSupported, runtime.GOOS)
}

func defaultWorkspaceChooserCapability() workspaceChooserCapability {
	return workspaceChooserCapability{
		Supported: false,
		Reason:    "当前服务端环境不支持原生文件夹选择，请手动填写服务端工作区路径。",
	}
}
