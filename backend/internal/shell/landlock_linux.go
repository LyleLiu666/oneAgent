//go:build linux

package shell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

var errLandlockUnavailable = errors.New("landlock is unavailable")

func landlockABIVersion() (uint64, error) {
	r1, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, uintptr(unix.LANDLOCK_CREATE_RULESET_VERSION))
	if errno != 0 {
		if errno == unix.ENOSYS {
			return 0, fmt.Errorf("%w: landlock syscalls not supported by kernel", errLandlockUnavailable)
		}
		return 0, fmt.Errorf("%w: failed to query landlock ABI version: %v", errLandlockUnavailable, errno)
	}
	return uint64(r1), nil
}

func landlockWriteAccessMask(abi uint64) uint64 {
	// Landlock v1 (Linux >= 5.13) supports basic filesystem access rights.
	handled := uint64(0) |
		unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
		unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
		unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
		unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
		unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
		unix.LANDLOCK_ACCESS_FS_MAKE_REG |
		unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
		unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_SYM

	// v2: REFER (rename/link across directories).
	if abi >= 2 {
		handled |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	// v3: TRUNCATE.
	if abi >= 3 {
		handled |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	// v5: IOCTL_DEV.
	if abi >= 5 {
		handled |= unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
	}

	return handled
}

func applyLandlockWriteSandbox(root string) error {
	abi, err := landlockABIVersion()
	if err != nil {
		return fmt.Errorf("native sandbox unavailable: %w (use sandbox_mode=docker)", err)
	}
	if abi < 1 {
		return fmt.Errorf("native sandbox unavailable: %w: unexpected ABI version=%d", errLandlockUnavailable, abi)
	}

	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("native sandbox unavailable: %w: root is required", errLandlockUnavailable)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("native sandbox unavailable: %w: resolve root: %v", errLandlockUnavailable, err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return fmt.Errorf("native sandbox unavailable: %w: resolve root symlinks: %v", errLandlockUnavailable, err)
	}

	info, err := os.Stat(realRoot)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("native sandbox unavailable: %w: root is not a directory: %s", errLandlockUnavailable, realRoot)
	}

	// Landlock requires no_new_privs.
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("native sandbox unavailable: landlock: failed to set no_new_privs: %v", err)
	}

	handled := landlockWriteAccessMask(abi)
	rulesetAttr := unix.LandlockRulesetAttr{Access_fs: handled}
	r1, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&rulesetAttr)), unsafe.Sizeof(rulesetAttr), 0)
	if errno != 0 {
		return fmt.Errorf("native sandbox unavailable: landlock_create_ruleset failed: %v", errno)
	}
	rulesetFD := int(r1)
	defer unix.Close(rulesetFD)

	rootFD, err := unix.Open(realRoot, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("native sandbox unavailable: landlock: failed to open root for rule: %v", err)
	}
	defer unix.Close(rootFD)

	rule := unix.LandlockPathBeneathAttr{
		Allowed_access: handled,
		Parent_fd:      int32(rootFD),
	}
	_, _, errno = unix.Syscall6(
		unix.SYS_LANDLOCK_ADD_RULE,
		uintptr(rulesetFD),
		uintptr(unix.LANDLOCK_RULE_PATH_BENEATH),
		uintptr(unsafe.Pointer(&rule)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("native sandbox unavailable: landlock_add_rule failed: %v", errno)
	}

	_, _, errno = unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, uintptr(rulesetFD), 0, 0)
	if errno != 0 {
		return fmt.Errorf("native sandbox unavailable: landlock_restrict_self failed: %v", errno)
	}

	return nil
}

