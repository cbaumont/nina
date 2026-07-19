package profile

import (
	"reflect"
	"testing"
)

func TestParseValidation(t *testing.T) {
	if _, err := ParseLevel("beginner"); err != nil {
		t.Error(err)
	}
	if _, err := ParseLevel("expert"); err == nil {
		t.Error("invalid level accepted")
	}
	if _, err := ParseHintSpeed("slow"); err != nil {
		t.Error(err)
	}
	if _, err := ParseHintSpeed("warp"); err == nil {
		t.Error("invalid hint speed accepted")
	}
	for input, want := range map[string]int{"0": 0, "3": 3} {
		if dial, err := ParseDial(input); err != nil || dial != want {
			t.Errorf("ParseDial(%q) = %d, %v", input, dial, err)
		}
	}
	for _, input := range []string{"4", "-1", "x", ""} {
		if _, err := ParseDial(input); err == nil {
			t.Errorf("ParseDial(%q) accepted", input)
		}
	}
}

func TestLoadWithoutProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	prof, found, err := Load(t.TempDir())
	if err != nil || found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if !reflect.DeepEqual(prof, Default()) {
		t.Errorf("profile = %+v", prof)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	prof := Profile{
		Experience:     LevelProfessional,
		StackFamiliar:  LevelNone,
		KnownStacks:    []string{"go", "sql"},
		Dial:           2,
		HintEscalation: HintFast,
	}
	if err := Save(dir, prof); err != nil {
		t.Fatal(err)
	}
	got, found, err := Load(dir)
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if !reflect.DeepEqual(got, prof) {
		t.Errorf("profile = %+v, want %+v", got, prof)
	}

	// The saved profile also becomes the global default for new projects.
	got, found, err = Load(t.TempDir())
	if err != nil || !found {
		t.Fatalf("global: found = %v, err = %v", found, err)
	}
	if !reflect.DeepEqual(got, prof) {
		t.Errorf("global profile = %+v, want %+v", got, prof)
	}
}
