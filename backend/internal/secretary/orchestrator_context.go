package secretary

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/liu_y/oneAgent/backend/internal/model"
)

const (
	triageCarryMaxQuestions      = 3
	triageCarryQuestionMaxRunes  = 220
	triageCarrySummaryMaxRunes   = 1200
	triageCarryAnchorKeep        = 10
	triageCarryAnchorMaxRunes    = 320
	triageSearchMaxCandidates    = 12
	triageSearchDefaultMaxDepth  = 5
	triageSearchDefaultMaxVisits = 6000
)

type triageCarryContext struct {
	PendingQuestions       []string
	LastSummary            string
	SemanticAnchors        []string
	OmittedAnchors         int
	ShortFollowupLikelyAns bool

	SearchPolicy   secretarySearchPolicy
	SearchSnapshot secretarySearchSnapshot
}

type secretarySearchPolicy struct {
	DefaultReadonlySearchRoot string

	Phase1HomeRoots      []string
	Phase2CommonDevRoots []string
	Phase3WhitelistRoots []string

	TimeoutPerPhase time.Duration
	MaxVisitedDirs  int
	MaxDepth        int
	MaxCandidates   int

	ExcludeHidden       bool
	ExcludedDirNames    map[string]struct{}
	ExcludedRootPrefix  []string
	IncludeSystemByHint bool
}

type secretarySearchSnapshot struct {
	Enabled      bool
	Query        string
	FinalPhase   string
	Expanded     bool
	TimeoutHit   bool
	ScannedRoots []string
	Candidates   []string
	Omitted      int
}

