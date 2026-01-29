package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/builtinskills"
	"github.com/liu_y/oneAgent/backend/internal/fsutil"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/skill"
)

type skillInfo struct {
	SkillID     string       `json:"skill_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Source      skill.Source `json:"source"`
	Path        string       `json:"path"`
	Archivable  bool         `json:"archivable"`
}

func ListSkills(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	cat, err := skill.Discover(c.Request.Context(), skill.DiscoverOptions{})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	home := strings.TrimSpace(rt.Config.Home)
	out := make([]skillInfo, 0, len(cat.Skills))
	for _, s := range cat.Skills {
		out = append(out, skillInfo{
			SkillID:     s.ID,
			Name:        s.Name,
			Description: s.Description,
			Source:      s.Source,
			Path:        s.Path,
			Archivable:  canArchiveSkill(home, s),
		})
	}

	c.JSON(http.StatusOK, out)
}

type skillCandidateInfo struct {
	SkillID        string       `json:"skill_id"`
	Name           string       `json:"name"`
	Description    string       `json:"description"`
	Source         skill.Source `json:"source"`
	Path           string       `json:"path"`
	Archivable     bool         `json:"archivable"`
	Effective      bool         `json:"effective"`
	PrecedenceRank int          `json:"precedence_rank"`
}

type skillDuplicateGroup struct {
	SkillID    string               `json:"skill_id"`
	Candidates []skillCandidateInfo `json:"candidates"`
}

func ListSkillDuplicates(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	workspaceRoot := strings.TrimSpace(c.Query("workspace"))
	candidates, err := skill.DiscoverCandidates(c.Request.Context(), skill.DiscoverOptions{WorkspaceRoot: workspaceRoot})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	home := strings.TrimSpace(rt.Config.Home)
	byID := make(map[string][]skillCandidateInfo, 32)

	for _, cand := range candidates {
		s := cand.Skill
		byID[s.ID] = append(byID[s.ID], skillCandidateInfo{
			SkillID:        s.ID,
			Name:           s.Name,
			Description:    s.Description,
			Source:         s.Source,
			Path:           s.Path,
			Archivable:     canArchiveSkill(home, s),
			Effective:      cand.Effective,
			PrecedenceRank: cand.PrecedenceRank,
		})
	}

	keys := make([]string, 0, len(byID))
	for id, list := range byID {
		if len(list) < 2 {
			continue
		}
		keys = append(keys, id)
	}
	sort.Strings(keys)

	out := make([]skillDuplicateGroup, 0, len(keys))
	for _, id := range keys {
		out = append(out, skillDuplicateGroup{
			SkillID:    id,
			Candidates: byID[id],
		})
	}

	c.JSON(http.StatusOK, out)
}

type archiveSkillResult struct {
	OK           bool   `json:"ok"`
	SkillID      string `json:"skill_id"`
	ArchivedPath string `json:"archived_path"`
}

type getSkillResult struct {
	SkillID     string       `json:"skill_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Source      skill.Source `json:"source"`
	Path        string       `json:"path"`
	Archivable  bool         `json:"archivable"`

	Files []string `json:"files,omitempty"`

	SHA256  string `json:"sha256"`
	SkillMD string `json:"skill_md"`
}

func GetSkill(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill id is required"})
		return
	}

	cat, err := skill.Discover(c.Request.Context(), skill.DiscoverOptions{})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	s, ok := cat.ByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	home := strings.TrimSpace(rt.Config.Home)
	archivable := canArchiveSkill(home, s)

	files, err := listSkillFiles(c.Request.Context(), s)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	data, err := skill.ReadSkillFile(s.Path, 512*1024)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	sum := sha256.Sum256(data)

	c.JSON(http.StatusOK, getSkillResult{
		SkillID:     s.ID,
		Name:        s.Name,
		Description: s.Description,
		Source:      s.Source,
		Path:        s.Path,
		Archivable:  archivable,
		Files:       files,
		SHA256:      hex.EncodeToString(sum[:]),
		SkillMD:     string(data),
	})
}

type readSkillFileResult struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	// Content is UTF-8 decoded file contents.
	Content string `json:"content"`
}

func ReadSkillFile(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill id is required"})
		return
	}

	rel := strings.TrimSpace(c.Query("path"))
	if rel == "" {
		rel = "SKILL.md"
	}
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "./")
	rel = path.Clean(rel)
	if rel == "" || rel == "." || strings.HasPrefix(rel, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}

	cat, err := skill.Discover(c.Request.Context(), skill.DiscoverOptions{})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	s, ok := cat.ByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	data, err := readSkillFileRelative(c.Request.Context(), s, rel, 512*1024)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	if !utf8.Valid(data) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is not valid UTF-8"})
		return
	}
	sum := sha256.Sum256(data)

	c.JSON(http.StatusOK, readSkillFileResult{
		Path:    rel,
		SHA256:  hex.EncodeToString(sum[:]),
		Content: string(data),
	})
}

