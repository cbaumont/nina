package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

type Level string

const (
	LevelNone         Level = "none"
	LevelBeginner     Level = "beginner"
	LevelIntermediate Level = "intermediate"
	LevelProfessional Level = "professional"
)

type HintSpeed string

const (
	HintSlow   HintSpeed = "slow"
	HintMedium HintSpeed = "medium"
	HintFast   HintSpeed = "fast"
)

// Profile configures how Nina teaches: how much foundational explanation
// to give, how the target stack relates to what the user knows, how much
// Nina may type (the dial, a ceiling not a target), and how quickly hints
// escalate during review.
type Profile struct {
	Experience     Level     `json:"experience"`
	StackFamiliar  Level     `json:"stack_familiarity"`
	KnownStacks    []string  `json:"known_stacks,omitempty"`
	Dial           int       `json:"dial"`
	HintEscalation HintSpeed `json:"hint_escalation"`
}

func Default() Profile {
	return Profile{
		Experience:     LevelBeginner,
		StackFamiliar:  LevelBeginner,
		Dial:           1,
		HintEscalation: HintMedium,
	}
}

func ParseLevel(s string) (Level, error) {
	level := Level(s)
	if slices.Contains([]Level{LevelNone, LevelBeginner, LevelIntermediate, LevelProfessional}, level) {
		return level, nil
	}
	return "", fmt.Errorf("experience must be none, beginner, intermediate, or professional")
}

func ParseHintSpeed(s string) (HintSpeed, error) {
	speed := HintSpeed(s)
	if slices.Contains([]HintSpeed{HintSlow, HintMedium, HintFast}, speed) {
		return speed, nil
	}
	return "", fmt.Errorf("hint escalation must be slow, medium, or fast")
}

func ParseDial(s string) (int, error) {
	dial, err := strconv.Atoi(s)
	if err != nil || dial < 0 || dial > 3 {
		return 0, fmt.Errorf("the dial goes from 0 (full manual) to 3 (collaborative)")
	}
	return dial, nil
}

func projectPath(workspaceDir string) string {
	return filepath.Join(workspaceDir, ".nina", "profile.json")
}

func globalPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "nina", "profile.json"), nil
}

// Load returns the profile for a workspace: the project's own if present,
// otherwise the user's global default. found reports whether either
// existed; when false the returned profile is Default().
func Load(workspaceDir string) (p Profile, found bool, err error) {
	if p, ok := read(projectPath(workspaceDir)); ok {
		return p, true, nil
	}
	if global, err := globalPath(); err == nil {
		if p, ok := read(global); ok {
			return p, true, nil
		}
	}
	return Default(), false, nil
}

func read(path string) (Profile, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, false
	}
	p := Default()
	if err := json.Unmarshal(raw, &p); err != nil {
		return Profile{}, false
	}
	return p, true
}

// Save writes the profile to the workspace's .nina directory and mirrors
// it as the global default for future projects.
func Save(workspaceDir string, p Profile) error {
	if err := write(projectPath(workspaceDir), p); err != nil {
		return err
	}
	if global, err := globalPath(); err == nil {
		_ = write(global, p) // best effort; the project copy is authoritative
	}
	return nil
}

func write(path string, p Profile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