func normalizeCarryQuestions(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, minInt(len(in), triageCarryMaxQuestions))
	seen := make(map[string]struct{}, len(in))
	for _, q := range in {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		q = truncateString(q, triageCarryQuestionMaxRunes)
		if _, ok := seen[q]; ok {
			continue
		}
		seen[q] = struct{}{}
		out = append(out, q)
		if len(out) >= triageCarryMaxQuestions {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectSemanticAnchors(msgs []model.ChatMessage, cursor uint) ([]string, int) {
	all := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if m.ID == 0 || m.ID > cursor {
			continue
		}
		if m.Type != model.MessageTypeText {
			continue
		}
		if m.Role != model.MessageRoleUser && m.Role != model.MessageRoleAssistant {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		content = truncateString(content, triageCarryAnchorMaxRunes)
		all = append(all, fmt.Sprintf("[%s#%d] %s", strings.TrimSpace(m.Role), m.ID, content))
	}
	if len(all) == 0 {
		return nil, 0
	}
	if len(all) <= triageCarryAnchorKeep {
		return all, 0
	}
	omitted := len(all) - triageCarryAnchorKeep
	return append([]string{}, all[omitted:]...), omitted
}

func likelyShortFollowupAnswer(msgs []model.ChatMessage, pendingQuestions []string) bool {
	if len(msgs) != 1 || len(pendingQuestions) == 0 {
		return false
	}
	text := strings.TrimSpace(msgs[0].Content)
	if text == "" {
		return false
	}
	if utf8.RuneCountInString(text) > 64 {
		return false
	}
	if strings.ContainsAny(text, "?？") {
		return false
	}
	return true
}

func isLikelyPathLookupQuestion(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	containsAny := func(needles ...string) bool {
		for _, needle := range needles {
			if strings.Contains(lower, needle) {
				return true
			}
		}
		return false
	}
	if containsAny("path", "目录", "路径", "在哪", "哪里", "在哪个", "location") {
		return true
	}
	if containsAny("find", "查", "找") && containsAny("project", "repo", "仓库", "项目", "go", "golang", "代码") {
		return true
	}
	if containsAny("home", "user home", "主目录") {
		return true
	}
	return false
}

func hasWorkspaceBindingQuestion(questions []string) bool {
	for _, q := range questions {
		q = strings.ToLower(strings.TrimSpace(q))
		if q == "" {
			continue
		}
		if strings.Contains(q, "目录") || strings.Contains(q, "路径") || strings.Contains(q, "workspace") || strings.Contains(q, "repo") || strings.Contains(q, "仓库") {
			return true
		}
	}
	return false
}

func defaultSecretarySearchPolicy() secretarySearchPolicy {
	home, _ := os.UserHomeDir()
	home = strings.TrimSpace(home)
	home = filepath.Clean(home)

	phase1 := []string{}
	if home != "" && home != "." {
		phase1 = append(phase1, home)
	}

	phase2 := []string{}
	phase3 := []string{}
	switch runtime.GOOS {
	case "darwin":
		phase2 = []string{"/Users", "/Volumes", "/opt", "/workspace", "/usr/local/src"}
		phase3 = []string{"/private", "/usr/local", "/Applications"}
	case "windows":
		phase2 = []string{"C:\\Users", "D:\\"}
		phase3 = []string{"C:\\work", "C:\\workspace"}
	default:
		phase2 = []string{"/home", "/opt", "/srv", "/workspace", "/mnt"}
		phase3 = []string{"/var/www", "/data"}
	}

	excludedNames := map[string]struct{}{
		".git":         {},
		".svn":         {},
		".hg":          {},
		"node_modules": {},
		"__pycache__":  {},
		".cache":       {},
		"Library":      {},
		"System":       {},
		"Applications": {},
		"tmp":          {},
		"var":          {},
		"proc":         {},
		"sys":          {},
		"dev":          {},
	}

	excludedPrefixes := []string{}
	switch runtime.GOOS {
	case "darwin":
		excludedPrefixes = []string{"/System", "/Library", "/usr", "/bin", "/sbin", "/private/var"}
	case "windows":
		excludedPrefixes = []string{"C:\\Windows", "C:\\Program Files", "C:\\ProgramData"}
	default:
		excludedPrefixes = []string{"/proc", "/sys", "/dev", "/run", "/lib", "/lib64", "/usr"}
	}

	return secretarySearchPolicy{
		DefaultReadonlySearchRoot: home,
		Phase1HomeRoots:           dedupeAndExistingDirs(phase1),
		Phase2CommonDevRoots:      dedupeAndExistingDirs(phase2),
		Phase3WhitelistRoots:      dedupeAndExistingDirs(phase3),
		TimeoutPerPhase:           1200 * time.Millisecond,
		MaxVisitedDirs:            triageSearchDefaultMaxVisits,
		MaxDepth:                  triageSearchDefaultMaxDepth,
		MaxCandidates:             triageSearchMaxCandidates,
		ExcludeHidden:             true,
		ExcludedDirNames:          excludedNames,
		ExcludedRootPrefix:        excludedPrefixes,
	}
}

func dedupeAndExistingDirs(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		root := filepath.Clean(strings.TrimSpace(raw))
		if root == "" {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		st, err := os.Stat(root)
		if err != nil || !st.IsDir() {
			continue
		}
		seen[root] = struct{}{}
		out = append(out, root)
	}
	return out
}

func shouldSkipSearchDir(path string, name string, policy secretarySearchPolicy) bool {
	if strings.TrimSpace(name) == "" {
		return true
	}
	if policy.ExcludeHidden && strings.HasPrefix(name, ".") {
		return true
	}
	if _, ok := policy.ExcludedDirNames[name]; ok {
		return true
	}
	if !policy.IncludeSystemByHint {
		cleanPath := filepath.Clean(path)
		for _, prefix := range policy.ExcludedRootPrefix {
			prefix = filepath.Clean(strings.TrimSpace(prefix))
			if prefix == "" || prefix == "." {
				continue
			}
			if cleanPath == prefix || strings.HasPrefix(cleanPath, prefix+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

func queryLooksGoRelated(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	for _, kw := range []string{"go ", " go", "golang", "go语言", "go项目", "go 项目", "go repo"} {
		if strings.Contains(q, kw) {
			return true
		}
	}
	return false
}

func tokenizeQuery(query string) []string {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return nil
	}
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return ' '
	}, q)
	parts := strings.Fields(normalized)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if utf8.RuneCountInString(p) < 2 {
			continue
		}
		out = append(out, p)
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func matchesQueryTokens(path string, tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	lower := strings.ToLower(path)
	for _, tok := range tokens {
		if strings.Contains(lower, tok) {
			return true
		}
	}
	return false
}

func hasProjectMarker(dir string, entries []os.DirEntry, preferGo bool, queryTokens []string) bool {
	if len(entries) == 0 {
		return false
	}
	hasGoMarker := false
	hasGenericMarker := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(e.Name()))
		switch name {
		case "go.mod", "go.work":
			hasGoMarker = true
			hasGenericMarker = true
		case "package.json", "pyproject.toml", "cargo.toml", "pom.xml", "build.gradle", "makefile":
			hasGenericMarker = true
		case ".git":
			hasGenericMarker = true
		}
	}
	if preferGo {
		if hasGoMarker {
			return true
		}
		if hasGenericMarker && matchesQueryTokens(dir, queryTokens) {
			return true
		}
		return false
	}
	if hasGenericMarker {
		return true
	}
	return matchesQueryTokens(dir, queryTokens)
}

func scanRootForCandidates(root string, query string, policy secretarySearchPolicy, deadline time.Time) ([]string, bool) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" {
		return nil, false
	}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return nil, false
	}

	preferGo := queryLooksGoRelated(query)
	tokens := tokenizeQuery(query)
	maxDepth := policy.MaxDepth
	if maxDepth <= 0 {
		maxDepth = triageSearchDefaultMaxDepth
	}
	maxVisits := policy.MaxVisitedDirs
	if maxVisits <= 0 {
		maxVisits = triageSearchDefaultMaxVisits
	}
	maxCandidates := policy.MaxCandidates
	if maxCandidates <= 0 {
		maxCandidates = triageSearchMaxCandidates
	}

	type node struct {
		path  string
		depth int
	}
	queue := []node{{path: root, depth: 0}}
	visited := 0
	candidates := make([]string, 0, maxCandidates)
	timeoutHit := false

	for len(queue) > 0 {
		if !deadline.IsZero() && time.Now().After(deadline) {
			timeoutHit = true
			break
		}

		n := queue[0]
		queue = queue[1:]
		visited++
		if visited > maxVisits {
			timeoutHit = true
			break
		}

		entries, err := os.ReadDir(n.path)
		if err != nil {
			continue
		}
		if hasProjectMarker(n.path, entries, preferGo, tokens) {
			candidates = append(candidates, n.path)
			if len(candidates) >= maxCandidates {
				break
			}
		}
		if n.depth >= maxDepth {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.TrimSpace(entry.Name())
			next := filepath.Join(n.path, name)
			if shouldSkipSearchDir(next, name, policy) {
				continue
			}
			queue = append(queue, node{path: next, depth: n.depth + 1})
		}
	}
	return candidates, timeoutHit
}