func listSkillFiles(ctx context.Context, s skill.Skill) ([]string, error) {
	const maxFiles = 2000

	p := strings.TrimSpace(s.Path)
	if strings.HasPrefix(p, "builtin:") {
		rel := strings.TrimPrefix(p, "builtin:")
		rel = strings.TrimPrefix(rel, "/")
		rel = strings.TrimPrefix(rel, "./")
		rel = path.Clean(rel)
		if rel == "" || rel == "." || strings.HasPrefix(rel, "..") {
			return nil, errors.New("invalid builtin skill path")
		}

		root := path.Dir(rel)
		files := make([]string, 0, 8)
		err := fs.WalkDir(builtinskills.FS, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if d == nil || d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			p = path.Clean(p)
			rootPrefix := root + "/"
			if !strings.HasPrefix(p, rootPrefix) {
				return nil
			}
			r := strings.TrimPrefix(p, rootPrefix)
			r = path.Clean(r)
			if r == "" || r == "." || strings.HasPrefix(r, "..") {
				return nil
			}
			files = append(files, r)
			if len(files) > maxFiles {
				return errors.New("too many files in skill")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
		return files, nil
	}

	root := filepath.Clean(filepath.Dir(p))
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("skill root is not a directory")
	}

	files := make([]string, 0, 16)
	err = filepath.WalkDir(root, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d == nil || d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		r, err := filepath.Rel(root, full)
		if err != nil {
			return err
		}
		r = filepath.ToSlash(filepath.Clean(r))
		if r == "" || r == "." || strings.HasPrefix(r, "..") {
			return nil
		}
		files = append(files, r)
		if len(files) > maxFiles {
			return errors.New("too many files in skill")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func readSkillFileRelative(ctx context.Context, s skill.Skill, rel string, maxBytes int64) ([]byte, error) {
	_ = ctx
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "./")
	rel = path.Clean(rel)
	if rel == "" || rel == "." || strings.HasPrefix(rel, "..") {
		return nil, errors.New("invalid path")
	}

	p := strings.TrimSpace(s.Path)
	if strings.HasPrefix(p, "builtin:") {
		base := strings.TrimPrefix(p, "builtin:")
		base = strings.TrimPrefix(base, "/")
		base = strings.TrimPrefix(base, "./")
		base = path.Clean(base)
		if base == "" || base == "." || strings.HasPrefix(base, "..") {
			return nil, errors.New("invalid builtin skill path")
		}
		root := path.Dir(base)
		full := path.Join(root, rel)
		full = path.Clean(full)
		if full == "" || full == "." || strings.HasPrefix(full, "..") {
			return nil, errors.New("invalid path")
		}
		return skill.ReadSkillFile("builtin:"+full, maxBytes)
	}

	root := filepath.Clean(filepath.Dir(p))
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("skill root is not a directory")
	}

	rootReal := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil && strings.TrimSpace(resolved) != "" {
		rootReal = filepath.Clean(resolved)
	}

	candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	rootPrefix := root + string(os.PathSeparator)
	if !strings.HasPrefix(candidate+string(os.PathSeparator), rootPrefix) {
		return nil, errors.New("path escapes skill root")
	}

	target := candidate
	if fi, err := os.Lstat(candidate); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil || strings.TrimSpace(resolved) == "" {
			return nil, errors.New("failed to resolve symlink")
		}
		target = filepath.Clean(resolved)
	}

	rootRealPrefix := rootReal + string(os.PathSeparator)
	if !strings.HasPrefix(target+string(os.PathSeparator), rootRealPrefix) {
		return nil, errors.New("path escapes skill root")
	}

	return skill.ReadSkillFile(candidate, maxBytes)
}

type updateSkillRequest struct {
	SkillMD        string `json:"skill_md"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty"`
}

func UpdateSkill(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill id is required"})
		return
	}

	var req updateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if strings.TrimSpace(req.SkillMD) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill_md is required"})
		return
	}

	cat, err := skill.Discover(c.Request.Context(), skill.DiscoverOptions{})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	s, ok := cat.ByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	home := strings.TrimSpace(rt.Config.Home)
	if !canArchiveSkill(home, s) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill is not editable"})
		return
	}

	current, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	curSum := sha256.Sum256(current)
	curSHA := hex.EncodeToString(curSum[:])
	if strings.TrimSpace(req.ExpectedSHA256) != "" && strings.TrimSpace(req.ExpectedSHA256) != curSHA {
		c.JSON(http.StatusConflict, gin.H{"error": "precondition failed: skill changed (expected_sha256 mismatch)"})
		return
	}

	if err := fsutil.AtomicWriteFile(s.Path, []byte(req.SkillMD), 0o644); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	updated, _ := os.ReadFile(s.Path)
	newSum := sha256.Sum256(updated)

	c.JSON(http.StatusOK, getSkillResult{
		SkillID:     s.ID,
		Name:        s.Name,
		Description: s.Description,
		Source:      s.Source,
		Path:        s.Path,
		Archivable:  true,
		SHA256:      hex.EncodeToString(newSum[:]),
		SkillMD:     string(updated),
	})
}

