package taskqueue

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/liu_y/oneAgent/backend/internal/scope"
)

type Store struct {
	tasksDir string
	muByTask sync.Map // map[string]*sync.Mutex

	govMu      sync.Mutex
	govCache   QueueGovernance
	govCacheAt time.Time
}

type taskFile struct {
	Version int  `json:"version"`
	Task    Task `json:"task"`
}

// NewID generates unique identifiers; overrideable in tests.
var NewID = uuid.NewString

func NewStore(tasksDir string) (*Store, error) {
	if strings.TrimSpace(tasksDir) == "" {
		return nil, errors.New("tasksDir is required")
	}
	if err := os.MkdirAll(tasksDir, 0o700); err != nil {
		return nil, fmt.Errorf("create tasks dir: %w", err)
	}
	return &Store{tasksDir: tasksDir}, nil
}

func (s *Store) TasksDir() string {
	return s.tasksDir
}

func (s *Store) taskDir(taskID string) string {
	return filepath.Join(s.tasksDir, taskID)
}

func (s *Store) taskPath(taskID string) string {
	return filepath.Join(s.taskDir(taskID), "task.json")
}

func (s *Store) eventsPath(taskID string) string {
	return filepath.Join(s.taskDir(taskID), "events.jsonl")
}

func (s *Store) lock(taskID string) *sync.Mutex {
	val, _ := s.muByTask.LoadOrStore(taskID, &sync.Mutex{})
	return val.(*sync.Mutex)
}

func (s *Store) CreateTask(userID, workspace, title, prompt, modelID string, limits Limits) (Task, error) {
	if strings.TrimSpace(userID) == "" {
		return Task{}, errors.New("userID is required")
	}
	if strings.TrimSpace(workspace) == "" {
		return Task{}, errors.New("workspace is required")
	}
	normalizedWorkspace, err := scope.NormalizeWorkspaceRoot(workspace)
	if err != nil {
		return Task{}, err
	}
	if strings.TrimSpace(prompt) == "" {
		return Task{}, errors.New("prompt is required")
	}

	now := Now()
	taskID := NewID()
	attemptID := NewID()

	task := Task{
		ID:        taskID,
		UserID:    userID,
		Workspace: normalizedWorkspace,
		Title:     strings.TrimSpace(title),
		Prompt:    prompt,
		ModelID:   strings.TrimSpace(modelID),
		Limits:    limits,
		CreatedAt: now,
		UpdatedAt: now,
		Attempts: []Attempt{{
			ID:          attemptID,
			Status:      AttemptQueued,
			CreatedAt:   now,
			PrincipalID: userID,
		}},
	}

	mu := s.lock(taskID)
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(s.taskDir(taskID), 0o700); err != nil {
		return Task{}, fmt.Errorf("create task dir: %w", err)
	}

	if err := s.writeTaskLocked(task); err != nil {
		return Task{}, err
	}

	if err := touchFile(s.eventsPath(taskID), 0o600); err != nil {
		return Task{}, err
	}

	if err := s.appendEventLocked(Event{
		TS:        now,
		TaskID:    taskID,
		AttemptID: attemptID,
		Type:      "task.created",
		Message:   "Task created",
	}); err != nil {
		return Task{}, err
	}

	return task, nil
}

func (s *Store) CreateTaskWithID(taskID string, userID, workspace, title, prompt, modelID string, limits Limits) (Task, bool, error) {
	if strings.TrimSpace(taskID) == "" {
		return Task{}, false, errors.New("taskID is required")
	}
	if strings.TrimSpace(userID) == "" {
		return Task{}, false, errors.New("userID is required")
	}
	if strings.TrimSpace(workspace) == "" {
		return Task{}, false, errors.New("workspace is required")
	}
	normalizedWorkspace, err := scope.NormalizeWorkspaceRoot(workspace)
	if err != nil {
		return Task{}, false, err
	}
	if strings.TrimSpace(prompt) == "" {
		return Task{}, false, errors.New("prompt is required")
	}

	now := Now()

	mu := s.lock(taskID)
	mu.Lock()
	defer mu.Unlock()

	if _, err := os.Stat(s.taskPath(taskID)); err == nil {
		task, err := s.loadTaskLocked(taskID)
		if err != nil {
			return Task{}, false, err
		}
		return task, false, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Task{}, false, err
	}

	attemptID := NewID()
	task := Task{
		ID:        taskID,
		UserID:    userID,
		Workspace: normalizedWorkspace,
		Title:     strings.TrimSpace(title),
		Prompt:    prompt,
		ModelID:   strings.TrimSpace(modelID),
		Limits:    limits,
		CreatedAt: now,
		UpdatedAt: now,
		Attempts: []Attempt{{
			ID:          attemptID,
			Status:      AttemptQueued,
			CreatedAt:   now,
			PrincipalID: userID,
		}},
	}

	if err := os.MkdirAll(s.taskDir(taskID), 0o700); err != nil {
		return Task{}, false, fmt.Errorf("create task dir: %w", err)
	}

	if err := s.writeTaskLocked(task); err != nil {
		return Task{}, false, err
	}

	if err := touchFile(s.eventsPath(taskID), 0o600); err != nil {
		return Task{}, false, err
	}

	if err := s.appendEventLocked(Event{
		TS:        now,
		TaskID:    taskID,
		AttemptID: attemptID,
		Type:      "task.created",
		Message:   "Task created",
	}); err != nil {
		return Task{}, false, err
	}

	return task, true, nil
}

