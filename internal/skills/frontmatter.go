package skills

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

const SkillFilename = "SKILL.md"

const MaxDescriptionLen = 1024

var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Frontmatter struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	DisableModelInvocation *bool  `yaml:"disable-model-invocation" json:"DisableModelInvocation,omitempty"`
}

var ErrNoFrontmatter = errors.New("no YAML frontmatter block found")

func Parse(path string) (Frontmatter, error) {
	var fm Frontmatter

	if filepath.Base(path) != SkillFilename {
		return fm, fmt.Errorf("file must be named %s, got %s", SkillFilename, filepath.Base(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fm, fmt.Errorf("read %s: %w", path, err)
	}

	block, err := extractFrontmatter(data)
	if err != nil {
		return fm, fmt.Errorf("%s: %w", path, err)
	}

	if err := yaml.Unmarshal(block, &fm); err != nil {
		return fm, fmt.Errorf("%s: parse frontmatter: %w", path, err)
	}
	return fm, nil
}

func extractFrontmatter(data []byte) ([]byte, error) {
	data = bytes.TrimLeft(data, "\ufeff \t\r\n")
	if !bytes.HasPrefix(data, []byte("---")) {
		return nil, ErrNoFrontmatter
	}
	rest := data[3:]
	if len(rest) == 0 || (rest[0] != '\n' && rest[0] != '\r') {
		return nil, ErrNoFrontmatter
	}
	closeFence := []byte("\n---")
	idx := bytes.Index(rest, closeFence)
	if idx < 0 {
		return nil, ErrNoFrontmatter
	}
	after := idx + len(closeFence)
	if after < len(rest) && rest[after] != '\n' && rest[after] != '\r' {
		return nil, ErrNoFrontmatter
	}
	return rest[:idx], nil
}

func Validate(fm Frontmatter, dirName string) error {
	if fm.Name == "" {
		return errors.New("frontmatter: name is required")
	}
	if !nameRegex.MatchString(fm.Name) {
		return fmt.Errorf("frontmatter: name %q must match %s", fm.Name, nameRegex.String())
	}
	if fm.Description == "" {
		return errors.New("frontmatter: description is required")
	}
	if len(fm.Description) > MaxDescriptionLen {
		return fmt.Errorf("frontmatter: description is %d bytes, max %d", len(fm.Description), MaxDescriptionLen)
	}
	if dirName != "" && fm.Name != dirName {
		return fmt.Errorf("frontmatter name %q does not match directory %q", fm.Name, dirName)
	}
	return nil
}