func ArchiveSkill(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	home := strings.TrimSpace(rt.Config.Home)
	if home == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "home is required"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill id is required"})
		return
	}

	cat, err := skill.Discover(c.Request.Context(), skill.DiscoverOptions{})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	s, ok := cat.ByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}
	if !canArchiveSkill(home, s) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill is not archivable"})
		return
	}

	archivedPath, err := archiveOneAgentSkill(home, s)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, archiveSkillResult{
		OK:           true,
		SkillID:      s.ID,
		ArchivedPath: archivedPath,
	})
}

func canArchiveSkill(home string, s skill.Skill) bool {
	home = strings.TrimSpace(home)
	if home == "" {
		return false
	}
	if s.Source != skill.SourceOneAgent {
		return false
	}
	root := filepath.Clean(filepath.Join(home, ".oneagent", "skills"))
	if resolved, err := filepath.EvalSymlinks(root); err == nil && strings.TrimSpace(resolved) != "" {
		root = filepath.Clean(resolved)
	}
	root = root + string(os.PathSeparator)

	p := filepath.Clean(s.Path)
	if resolved, err := filepath.EvalSymlinks(p); err == nil && strings.TrimSpace(resolved) != "" {
		p = filepath.Clean(resolved)
	}
	return strings.HasPrefix(p+string(os.PathSeparator), root)
}

func archiveOneAgentSkill(home string, s skill.Skill) (string, error) {
	home = strings.TrimSpace(home)
	if home == "" {
		return "", errors.New("home is required")
	}
	if strings.TrimSpace(s.Path) == "" {
		return "", errors.New("skill path is required")
	}

	srcDir := filepath.Dir(s.Path)
	if _, err := os.Stat(srcDir); err != nil {
		return "", err
	}

	archivedRoot := filepath.Join(home, ".oneagent", "skills-archived")
	if err := os.MkdirAll(archivedRoot, 0o700); err != nil {
		return "", err
	}

	base := s.ID
	if strings.TrimSpace(base) == "" {
		base = filepath.Base(filepath.Dir(s.Path))
	}

	dstDir := filepath.Join(archivedRoot, base)
	if _, err := os.Stat(dstDir); err == nil {
		dstDir = filepath.Join(archivedRoot, base+"-"+time.Now().Format("20060102-150405"))
	}

	if err := os.Rename(srcDir, dstDir); err != nil {
		return "", err
	}

	return filepath.Join(dstDir, "SKILL.md"), nil
}

type pinSkillRequest struct {
	WorkspaceRoot   string       `json:"workspace_root,omitempty"`
	Source          skill.Source `json:"source"`
	Path            string       `json:"path"`
	ArchiveShadowed bool         `json:"archive_shadowed_personal,omitempty"`
}

type pinSkillResult struct {
	OK            bool                 `json:"ok"`
	SkillID       string               `json:"skill_id"`
	CanonicalPath string               `json:"canonical_path"`
	ArchivedPaths []string             `json:"archived_paths,omitempty"`
	Shadowed      []skillCandidateInfo `json:"shadowed_candidates,omitempty"`
}

