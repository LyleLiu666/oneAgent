//go:build windows

package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	bifReturnOnlyFSDirs  = 0x0001
	bifNewDialogStyle    = 0x0040
	bifEditBox           = 0x0010
	bifUseNewUI          = bifNewDialogStyle | bifEditBox
	maxPathUTF16Elements = 260
)

type browseInfo struct {
	hwndOwner      windows.Handle
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

var (
	shell32               = windows.NewLazySystemDLL("shell32.dll")
	procSHBrowseForFolder = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromID   = shell32.NewProc("SHGetPathFromIDListW")
)

func defaultChooseWorkspaceDir(ctx context.Context) (string, bool, error) {
	return chooseWorkspaceWindows(ctx)
}

func chooseWorkspaceWindows(ctx context.Context) (string, bool, error) {
	select {
	case <-ctx.Done():
		return "", true, ctx.Err()
	default:
	}

	initialized := false
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil {
		// RPC_E_CHANGED_MODE means COM was already initialized with a different threading model.
		// In that case, proceed without failing.
		if !errors.Is(err, syscall.Errno(windows.RPC_E_CHANGED_MODE)) {
			return "", false, fmt.Errorf("choose folder failed: %w", err)
		}
	} else {
		initialized = true
	}
	if initialized {
		defer windows.CoUninitialize()
	}

	title, err := windows.UTF16PtrFromString("Select workspace folder")
	if err != nil {
		return "", false, err
	}

	display := make([]uint16, maxPathUTF16Elements)
	bi := browseInfo{
		hwndOwner:      0,
		pidlRoot:       0,
		pszDisplayName: &display[0],
		lpszTitle:      title,
		ulFlags:        bifReturnOnlyFSDirs | bifUseNewUI,
		lpfn:           0,
		lParam:         0,
		iImage:         0,
	}

	pidl, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", true, nil
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(pidl))

	buf := make([]uint16, windows.MAX_PATH)
	ok, _, _ := procSHGetPathFromID.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))
	if ok == 0 {
		return "", false, errors.New("choose folder failed")
	}

	path := strings.TrimSpace(windows.UTF16ToString(buf))
	if path == "" {
		return "", true, nil
	}
	return path, false, nil
}
