package claudeagent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProcessBackendConfig configures the local process sandbox backend.
type ProcessBackendConfig struct {
	// TempDir is the base directory for sandbox working directories.
	// Defaults to os.TempDir().
	TempDir string
}

// ProcessBackend runs code as local subprocesses. It does NOT provide true
// isolation — suitable for development and testing only.
type ProcessBackend struct {
	tempDir  string
	mu       sync.Mutex
	sessions map[string]*processSession
	nextID   int
}

// NewProcessBackend creates a new process-based sandbox backend.
func NewProcessBackend(cfg ProcessBackendConfig) *ProcessBackend {
	dir := cfg.TempDir
	if dir == "" {
		dir = os.TempDir()
	}
	return &ProcessBackend{
		tempDir:  dir,
		sessions: make(map[string]*processSession),
	}
}

// CreateSession creates a new process-based sandbox session.
func (b *ProcessBackend) CreateSession(ctx context.Context, opts SessionOptions) (SandboxSession, error) {
	dir, err := os.MkdirTemp(b.tempDir, "sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	b.mu.Lock()
	b.nextID++
	id := fmt.Sprintf("sandbox_%d", b.nextID)
	sess := &processSession{
		id:       id,
		language: opts.Language,
		limits:   opts.Limits,
		workDir:  dir,
	}
	b.sessions[id] = sess
	b.mu.Unlock()

	return sess, nil
}

// Close cleans up all sessions.
func (b *ProcessBackend) Close() error {
	b.mu.Lock()
	sessions := make([]*processSession, 0, len(b.sessions))
	for _, s := range b.sessions {
		sessions = append(sessions, s)
	}
	b.sessions = make(map[string]*processSession)
	b.mu.Unlock()

	for _, s := range sessions {
		_ = s.Destroy(context.Background())
	}
	return nil
}

type processSession struct {
	id       string
	language Language
	limits   ResourceLimits
	workDir  string
}

func (s *processSession) ID() string { return s.id }

func (s *processSession) Execute(ctx context.Context, code string) (*ExecResult, error) {
	interpreter, args, err := s.interpreterCmd(code)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(s.limits.WallClockSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, interpreter, args...) // #nosec G204 -- sandbox intentionally executes user-provided code
	cmd.Dir = s.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// For languages that read from a file, write a temp file.
	tmpFile, err := s.writeCodeFile(code)
	if err != nil {
		return nil, err
	}
	if tmpFile != "" {
		// Replace the placeholder in args with the actual file path.
		for i, a := range cmd.Args {
			if a == "__CODE_FILE__" {
				cmd.Args[i] = tmpFile
			}
		}
		defer os.Remove(tmpFile) //nolint:errcheck // best-effort cleanup
	}

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	result := &ExecResult{
		Stdout:   truncateOutput(stdout.String(), s.limits.MaxOutputBytes),
		Stderr:   truncateOutput(stderr.String(), s.limits.MaxOutputBytes),
		Duration: duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = 124 // standard timeout exit code
	} else if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("execution error: %w", runErr)
		}
	}

	return result, nil
}

func (s *processSession) RunCommand(ctx context.Context, command string) (*ExecResult, error) {
	timeout := time.Duration(s.limits.WallClockSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command) // #nosec G204 -- sandbox intentionally executes user-provided commands
	cmd.Dir = s.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	result := &ExecResult{
		Stdout:   truncateOutput(stdout.String(), s.limits.MaxOutputBytes),
		Stderr:   truncateOutput(stderr.String(), s.limits.MaxOutputBytes),
		Duration: duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = 124
	} else if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("command error: %w", runErr)
		}
	}

	return result, nil
}

// safePath resolves a path within the sandbox working directory and ensures
// it does not escape via traversal (e.g., "../../etc/passwd").
func (s *processSession) safePath(path string) (string, error) {
	full := filepath.Join(s.workDir, path)
	cleanWork := filepath.Clean(s.workDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(full)+string(filepath.Separator), cleanWork) && filepath.Clean(full) != filepath.Clean(s.workDir) {
		return "", fmt.Errorf("path escapes sandbox: %s", path)
	}
	return full, nil
}

func (s *processSession) WriteFile(_ context.Context, path string, content []byte) error {
	full, err := s.safePath(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return os.WriteFile(full, content, 0600)
}

func (s *processSession) ReadFile(_ context.Context, path string) ([]byte, error) {
	full, err := s.safePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full) // #nosec G304 -- path is validated by safePath to stay within sandbox
}

func (s *processSession) ListFiles(_ context.Context, dir string) ([]SandboxFileInfo, error) {
	full, err := s.safePath(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}

	files := make([]SandboxFileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, SandboxFileInfo{
			Path:  e.Name(),
			Size:  info.Size(),
			IsDir: e.IsDir(),
		})
	}
	return files, nil
}

func (s *processSession) Destroy(_ context.Context) error {
	return os.RemoveAll(s.workDir)
}

// interpreterCmd returns the interpreter binary and arguments for executing code.
func (s *processSession) interpreterCmd(code string) (string, []string, error) {
	switch s.language {
	case LangPython:
		return "python3", []string{"-c", code}, nil
	case LangJavaScript:
		return "node", []string{"-e", code}, nil
	case LangBash:
		return "bash", []string{"-c", code}, nil
	case LangGo:
		// Go needs a file — use placeholder that gets replaced.
		return "go", []string{"run", "__CODE_FILE__"}, nil
	default:
		return "", nil, fmt.Errorf("unsupported language: %s", s.language)
	}
}

// writeCodeFile writes code to a temp file for languages that need it (Go).
func (s *processSession) writeCodeFile(code string) (string, error) {
	if s.language != LangGo {
		return "", nil
	}
	path := filepath.Join(s.workDir, "main.go")
	if err := os.WriteFile(path, []byte(code), 0600); err != nil {
		return "", fmt.Errorf("write code file: %w", err)
	}
	return path, nil
}

// truncateOutput truncates output to maxBytes, appending a truncation notice.
func truncateOutput(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	truncated := s[:maxBytes]
	// Try to truncate at a line boundary.
	if idx := strings.LastIndex(truncated, "\n"); idx > maxBytes/2 {
		truncated = truncated[:idx+1]
	}
	return truncated + fmt.Sprintf("\n... (truncated, %d bytes total)", len(s))
}