func executeLayeredReadonlySearch(policy secretarySearchPolicy, query string) secretarySearchSnapshot {
	query = strings.TrimSpace(query)
	if strings.TrimSpace(policy.DefaultReadonlySearchRoot) == "" {
		return secretarySearchSnapshot{}
	}

	timeoutPerPhase := policy.TimeoutPerPhase
	if timeoutPerPhase <= 0 {
		timeoutPerPhase = 1200 * time.Millisecond
	}

	type phase struct {
		name  string
		roots []string
	}
	phases := []phase{
		{name: "home", roots: dedupeAndExistingDirs(policy.Phase1HomeRoots)},
		{name: "common-dev", roots: dedupeAndExistingDirs(policy.Phase2CommonDevRoots)},
		{name: "whitelist", roots: dedupeAndExistingDirs(policy.Phase3WhitelistRoots)},
	}

	snap := secretarySearchSnapshot{
		Enabled: true,
		Query:   query,
	}

	// Honor explicit absolute paths first.
	explicit := extractAbsolutePathCandidates(query)
	if len(explicit) > 0 {
		snap.Candidates = append(snap.Candidates, explicit...)
		snap.FinalPhase = "explicit-path"
		snap.Expanded = false
		if len(snap.Candidates) > triageSearchMaxCandidates {
			snap.Omitted = len(snap.Candidates) - triageSearchMaxCandidates
			snap.Candidates = snap.Candidates[:triageSearchMaxCandidates]
		}
		return snap
	}

	for i, ph := range phases {
		if len(ph.roots) == 0 {
			continue
		}
		deadline := time.Now().Add(timeoutPerPhase)
		for _, root := range ph.roots {
			snap.ScannedRoots = append(snap.ScannedRoots, root)
			cands, timedOut := scanRootForCandidates(root, query, policy, deadline)
			if timedOut {
				snap.TimeoutHit = true
			}
			if len(cands) > 0 {
				snap.Candidates = append(snap.Candidates, cands...)
			}
			if len(snap.Candidates) >= triageSearchMaxCandidates {
				break
			}
			if snap.TimeoutHit {
				break
			}
		}
		snap.Candidates = dedupeAndSortPaths(snap.Candidates)
		if len(snap.Candidates) > 0 {
			snap.FinalPhase = ph.name
			snap.Expanded = i > 0
			break
		}
		if snap.TimeoutHit {
			snap.FinalPhase = ph.name
			snap.Expanded = i > 0
			break
		}
	}

	if snap.FinalPhase == "" {
		snap.FinalPhase = "home"
	}

	if len(snap.Candidates) > triageSearchMaxCandidates {
		snap.Omitted = len(snap.Candidates) - triageSearchMaxCandidates
		snap.Candidates = snap.Candidates[:triageSearchMaxCandidates]
	}
	return snap
}

