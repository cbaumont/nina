package state

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cbaumont/nina/internal/llm"
)

const dirName = ".nina"

type Session struct {
	SessionID string         `json:"session_id"`
	Goal      string         `json:"goal"`
	State     string         `json:"state"`
	PlanTitle string         `json:"plan_title"`
	Steps     []llm.PlanStep `json:"steps"`
	StepIndex int            `json:"step_index"`
	Snapshots int            `json:"snapshots"`
	LastRef   string         `json:"last_ref"`
}

func Save(workspaceDir string, sess *Session, messages []llm.Message) error {
	dir := filepath.Join(workspaceDir, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, "session.json"), raw); err != nil {
		return err
	}
	var transcript bytes.Buffer
	for _, message := range messages {
		line, err := json.Marshal(message)
		if err != nil {
			return err
		}
		transcript.Write(line)
		transcript.WriteByte('\n')
	}
	return writeAtomic(filepath.Join(dir, "transcript.jsonl"), transcript.Bytes())
}

func Load(workspaceDir string) (*Session, []llm.Message, error) {
	raw, err := os.ReadFile(filepath.Join(workspaceDir, dirName, "session.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	sess := &Session{}
	if err := json.Unmarshal(raw, sess); err != nil {
		return nil, nil, fmt.Errorf("reading session.json: %w", err)
	}
	messages, err := loadTranscript(filepath.Join(workspaceDir, dirName, "transcript.jsonl"))
	if err != nil {
		return nil, nil, err
	}
	return sess, messages, nil
}

func loadTranscript(path string) ([]llm.Message, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var messages []llm.Message
	scanner := bufio.NewScanner(file)
	scanner.Buffer(nil, 4*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var message llm.Message
		if err := json.Unmarshal(line, &message); err != nil {
			return nil, fmt.Errorf("reading transcript.jsonl: %w", err)
		}
		messages = append(messages, message)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func SaveHistory(workspaceDir, text string) error {
	dir := filepath.Join(workspaceDir, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "history.md"), []byte(text))
}

func LoadHistory(workspaceDir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(workspaceDir, dirName, "history.md"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
