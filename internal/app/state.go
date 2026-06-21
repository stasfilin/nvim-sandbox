package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	base string
}

func NewState() (*State, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return &State{base: filepath.Join(base, "nvim-sandbox")}, nil
}

func (s *State) Base() string {
	return s.base
}

func (s *State) Ensure() error {
	for _, dir := range []string{s.projectsDir(), s.decisionsDir(), s.logsDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (s *State) projectsDir() string {
	return filepath.Join(s.base, "projects")
}

func (s *State) decisionsDir() string {
	return filepath.Join(s.base, "decisions")
}

func (s *State) logsDir() string {
	return filepath.Join(s.base, "logs")
}

func (s *State) projectPath(id string) string {
	return filepath.Join(s.projectsDir(), id+".json")
}

func (s *State) decisionPath(id string) string {
	return filepath.Join(s.decisionsDir(), id+".json")
}

func (s *State) logPath(id string) string {
	return filepath.Join(s.logsDir(), id+".log")
}

func readJSON[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func writeJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(dir, ".nvim-sandbox-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func (s *State) ReadDecision(id string) (*Decision, error) {
	return readJSON[Decision](s.decisionPath(id))
}

func (s *State) WriteDecision(ctx Context, value string) (*Decision, error) {
	decision := Decision{
		WorkspaceID:      ctx.WorkspaceID,
		WorkspaceIDShort: ctx.WorkspaceIDShort,
		ProjectRoot:      ctx.ProjectRoot,
		Decision:         value,
		UpdatedAt:        now(),
	}
	return &decision, writeJSON(s.decisionPath(ctx.WorkspaceID), decision)
}

func (s *State) DeleteDecision(id string) error {
	if err := os.Remove(s.decisionPath(id)); errors.Is(err, os.ErrNotExist) {
		return nil
	} else {
		return err
	}
}

func (s *State) ReadProject(id string) (*Metadata, error) {
	path := s.projectPath(id)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	if _, ok := raw["network"]; !ok {
		metadata.Network = Network{Enabled: true, Ports: []string{}}
	}
	if metadata.Network.Ports == nil {
		metadata.Network.Ports = []string{}
	}
	if metadata.ExtraMounts == nil {
		metadata.ExtraMounts = []Mount{}
	}
	return &metadata, nil
}

func (s *State) WriteProject(ctx Context, metadata Metadata) (*Metadata, error) {
	metadata.WorkspaceID = ctx.WorkspaceID
	metadata.WorkspaceIDShort = ctx.WorkspaceIDShort
	metadata.ProjectRoot = ctx.ProjectRoot
	metadata.ContainerName = ctx.ContainerName
	if metadata.Network.Ports == nil {
		metadata.Network.Ports = []string{}
	}
	if metadata.ExtraMounts == nil {
		metadata.ExtraMounts = []Mount{}
	}
	return &metadata, writeJSON(s.projectPath(ctx.WorkspaceID), metadata)
}

func (s *State) DeleteProject(id string) error {
	if err := os.Remove(s.projectPath(id)); errors.Is(err, os.ErrNotExist) {
		return nil
	} else {
		return err
	}
}

func (s *State) ReadLog(id string) (string, error) {
	data, err := os.ReadFile(s.logPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
