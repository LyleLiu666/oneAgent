//go:build windows

package shell

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	createRestrictedTokenDisableMaxPrivilege = 0x1
	createRestrictedTokenSandboxInert        = 0x2
)

var (
	advapi32                  = windows.NewLazySystemDLL("advapi32.dll")
	procCreateRestrictedToken = advapi32.NewProc("CreateRestrictedToken")
)

func createWindowsNativeSandboxToken(workspaceRoot string) (syscall.Token, func(), error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return 0, nil, fmt.Errorf("native sandbox unavailable: workspace root is required")
	}

	// Use BUILTIN\\Users as a widely-present restricted SID to keep system DLL/exe reads working,
	// while still preventing access to user-private locations by default (best-effort).
	usersSID, err := windows.StringToSid("S-1-5-32-545")
	if err != nil {
		return 0, nil, fmt.Errorf("native sandbox unavailable: parse Users SID: %v", err)
	}

	if err := ensureDirACLAllowsSID(root, usersSID); err != nil {
		return 0, nil, fmt.Errorf("native sandbox unavailable: update workspace ACL: %v", err)
	}

	var base windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY, &base); err != nil {
		return 0, nil, fmt.Errorf("native sandbox unavailable: open process token: %v", err)
	}
	defer base.Close()

	restricted, err := createRestrictedToken(base, []windows.SIDAndAttributes{
		{Sid: usersSID, Attributes: windows.SE_GROUP_ENABLED},
	})
	if err != nil {
		return 0, nil, fmt.Errorf("native sandbox unavailable: create restricted token: %v", err)
	}

	cleanup := func() { _ = restricted.Close() }
	return syscall.Token(restricted), cleanup, nil
}

func createRestrictedToken(base windows.Token, restricted []windows.SIDAndAttributes) (windows.Token, error) {
	var out windows.Token

	var restrictedPtr *windows.SIDAndAttributes
	if len(restricted) > 0 {
		restrictedPtr = &restricted[0]
	}

	flags := uintptr(createRestrictedTokenDisableMaxPrivilege | createRestrictedTokenSandboxInert)
	r1, _, e1 := procCreateRestrictedToken.Call(
		uintptr(base),
		flags,
		0,
		0,
		0,
		0,
		uintptr(len(restricted)),
		uintptr(unsafe.Pointer(restrictedPtr)),
		uintptr(unsafe.Pointer(&out)),
	)
	if r1 == 0 {
		if e1 != nil && e1 != syscall.Errno(0) {
			return 0, e1
		}
		return 0, syscall.EINVAL
	}
	return out, nil
}

func ensureDirACLAllowsSID(path string, sid *windows.SID) error {
	path = strings.TrimSpace(path)
	if path == "" || sid == nil {
		return syscall.EINVAL
	}

	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	existingDACL, _, daclErr := sd.DACL()
	if daclErr != nil && daclErr != windows.ERROR_OBJECT_NOT_FOUND {
		return daclErr
	}

	entry := windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}

	newDACL, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{entry}, existingDACL)
	if err != nil {
		return err
	}

	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, newDACL, nil)
}

