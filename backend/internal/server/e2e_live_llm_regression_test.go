package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type liveLLMReport struct {
	Meta    liveLLMReportMeta    `json:"meta"`
	Summary liveLLMReportSummary `json:"summary"`
	Cases   []liveLLMCaseResult  `json:"cases"`
}

type liveLLMReportMeta struct {
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`

	RepoRoot string `json:"repo_root"`

	ProviderType string `json:"provider_type,omitempty"`
	EndpointHost string `json:"endpoint_host,omitempty"`
	Model        string `json:"model,omitempty"`

	SettingsDB string `json:"settings_db,omitempty"`

	Env struct {
		RunsPerCase int `json:"runs_per_case"`
		TimeoutSec  int `json:"timeout_sec"`
	} `json:"env"`
}

type liveLLMReportSummary struct {
	TotalRuns   int     `json:"total_runs"`
	SuccessRuns int     `json:"success_runs"`
	SuccessRate float64 `json:"success_rate"`

	ByProtocol map[string]liveLLMSummaryBucket `json:"by_protocol"`
	ByCase     map[string]liveLLMSummaryBucket `json:"by_case"`
}

type liveLLMSummaryBucket struct {
	Total   int     `json:"total"`
	Success int     `json:"success"`
	Rate    float64 `json:"rate"`

	DurationMsP50 int64 `json:"duration_ms_p50,omitempty"`
	DurationMsP90 int64 `json:"duration_ms_p90,omitempty"`
	FirstTokenMsP50 int64 `json:"first_token_ms_p50,omitempty"`
	FirstTokenMsP90 int64 `json:"first_token_ms_p90,omitempty"`
}

type liveLLMCaseResult struct {
	CaseName  string `json:"case_name"`
	Protocol  string `json:"protocol"`
	Attempt   int    `json:"attempt"`
	SessionID string `json:"session_id"`
	Workspace string `json:"workspace"`

	OK bool `json:"ok"`

	DurationMs   int64 `json:"duration_ms"`
	FirstTokenMs int64 `json:"first_token_ms,omitempty"`

	ToolCalls   int `json:"tool_calls"`
	ToolResults int `json:"tool_results"`

	VerifiedFile  string `json:"verified_file,omitempty"`
	VerifiedRunes int    `json:"verified_runes,omitempty"`
	VerifiedBytes int    `json:"verified_bytes,omitempty"`

	AssistantTextSHA256 string `json:"assistant_text_sha256,omitempty"`
	AssistantTextChars  int    `json:"assistant_text_chars,omitempty"`

	Error string `json:"error,omitempty"`
}

func TestE2E_LiveLLM_RegressionSuite(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ONEAGENT_LIVE_LLM")) != "1" {
		t.Skip("set ONEAGENT_LIVE_LLM=1 to enable live provider regression suite")
	}

	runsPerCase := getenvInt("ONEAGENT_LIVE_LLM_RUNS_PER_CASE", 6)
	if runsPerCase <= 0 {
		runsPerCase = 1
	}
	timeoutSec := getenvInt("ONEAGENT_LIVE_LLM_TIMEOUT_SEC", 90)
	if timeoutSec <= 0 {
		timeoutSec = 90
	}
	caseTimeout := time.Duration(timeoutSec) * time.Second

	repoRoot := findRepoRoot(t)
	srcSettingsDB := filepath.Join(repoRoot, ".oneagent", "settings.db")
	if override := strings.TrimSpace(os.Getenv("ONEAGENT_LIVE_LLM_SETTINGS_DB")); override != "" {
		srcSettingsDB = override
		repoRoot = filepath.Dir(filepath.Dir(override))
	}
	if _, err := os.Stat(srcSettingsDB); err != nil {
		t.Skipf("missing settings db at %s", srcSettingsDB)
	}

	home := t.TempDir()
	copySettingsDBToHome(t, srcSettingsDB, home)

	prevCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prevCfg })

	cfg, err := config.Load(config.LoadOptions{
		Home:     home,
		Profile:  "dev",
		AuthMode: "none",
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	provider, model, ok := resolveDefaultProviderAndModel(t, rt.Settings)
	if !ok {
		t.Skip("no default provider/model configured for user=local in settings db")
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	startedAt := time.Now()
	report := liveLLMReport{
		Meta: liveLLMReportMeta{
			StartedAt:    startedAt.UTC().Format(time.RFC3339),
			RepoRoot:     repoRoot,
			ProviderType: provider.ProviderType,
			EndpointHost: endpointHost(provider.BaseURL),
			Model:        model.Model,
			SettingsDB:   srcSettingsDB,
		},
	}
	report.Meta.Env.RunsPerCase = runsPerCase
	report.Meta.Env.TimeoutSec = timeoutSec

	// Keep the payload ~3000 runes but within the single-call write_file limit to avoid truncation-by-design.
	longPayload := buildLongTextPayload(2900)

	baseCases := []struct {
		name     string
		tools    []string
		buildMsg func() string
		verify   func(workspace string, out *liveLLMCaseResult) error
	}{
		{
			name:  "short_write_file",
			tools: []string{tool.ToolIDWriteFile},
			buildMsg: func() string {
				return strings.TrimSpace(`
请调用 write_file 工具，在 workspace 根目录创建 hello.txt，内容严格等于：hello
完成后你可以简单回复 OK。`)
			},
			verify: func(workspace string, out *liveLLMCaseResult) error {
				p := filepath.Join(workspace, "hello.txt")
				b, err := os.ReadFile(p)
				if err != nil {
					return fmt.Errorf("read hello.txt: %w", err)
				}
				if strings.TrimSpace(string(b)) != "hello" {
					return fmt.Errorf("hello.txt content mismatch: %q", string(b))
				}
				if out != nil {
					out.VerifiedFile = "hello.txt"
					out.VerifiedBytes = len(b)
					out.VerifiedRunes = utf8RuneCount(string(b))
				}
				return nil
			},
		},
		{
			name:  "long_write_file_3000ish_runes",
			tools: []string{tool.ToolIDWriteFile},
			buildMsg: func() string {
				return fmt.Sprintf(strings.TrimSpace(`
请调用 write_file 工具，在 workspace 根目录创建 long.txt。

要求：
1) 只允许调用一次 write_file（append=false，不要分段）
2) filePath 必须是 long.txt（不要加任何前缀：不要用 /long.txt 或 /workspace/long.txt）
3) 内容已控制在单次 write_file 限制内，可以一次写完；不要为了“省 token”自行缩短
4) 内容必须严格等于下面 BEGIN_PAYLOAD 与 END_PAYLOAD 之间的文本（不要改动任何字符，不要增删空格或换行）