func (s *Store) GetTask(taskID string) (Task, error) {
	if strings.TrimSpace(taskID) == "" {
		return Task{}, errors.New("taskID is required")
	}
	mu := s.lock(taskID)
	mu.Lock()
	defer mu.Unlock()
	return s.loadTaskLocked(taskID)
}

func (s *Store) ListTasks(userID, workspace string) ([]Task, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace != "" {
		normalized, err := scope.NormalizeWorkspaceRoot(workspace)
		if err != nil {
			return nil, err
		}
		workspace = normalized
	}

	entries, err := os.ReadDir(s.tasksDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Task{}, nil
		}
		return nil, err
	}

	tasks := make([]Task, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		taskID := e.Name()
		mu := s.lock(taskID)
		mu.Lock()
		task, err := s.loadTaskLocked(taskID)
		mu.Unlock()
		if err != nil {
			continue
		}
		if userID != "" && task.UserID != userID {
			continue
		}
		if workspace != "" && task.Workspace != workspace {
			continue
		}
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})
	return tasks, nil
}

func (s *Store) UpdateTask(taskID string, fn func(*Task) error) (Task, error) {
	if strings.TrimSpace(taskID) == "" {
		return Task{}, errors.New("taskID is required")
	}
	if fn == nil {
		return Task{}, errors.New("update function is required")
	}

	mu := s.lock(taskID)
	mu.Lock()
	defer mu.Unlock()

	task, err := s.loadTaskLocked(taskID)
	if err != nil {
		return Task{}, err
	}

	if err := fn(&task); err != nil {
		return Task{}, err
	}

	task.UpdatedAt = Now()
	if err := s.writeTaskLocked(task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Store) AppendEvent(ev Event) error {
	if strings.TrimSpace(ev.TaskID) == "" {
		return errors.New("event task_id is required")
	}
	if strings.TrimSpace(ev.Type) == "" {
		return errors.New("event type is required")
	}
	if ev.TS.IsZero() {
		ev.TS = Now()
	}

	taskID := ev.TaskID
	mu := s.lock(taskID)
	mu.Lock()
	defer mu.Unlock()
	return s.appendEventLocked(ev)
}

func (s *Store) appendEventLocked(ev Event) error {
	if ev.TS.IsZero() {
		ev.TS = Now()
	}
	path := s.eventsPath(ev.TaskID)
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

func (s *Store) ReadEvents(taskID string) ([]Event, error) {
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("taskID is required")
	}
	mu := s.lock(taskID)
	mu.Lock()
	defer mu.Unlock()

	path := s.eventsPath(taskID)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Event{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) loadTaskLocked(taskID string) (Task, error) {
	data, err := os.ReadFile(s.taskPath(taskID))
	if err != nil {
		return Task{}, err
	}

	var tf taskFile
	if err := json.Unmarshal(data, &tf); err == nil && tf.Task.ID != "" {
		return tf.Task, nil
	}

	var t Task
	if err := json.Unmarshal(data, &t); err != nil {
		return Task{}, err
	}
	return t, nil
}

func (s *Store) writeTaskLocked(task Task) error {
	if strings.TrimSpace(task.ID) == "" {
		return errors.New("task id is required")
	}
	tf := taskFile{
		Version: 1,
		Task:    task,
	}
	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWriteFile(s.taskPath(task.ID), data, 0o600)
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func touchFile(path string, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE, mode)
	if err != nil {
		return err
	}
	return f.Close()
}
