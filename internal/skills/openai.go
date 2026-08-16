package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jyablonski/arc/internal/filemode"
	"gopkg.in/yaml.v3"
)

const openAIMetadataPath = "agents/openai.yaml"

func (m *Manager) syncOpenAIMetadata(canonical string, fm Frontmatter) (bool, error) {
	if fm.DisableModelInvocation == nil {
		return false, nil
	}

	path := filepath.Join(canonical, filepath.FromSlash(openAIMetadataPath))
	desired := !*fm.DisableModelInvocation
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if desired {
			return false, nil
		}
		root := newOpenAIMetadata(desired)
		return m.writeOpenAIMetadata(path, root, "create")
	}
	if err != nil {
		return false, fmt.Errorf("lstat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("%s is a symlink; refusing to replace it", path)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	root, err := documentMapping(&doc)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	changed, err := setMappingBool(root, "policy", "allow_implicit_invocation", desired)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	if !changed {
		return false, nil
	}
	return m.writeOpenAIMetadata(path, &doc, "update")
}

func newOpenAIMetadata(allowImplicit bool) *yaml.Node {
	policy := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			stringNode("allow_implicit_invocation"),
			boolNode(allowImplicit),
		},
	}
	root := &yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     "!!map",
		Content: []*yaml.Node{stringNode("policy"), policy},
	}
	return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
}

func documentMapping(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return nil, fmt.Errorf("metadata must contain one YAML document")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("metadata root must be a mapping")
	}
	return root, nil
}

func setMappingBool(root *yaml.Node, section, key string, desired bool) (bool, error) {
	sectionNode := mappingValue(root, section)
	changed := false
	if sectionNode == nil {
		sectionNode = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, stringNode(section), sectionNode)
		changed = true
	} else if sectionNode.Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s must be a mapping", section)
	}

	valueNode := mappingValue(sectionNode, key)
	if valueNode == nil {
		sectionNode.Content = append(sectionNode.Content, stringNode(key), boolNode(desired))
		return true, nil
	}
	var current bool
	if err := valueNode.Decode(&current); err != nil || valueNode.Tag != "!!bool" {
		return false, fmt.Errorf("%s.%s must be a boolean", section, key)
	}
	if current == desired {
		return changed, nil
	}
	valueNode.Kind = yaml.ScalarNode
	valueNode.Tag = "!!bool"
	valueNode.Value = strconv.FormatBool(desired)
	return true, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func stringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func boolNode(value bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value)}
}

func (m *Manager) writeOpenAIMetadata(path string, doc *yaml.Node, verb string) (bool, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return false, fmt.Errorf("encode %s: %w", path, err)
	}
	if err := enc.Close(); err != nil {
		return false, fmt.Errorf("encode %s: %w", path, err)
	}

	m.announce("%s Codex metadata %s", verb, path)
	if m.dryRun {
		return true, nil
	}
	if err := m.fs.WriteFileAtomic(path, buf.Bytes(), filemode.File); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