func dedupeAndSortPaths(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		p := filepath.Clean(strings.TrimSpace(raw))
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func extractAbsolutePathCandidates(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := strings.Fields(text)
	cands := make([]string, 0, 2)
	for _, p := range parts {
		p = strings.Trim(p, "`'\"，。,.!?！？；;：:\\")
		if p == "" {
			continue
		}
		if isLikelyAbsolutePath(p) {
			cands = append(cands, p)
		}
	}
	return dedupeAndSortPaths(cands)
}

func isLikelyAbsolutePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if strings.HasPrefix(path, "/") {
		return true
	}
	if runtime.GOOS == "windows" {
		if len(path) >= 3 && ((path[1] == ':' && (path[2] == '\\' || path[2] == '/')) || strings.HasPrefix(path, "\\\\")) {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (o *Orchestrator) buildTriageCarryContext(msgs []model.ChatMessage, st State, cursor uint, newUserMsgs []model.ChatMessage, sessionWorkspace string) triageCarryContext {
	carry := triageCarryContext{}

	if len(st.TriageRuns) > 0 {
		last := st.TriageRuns[len(st.TriageRuns)-1]
		carry.PendingQuestions = normalizeCarryQuestions(last.Questions)
		carry.LastSummary = truncateString(strings.TrimSpace(last.SummaryMessage), triageCarrySummaryMaxRunes)
	}

	anchors, omitted := collectSemanticAnchors(msgs, cursor)
	carry.SemanticAnchors = anchors
	carry.OmittedAnchors = omitted
	carry.ShortFollowupLikelyAns = likelyShortFollowupAnswer(newUserMsgs, carry.PendingQuestions)

	sessionWorkspace = strings.TrimSpace(sessionWorkspace)
	if sessionWorkspace != "" {
		return carry
	}

	latest := ""
	if len(newUserMsgs) > 0 {
		latest = strings.TrimSpace(newUserMsgs[len(newUserMsgs)-1].Content)
	}
	if !isLikelyPathLookupQuestion(latest) && !(carry.ShortFollowupLikelyAns && hasWorkspaceBindingQuestion(carry.PendingQuestions)) {
		return carry
	}

	policy := defaultSecretarySearchPolicy()
	if strings.TrimSpace(policy.DefaultReadonlySearchRoot) == "" {
		return carry
	}
	carry.SearchPolicy = policy
	carry.SearchSnapshot = executeLayeredReadonlySearch(policy, latest)
	return carry
}

func (c triageCarryContext) SearchContextForRun() *SearchContext {
	if strings.TrimSpace(c.SearchPolicy.DefaultReadonlySearchRoot) == "" && !c.SearchSnapshot.Enabled {
		return nil
	}
	out := &SearchContext{
		DefaultRoot: strings.TrimSpace(c.SearchPolicy.DefaultReadonlySearchRoot),
		FinalPhase:  strings.TrimSpace(c.SearchSnapshot.FinalPhase),
		Expanded:    c.SearchSnapshot.Expanded,
		TimeoutHit:  c.SearchSnapshot.TimeoutHit,
		ScannedRoot: append([]string{}, c.SearchSnapshot.ScannedRoots...),
		Candidates:  append([]string{}, c.SearchSnapshot.Candidates...),
		Omitted:     c.SearchSnapshot.Omitted,
	}
	return out
}

func (c triageCarryContext) promptContextBlock() string {
	parts := make([]string, 0, 4)

	if len(c.PendingQuestions) > 0 || c.LastSummary != "" || len(c.SemanticAnchors) > 0 || c.OmittedAnchors > 0 {
		var b strings.Builder
		b.WriteString("## CONTINUITY_CONTEXT\n")
		if c.LastSummary != "" {
			b.WriteString("continuity_last_summary: ")
			b.WriteString(c.LastSummary)
			b.WriteString("\n")
		}
		if len(c.PendingQuestions) > 0 {
			b.WriteString("continuity_pending_questions:\n")
			for i, q := range c.PendingQuestions {
				b.WriteString(fmt.Sprintf("%d) %s\n", i+1, q))
			}
		}
		if c.ShortFollowupLikelyAns {
			b.WriteString("continuity_followup_hint: latest_user_reply_likely_answers_pending_questions\n")
		}
		if len(c.SemanticAnchors) > 0 {
			b.WriteString("continuity_recent_anchors:\n")
			for _, a := range c.SemanticAnchors {
				b.WriteString("- ")
				b.WriteString(a)
				b.WriteString("\n")
			}
		}
		if c.OmittedAnchors > 0 {
			b.WriteString(fmt.Sprintf("continuity_omitted_count: %d\n", c.OmittedAnchors))
		}
		parts = append(parts, strings.TrimSpace(b.String()))
	}

	if strings.TrimSpace(c.SearchPolicy.DefaultReadonlySearchRoot) != "" {
		var b strings.Builder
		b.WriteString("## READONLY_SEARCH_POLICY\n")
		b.WriteString("default_readonly_search_root: ")
		b.WriteString(c.SearchPolicy.DefaultReadonlySearchRoot)
		b.WriteString("\n")
		b.WriteString("search_phase_order: home -> common-dev -> whitelist\n")
		b.WriteString("search_exclude_hidden: true\n")
		b.WriteString("search_exclude_system_dirs: true\n")
		if c.SearchSnapshot.Enabled {
			b.WriteString("search_phase_result: ")
			b.WriteString(strings.TrimSpace(c.SearchSnapshot.FinalPhase))
			b.WriteString("\n")
			if c.SearchSnapshot.Expanded {
				b.WriteString("search_expanded_outside_home: true\n")
			}
			if c.SearchSnapshot.TimeoutHit {
				b.WriteString("search_timeout_hit: true\n")
			}
			if len(c.SearchSnapshot.ScannedRoots) > 0 {
				b.WriteString("search_scanned_roots:\n")
				for _, root := range c.SearchSnapshot.ScannedRoots {
					b.WriteString("- ")
					b.WriteString(root)
					b.WriteString("\n")
				}
			}
			if len(c.SearchSnapshot.Candidates) > 0 {
				b.WriteString("search_candidates:\n")
				for _, candidate := range c.SearchSnapshot.Candidates {
					b.WriteString("- ")
					b.WriteString(candidate)
					b.WriteString("\n")
				}
				if c.SearchSnapshot.Omitted > 0 {
					b.WriteString(fmt.Sprintf("search_candidates_omitted: %d\n", c.SearchSnapshot.Omitted))
				}
			}
		}
		parts = append(parts, strings.TrimSpace(b.String()))
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func (c triageCarryContext) withAnchorsDropped() triageCarryContext {
	if len(c.SemanticAnchors) == 0 {
		return c
	}
	next := c
	next.OmittedAnchors += len(next.SemanticAnchors)
	next.SemanticAnchors = nil
	return next
}
