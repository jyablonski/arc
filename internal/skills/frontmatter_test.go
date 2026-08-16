package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyablonski/arc/internal/filemode"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		body        string
		wantErr     string
		wantName    string
		wantDesc    string
		wantDisable *bool
	}{
		{
			name:     "valid frontmatter",
			filename: "SKILL.md",
			body:     "---\nname: foo\ndescription: does things\n---\n\nbody here",
			wantName: "foo",
			wantDesc: "does things",
		},
		{
			name:     "unknown keys pass through",
			filename: "SKILL.md",
			body:     "---\nname: foo\ndescription: x\nuser_invocable: true\n---\n",
			wantName: "foo",
			wantDesc: "x",
		},
		{
			name:        "disable model invocation",
			filename:    "SKILL.md",
			body:        "---\nname: foo\ndescription: x\ndisable-model-invocation: true\n---\n",
			wantName:    "foo",
			wantDesc:    "x",
			wantDisable: testBoolPointer(true),
		},
		{
			name:     "leading BOM tolerated",
			filename: "SKILL.md",
			body:     "\ufeff---\nname: foo\ndescription: x\n---\n",
			wantName: "foo",
		},
		{
			name:     "wrong basename",
			filename: "skill.md",
			body:     "---\nname: foo\ndescription: x\n---\n",
			wantErr:  "must be named SKILL.md",
		},
		{
			name:     "no frontmatter",
			filename: "SKILL.md",
			body:     "just a body with no fences\n",
			wantErr:  "no YAML frontmatter",
		},
		{
			name:     "unterminated frontmatter",
			filename: "SKILL.md",
			body:     "---\nname: foo\n",
			wantErr:  "no YAML frontmatter",
		},
		{
			name:     "malformed yaml",
			filename: "SKILL.md",
			body:     "---\nname: [unclosed\n---\n",
			wantErr:  "parse frontmatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.body), filemode.File); err != nil {
				t.Fatal(err)
			}
			fm, err := Parse(path)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("want error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fm.Name != tt.wantName {
				t.Errorf("name: got %q, want %q", fm.Name, tt.wantName)
			}
			if tt.wantDesc != "" && fm.Description != tt.wantDesc {
				t.Errorf("description: got %q, want %q", fm.Description, tt.wantDesc)
			}
			if tt.wantDisable == nil {
				if fm.DisableModelInvocation != nil {
					t.Errorf("disable-model-invocation: got %v, want absent", *fm.DisableModelInvocation)
				}
			} else if fm.DisableModelInvocation == nil || *fm.DisableModelInvocation != *tt.wantDisable {
				t.Errorf("disable-model-invocation: got %v, want %v", fm.DisableModelInvocation, *tt.wantDisable)
			}
		})
	}
}

func testBoolPointer(value bool) *bool {
	return &value
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		fm      Frontmatter
		dirName string
		wantErr string
	}{
		{
			name:    "valid",
			fm:      Frontmatter{Name: "foo", Description: "ok"},
			dirName: "foo",
		},
		{
			name:    "valid no dirname check",
			fm:      Frontmatter{Name: "foo", Description: "ok"},
			dirName: "",
		},
		{
			name:    "missing name",
			fm:      Frontmatter{Description: "ok"},
			wantErr: "name is required",
		},
		{
			name:    "bad name uppercase",
			fm:      Frontmatter{Name: "Foo", Description: "ok"},
			wantErr: "must match",
		},
		{
			name:    "bad name leading dash",
			fm:      Frontmatter{Name: "-foo", Description: "ok"},
			wantErr: "must match",
		},
		{
			name:    "bad name slash",
			fm:      Frontmatter{Name: "foo/bar", Description: "ok"},
			wantErr: "must match",
		},
		{
			name:    "missing description",
			fm:      Frontmatter{Name: "foo"},
			wantErr: "description is required",
		},
		{
			name:    "description too long",
			fm:      Frontmatter{Name: "foo", Description: strings.Repeat("x", MaxDescriptionLen+1)},
			wantErr: "max 1024",
		},
		{
			name:    "name dir mismatch",
			fm:      Frontmatter{Name: "foo", Description: "ok"},
			dirName: "bar",
			wantErr: "does not match directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.fm, tt.dirName)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
