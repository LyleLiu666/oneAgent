package checkpoint

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type WorkspaceCheckpoint struct {
	ArchivePath string
}

func CreateWorkspaceCheckpoint(ctx context.Context, workspaceRoot string, outDir string) (WorkspaceCheckpoint, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	outDir = strings.TrimSpace(outDir)
	if workspaceRoot == "" {
		return WorkspaceCheckpoint{}, errors.New("workspace_root is required")
	}
	if outDir == "" {
		return WorkspaceCheckpoint{}, errors.New("out_dir is required")
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return WorkspaceCheckpoint{}, fmt.Errorf("resolve workspace_root: %w", err)
	}
	root = filepath.Clean(root)
	if isFilesystemRoot(root) {
		return WorkspaceCheckpoint{}, fmt.Errorf("refusing to checkpoint filesystem root: %s", root)
	}

	if st, err := os.Stat(root); err != nil {
		return WorkspaceCheckpoint{}, fmt.Errorf("stat workspace_root: %w", err)
	} else if !st.IsDir() {
		return WorkspaceCheckpoint{}, fmt.Errorf("workspace_root is not a directory: %s", root)
	}

	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return WorkspaceCheckpoint{}, fmt.Errorf("create out_dir: %w", err)
	}

	archivePath := filepath.Join(outDir, "workspace.tgz")
	f, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return WorkspaceCheckpoint{}, fmt.Errorf("open checkpoint archive: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		info, err := os.Lstat(path)
		if err != nil {
			return err
		}

		name := filepath.ToSlash(rel)
		if name == "" || name == "." {
			return nil
		}

		linkTarget := ""
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			linkTarget = target
		}

		hdr, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		hdr.Name = name
		if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}

		switch {
		case info.IsDir():
			return tw.WriteHeader(hdr)
		case info.Mode()&os.ModeSymlink != 0:
			return tw.WriteHeader(hdr)
		case info.Mode().IsRegular():
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		default:
			// Skip special files (device, socket, etc).
			return nil
		}
	})
	if err != nil {
		return WorkspaceCheckpoint{}, err
	}

	return WorkspaceCheckpoint{ArchivePath: archivePath}, nil
}

func RestoreWorkspaceCheckpoint(ctx context.Context, workspaceRoot string, archivePath string) error {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	archivePath = strings.TrimSpace(archivePath)
	if workspaceRoot == "" {
		return errors.New("workspace_root is required")
	}
	if archivePath == "" {
		return errors.New("archive_path is required")
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve workspace_root: %w", err)
	}
	root = filepath.Clean(root)
	if isFilesystemRoot(root) {
		return fmt.Errorf("refusing to rollback filesystem root: %s", root)
	}

	if st, err := os.Stat(root); err != nil {
		return fmt.Errorf("stat workspace_root: %w", err)
	} else if !st.IsDir() {
		return fmt.Errorf("workspace_root is not a directory: %s", root)
	}

	if err := clearDir(root); err != nil {
		return err
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open checkpoint archive: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if hdr == nil {
			continue
		}

		name := strings.TrimSpace(hdr.Name)
		if name == "" {
			continue
		}

		rel := filepath.Clean(filepath.FromSlash(name))
		if rel == "." || rel == "" {
			continue
		}
		if filepath.IsAbs(rel) {
			return fmt.Errorf("invalid archive path (absolute): %s", name)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid archive path (traversal): %s", name)
		}

		targetPath := filepath.Join(root, rel)
		if !isWithinRoot(root, targetPath) {
			return fmt.Errorf("invalid archive path (outside root): %s", name)
		}

		mode := os.FileMode(hdr.Mode) & os.ModePerm
		if mode == 0 {
			mode = 0o700
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, mode); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
				return err
			}
			_ = os.RemoveAll(targetPath)
			if err := os.Symlink(hdr.Linkname, targetPath); err != nil {
				return err
			}
		case tar.TypeLink:
			linkRel := filepath.Clean(filepath.FromSlash(hdr.Linkname))
			if filepath.IsAbs(linkRel) || linkRel == ".." || strings.HasPrefix(linkRel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("invalid hardlink target: %s", hdr.Linkname)
			}
			linkTarget := filepath.Join(root, linkRel)
			if !isWithinRoot(root, linkTarget) {
				return fmt.Errorf("invalid hardlink target (outside root): %s", hdr.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
				return err
			}
			_ = os.RemoveAll(targetPath)
			if err := os.Link(linkTarget, targetPath); err != nil {
				return err
			}
		default:
			// Treat everything else as a regular file.
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
				return err
			}
			_ = os.RemoveAll(targetPath)
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func clearDir(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || name == "." || name == ".." {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	return nil
}

func isFilesystemRoot(path string) bool {
	path = filepath.Clean(path)
	return filepath.Dir(path) == path
}

func isWithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
