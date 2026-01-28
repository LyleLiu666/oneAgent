package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	candidates, err := skill.DiscoverCandidates(c.Request.Context(), skill.DiscoverOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s, ok := cat.ByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	home := strings.TrimSpace(rt.Config.Home)
	archivable := canArchiveSkill(home, s)

	if !archivable {
		// For now, only expose raw SKILL.md for personal skills to avoid widening read scope.
		c.JSON(http.StatusOK, getSkillResult{
			SkillID:     s.ID,
			Name:        s.Name,
			Description: s.Description,
			Source:      s.Source,
			Path:        s.Path,
			Archivable:  false,
			SHA256:      "",
			SkillMD:     "",
		})
		return
	}

	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sum := sha256.Sum256(data)

	c.JSON(http.StatusOK, getSkillResult{
		SkillID:     s.ID,
		Name:        s.Name,
		Description: s.Description,
		Source:      s.Source,
		Path:        s.Path,
		Archivable:  true,
		SHA256:      hex.EncodeToString(sum[:]),
		SkillMD:     string(data),
	})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	curSum := sha256.Sum256(current)
	curSHA := hex.EncodeToString(curSum[:])
	if strings.TrimSpace(req.ExpectedSHA256) != "" && strings.TrimSpace(req.ExpectedSHA256) != curSHA {
		c.JSON(http.StatusConflict, gin.H{"error": "precondition failed: skill changed (expected_sha256 mismatch)"})
		return
	}

	if err := fsutil.AtomicWriteFile(s.Path, []byte(req.SkillMD), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