BEGIN_PAYLOAD
%s
END_PAYLOAD

完成后你可以简单回复 OK。`), longPayload)
			},
			verify: func(workspace string, out *liveLLMCaseResult) error {
				p := filepath.Join(workspace, "long.txt")
				b, err := os.ReadFile(p)
				if err != nil {
					return fmt.Errorf("read long.txt: %w", err)
				}
				txt := string(b)
				runes := utf8RuneCount(txt)
				if runes < 2600 {
					return fmt.Errorf("long.txt too short: %d runes", runes)
				}
				for _, needle := range []string{"第1段:", "<tag>hello</tag>", "（中文）"} {
					if !strings.Contains(txt, needle) {
						return fmt.Errorf("long.txt missing required content: %q", needle)
					}
				}
				if out != nil {
					out.VerifiedFile = "long.txt"
					out.VerifiedBytes = len(b)
					out.VerifiedRunes = runes
				}
				return nil
			},
		},
	}

	protocols := []string{"json", "xml"}

	for _, proto := range protocols {
		for _, bc := range baseCases {
			for attempt := 1; attempt <= runsPerCase; attempt++ {
				ws := t.TempDir()
				sessionID := fmt.Sprintf("live-%s-%s-%d-%d", proto, bc.name, attempt, time.Now().UnixNano())

				reqBody := map[string]any{
					"message":       bc.buildMsg(),
					"session_id":    sessionID,
					"tool_protocol": proto,
					"tool_ids":      bc.tools,
					"workspace":     ws,
				}

				res := runLiveChatCase(t, srv.URL, reqBody, caseTimeout)
				res.CaseName = bc.name
				res.Protocol = proto
				res.Attempt = attempt
				res.SessionID = sessionID
				res.Workspace = ws

				if res.OK {
					if err := bc.verify(ws, &res); err != nil {
						res.OK = false
						res.Error = err.Error()
					}
				}

				report.Cases = append(report.Cases, res)
			}
		}
	}

	finishedAt := time.Now()
	report.Meta.FinishedAt = finishedAt.UTC().Format(time.RFC3339)
	report.Summary = summarizeLiveReport(report.Cases)

	reportDir := filepath.Join(repoRoot, ".oneagent", "tmp")
	_ = os.MkdirAll(reportDir, 0o700)
	stamp := finishedAt.UTC().Format("20060102_150405")
	jsonPath := filepath.Join(reportDir, "live_llm_regression_"+stamp+".json")
	mdPath := filepath.Join(reportDir, "live_llm_regression_"+stamp+".md")

	if err := writeLiveReportJSON(jsonPath, report); err != nil {
		t.Fatalf("write report json: %v", err)
	}
	if err := writeLiveReportMarkdown(mdPath, report, jsonPath); err != nil {
		t.Fatalf("write report md: %v", err)
	}
	t.Logf("live llm regression report written: %s", jsonPath)

	if report.Summary.TotalRuns == 0 {
		t.Fatalf("no runs executed")
	}
	if report.Summary.SuccessRate < 0.9 {
		t.Fatalf("success rate too low: %.2f (see %s)", report.Summary.SuccessRate, jsonPath)
	}
}

func getenvInt(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	start := dir
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".oneagent", "settings.db")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Fatalf("could not locate repo root with .oneagent/settings.db from %s", start)
	return ""
}

func copySettingsDBToHome(t *testing.T, srcSettingsDB, home string) {
	t.Helper()
	dstDir := filepath.Join(home, ".oneagent")
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		t.Fatalf("mkdir temp .oneagent: %v", err)
	}
	copyFile(t, filepath.Join(dstDir, "settings.db"), srcSettingsDB)

	for _, suffix := range []string{"-wal", "-shm"} {
		src := srcSettingsDB + suffix
		if _, err := os.Stat(src); err == nil {
			copyFile(t, filepath.Join(dstDir, "settings.db"+suffix), src)
		}
	}
}

func copyFile(t *testing.T, dst, src string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
	}
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}

func resolveDefaultProviderAndModel(t *testing.T, db *settingsdb.DB) (settingsdb.Provider, settingsdb.Model, bool) {
	t.Helper()
	if db == nil {
		return settingsdb.Provider{}, settingsdb.Model{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	providers, err := db.ListProviders(ctx, "local")
	if err != nil || len(providers) == 0 {
		return settingsdb.Provider{}, settingsdb.Model{}, false
	}

	// Prefer provider that owns the default model.
	for _, p := range providers {
		models, err := db.ListModels(ctx, "local", p.ID)
		if err != nil {
			continue
		}
		for _, m := range models {
			if m.IsDefault {
				return p, m, true
			}
		}
	}

	// Fallback: first provider + first model.
	p := providers[0]
	models, err := db.ListModels(ctx, "local", p.ID)
	if err != nil || len(models) == 0 {
		return settingsdb.Provider{}, settingsdb.Model{}, false
	}
	return p, models[0], true
}

func endpointHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	if i := strings.IndexByte(raw, '/'); i >= 0 {
		raw = raw[:i]
	}
	return raw
}

func buildLongTextPayload(minRunes int) string {
	pattern := "第1段: \"quoted\" \\\\ path \\n lineA\nlineB\n<tag>hello</tag> </div> end\n（中文）\n"
	per := utf8RuneCount(pattern)
	repeats := 1
	if per > 0 && minRunes > per {
		repeats = (minRunes + per - 1) / per
	}
	content := strings.Repeat(pattern, repeats)
	content = strings.ReplaceAll(content, "]]>", "]] ]>")

	payload := "ONEAGENT_LONGTEXT_BEGIN\n" + content + "\nONEAGENT_LONGTEXT_END"
	return payload
}

func utf8RuneCount(s string) int {
	return len([]rune(s))
}

type liveChatParse struct {
	assistantText strings.Builder
	firstTokenAt  time.Time
	toolCalls     int
	toolResults   int
	streamErr     string
}

func runLiveChatCase(t *testing.T, baseURL string, reqBody any, timeout time.Duration) liveLLMCaseResult {
	t.Helper()

	data, err := json.Marshal(reqBody)
	if err != nil {
		return liveLLMCaseResult{OK: false, Error: "marshal request: " + err.Error()}
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return liveLLMCaseResult{OK: false, Error: "new request: " + err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return liveLLMCaseResult{OK: false, Error: "do request: " + err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return liveLLMCaseResult{
			OK:    false,
			Error: fmt.Sprintf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b))),
		}
	}

	parsed := parseChatStream(t, resp.Body, start)
	duration := time.Since(start)

	out := liveLLMCaseResult{
		OK:          parsed.streamErr == "",
		DurationMs:  duration.Milliseconds(),
		ToolCalls:   parsed.toolCalls,
		ToolResults: parsed.toolResults,
	}
	if !parsed.firstTokenAt.IsZero() {
		out.FirstTokenMs = parsed.firstTokenAt.Sub(start).Milliseconds()
	}

	assistant := parsed.assistantText.String()
	out.AssistantTextChars = utf8RuneCount(assistant)
	if strings.TrimSpace(assistant) != "" {
		sum := sha256.Sum256([]byte(assistant))
		out.AssistantTextSHA256 = hex.EncodeToString(sum[:])
	}

	if parsed.streamErr != "" {
		out.Error = parsed.streamErr
	}

	return out
}

func parseChatStream(t *testing.T, body io.Reader, start time.Time) liveChatParse {
	t.Helper()

	var out liveChatParse
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 20*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			var evt streamEvent
			if err := json.Unmarshal([]byte(payload), &evt); err != nil {
				continue
			}

			switch evt.Type {
			case "error":
				if out.streamErr == "" {
					out.streamErr = strings.TrimSpace(evt.Data)
				}
				continue
			case "msg":
				var msg streamMsg
				if err := json.Unmarshal([]byte(evt.Data), &msg); err != nil {
					continue
				}
				if msg.Error != "" && out.streamErr == "" {
					out.streamErr = strings.TrimSpace(msg.Error)
				}
				switch msg.MsgType {
				case "text":
					if msg.Role == "assistant" && msg.Op == "delta" && msg.Delta != "" {
						if out.firstTokenAt.IsZero() {
							out.firstTokenAt = time.Now()
						}
						out.assistantText.WriteString(msg.Delta)
					}
				case "tool_call":
					if msg.Op == "final" {
						out.toolCalls++
					}
				case "tool_result":
					if msg.Op == "final" {
						out.toolResults++
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if out.streamErr == "" {
			out.streamErr = "scan stream: " + err.Error()
		}
	}

	// If no events and no error, treat as error: likely a hang or parse mismatch.
	if out.firstTokenAt.IsZero() && out.toolCalls == 0 && out.toolResults == 0 && out.streamErr == "" {
		out.streamErr = fmt.Sprintf("no sse events observed (elapsed=%s)", time.Since(start).String())
	}

	return out
}

func summarizeLiveReport(cases []liveLLMCaseResult) liveLLMReportSummary {
	summary := liveLLMReportSummary{
		ByProtocol: map[string]liveLLMSummaryBucket{},
		ByCase:     map[string]liveLLMSummaryBucket{},
	}
	if len(cases) == 0 {
		return summary
	}

	for _, c := range cases {
		summary.TotalRuns++
		if c.OK {
			summary.SuccessRuns++
		}

		bp := summary.ByProtocol[c.Protocol]
		bp.Total++
		if c.OK {
			bp.Success++
		}
		summary.ByProtocol[c.Protocol] = bp

		bc := summary.ByCase[c.CaseName]
		bc.Total++
		if c.OK {
			bc.Success++
		}
		summary.ByCase[c.CaseName] = bc
	}

	if summary.TotalRuns > 0 {
		summary.SuccessRate = float64(summary.SuccessRuns) / float64(summary.TotalRuns)
	}

	fillBuckets := func(selector func(liveLLMCaseResult) string, buckets map[string]liveLLMSummaryBucket) {
		durByKey := map[string][]int64{}
		ftByKey := map[string][]int64{}
		for _, c := range cases {
			key := selector(c)
			durByKey[key] = append(durByKey[key], c.DurationMs)
			if c.FirstTokenMs > 0 {
				ftByKey[key] = append(ftByKey[key], c.FirstTokenMs)
			}
		}

		for key, bucket := range buckets {
			bucket.Rate = 0
			if bucket.Total > 0 {
				bucket.Rate = float64(bucket.Success) / float64(bucket.Total)
			}

			bucket.DurationMsP50 = percentileMs(durByKey[key], 0.50)
			bucket.DurationMsP90 = percentileMs(durByKey[key], 0.90)
			bucket.FirstTokenMsP50 = percentileMs(ftByKey[key], 0.50)
			bucket.FirstTokenMsP90 = percentileMs(ftByKey[key], 0.90)

			buckets[key] = bucket
		}
	}

	fillBuckets(func(c liveLLMCaseResult) string { return c.Protocol }, summary.ByProtocol)
	fillBuckets(func(c liveLLMCaseResult) string { return c.CaseName }, summary.ByCase)

	return summary
}

func percentileMs(values []int64, q float64) int64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]int64(nil), values...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	if q <= 0 {
		return cp[0]
	}
	if q >= 1 {
		return cp[len(cp)-1]
	}
	pos := int(float64(len(cp)-1) * q)
	if pos < 0 {
		pos = 0
	}
	if pos >= len(cp) {
		pos = len(cp) - 1
	}
	return cp[pos]
}

func writeLiveReportJSON(path string, report liveLLMReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func writeLiveReportMarkdown(path string, report liveLLMReport, jsonPath string) error {
	var b strings.Builder
	b.WriteString("# Live LLM Regression Report\n\n")
	b.WriteString(fmt.Sprintf("- Started: %s\n", report.Meta.StartedAt))
	b.WriteString(fmt.Sprintf("- Finished: %s\n", report.Meta.FinishedAt))
	b.WriteString(fmt.Sprintf("- Provider: %s (%s)\n", report.Meta.ProviderType, report.Meta.EndpointHost))
	b.WriteString(fmt.Sprintf("- Model: %s\n", report.Meta.Model))
	b.WriteString(fmt.Sprintf("- Runs per case: %d\n", report.Meta.Env.RunsPerCase))
	b.WriteString(fmt.Sprintf("- Timeout per run: %ds\n", report.Meta.Env.TimeoutSec))
	b.WriteString(fmt.Sprintf("- JSON report: %s\n\n", jsonPath))

	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- Total runs: %d\n", report.Summary.TotalRuns))
	b.WriteString(fmt.Sprintf("- Success runs: %d\n", report.Summary.SuccessRuns))
	b.WriteString(fmt.Sprintf("- Success rate: %.2f\n\n", report.Summary.SuccessRate))

	writeBucketTable := func(title string, buckets map[string]liveLLMSummaryBucket) {
		b.WriteString("## " + title + "\n\n")
		b.WriteString("| Key | Total | Success | Rate | P50(ms) | P90(ms) | FT P50(ms) | FT P90(ms) |\n")
		b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")

		keys := make([]string, 0, len(buckets))
		for k := range buckets {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := buckets[k]
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %.2f | %d | %d | %d | %d |\n",
				k, v.Total, v.Success, v.Rate,
				v.DurationMsP50, v.DurationMsP90,
				v.FirstTokenMsP50, v.FirstTokenMsP90,
			))
		}
		b.WriteString("\n")
	}

	writeBucketTable("By Protocol", report.Summary.ByProtocol)
	writeBucketTable("By Case", report.Summary.ByCase)

	return os.WriteFile(path, []byte(b.String()), 0o600)
}