func PinSkillCandidate(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	home := strings.TrimSpace(rt.Config.Home)
	if home == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "home is required"})
		return
	}

	skillID := strings.TrimSpace(c.Param("id"))
	if skillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill id is required"})
		return
	}
	skillID = skill.NormalizeName(skillID)
	if skillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid skill id"})
		return
	}

	var req pinSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.Path = strings.TrimSpace(req.Path)
	if req.Path == "" || strings.TrimSpace(string(req.Source)) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source and path are required"})
		return
	}

	candidates, err := skill.DiscoverCandidates(c.Request.Context(), skill.DiscoverOptions{WorkspaceRoot: strings.TrimSpace(req.WorkspaceRoot)})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	var picked *skill.Skill
	for _, cand := range candidates {
		if cand.Skill.ID == skillID && cand.Skill.Source == req.Source && strings.TrimSpace(cand.Skill.Path) == req.Path {
			s := cand.Skill
			picked = &s
			break
		}
	}
	if picked == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "candidate not found"})
		return
	}

	data, err := os.ReadFile(picked.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "candidate not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	canonicalDir := filepath.Join(home, ".oneagent", "skills", skillID)
	canonicalPath := filepath.Join(canonicalDir, "SKILL.md")
	if err := os.MkdirAll(canonicalDir, 0o700); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := fsutil.AtomicWriteFile(canonicalPath, data, 0o644); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	archived := []string(nil)
	shadowed := []skillCandidateInfo(nil)
	if req.ArchiveShadowed {
		archived, shadowed, err = archiveShadowedPersonalDuplicates(c.Request.Context(), home, skillID, canonicalPath)
		if err != nil {
			RespondError(c, http.StatusBadRequest, err)
			return
		}
	}

	c.JSON(http.StatusOK, pinSkillResult{
		OK:            true,
		SkillID:       skillID,
		CanonicalPath: canonicalPath,
		ArchivedPaths: archived,
		Shadowed:      shadowed,
	})
}

type archiveShadowedResult struct {
	OK            bool     `json:"ok"`
	SkillID       string   `json:"skill_id"`
	ArchivedPaths []string `json:"archived_paths"`
}

func ArchiveShadowedPersonalDuplicates(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	home := strings.TrimSpace(rt.Config.Home)
	if home == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "home is required"})
		return
	}

	skillID := strings.TrimSpace(c.Param("id"))
	if skillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill id is required"})
		return
	}
	skillID = skill.NormalizeName(skillID)
	if skillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid skill id"})
		return
	}

	canonicalPath := filepath.Join(home, ".oneagent", "skills", skillID, "SKILL.md")
	archived, _, err := archiveShadowedPersonalDuplicates(c.Request.Context(), home, skillID, canonicalPath)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, archiveShadowedResult{
		OK:            true,
		SkillID:       skillID,
		ArchivedPaths: archived,
	})
}

func archiveShadowedPersonalDuplicates(ctx context.Context, home string, skillID string, canonicalPath string) ([]string, []skillCandidateInfo, error) {
	home = strings.TrimSpace(home)
	skillID = skill.NormalizeName(skillID)
	canonicalPath = filepath.Clean(strings.TrimSpace(canonicalPath))
	if home == "" || skillID == "" || canonicalPath == "" {
		return nil, nil, errors.New("home, skillID, canonicalPath are required")
	}
	if _, err := os.Stat(canonicalPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, errors.New("canonical skill not found; pin first")
		}
		return nil, nil, err
	}

	canonicalReal := canonicalPath
	if resolved, err := filepath.EvalSymlinks(canonicalPath); err == nil && strings.TrimSpace(resolved) != "" {
		canonicalReal = filepath.Clean(resolved)
	}

	candidates, err := skill.DiscoverCandidates(ctx, skill.DiscoverOptions{})
	if err != nil {
		return nil, nil, err
	}

	archived := make([]string, 0, 4)
	shadowed := make([]skillCandidateInfo, 0, 4)

	for _, cand := range candidates {
		s := cand.Skill
		if s.ID != skillID {
			continue
		}
		if s.Source != skill.SourceOneAgent {
			continue
		}
		if !canArchiveSkill(home, s) {
			continue
		}
		candPath := filepath.Clean(strings.TrimSpace(s.Path))
		if candPath == canonicalPath {
			continue
		}
		candReal := candPath
		if resolved, err := filepath.EvalSymlinks(candPath); err == nil && strings.TrimSpace(resolved) != "" {
			candReal = filepath.Clean(resolved)
		}
		if candReal == canonicalReal {
			continue
		}

		archivedPath, err := archiveOneAgentSkill(home, s)
		if err != nil {
			return nil, nil, err
		}
		archived = append(archived, archivedPath)
		shadowed = append(shadowed, skillCandidateInfo{
			SkillID:        s.ID,
			Name:           s.Name,
			Description:    s.Description,
			Source:         s.Source,
			Path:           s.Path,
			Archivable:     true,
			Effective:      cand.Effective,
			PrecedenceRank: cand.PrecedenceRank,
		})
	}

	return archived, shadowed, nil
}
