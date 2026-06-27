package xconfigs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

// SecretCheckResult holds the outcome of verifying a single secret reference in a config.
type SecretCheckResult struct {
	ConfigPath string
	Value      string
	OK         bool
	Message    string
}

// VerifyConfigSecrets builds the merged YAML for env + serviceConfigPath, walks every scalar,
// and resolves each awssm:// reference and plain "replace-me" placeholder through sm.
// Returns one result per reference found; never returns a partial result set on YAML parse errors.
func VerifyConfigSecrets(ctx context.Context, sharedFS fs.FS, env, serviceConfigPath string, sm SecretManager) ([]SecretCheckResult, error) {
	combined, err := BuildCombinedYAML(sharedFS, env, serviceConfigPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("build combined yaml: %w", err)
	}

	refs, err := findSecretRefs(combined)
	if err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	results := make([]SecretCheckResult, 0, len(refs))
	for _, r := range refs {
		result, err := checkSecretRef(ctx, r.configPath, r.value, sm)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

type secretRef struct {
	configPath string
	value      string
}

func findSecretRefs(combined string) ([]secretRef, error) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(combined), &node); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	var refs []secretRef
	walkNodeForRefs(&node, "", &refs)
	return refs, nil
}

func walkNodeForRefs(node *yaml.Node, path string, refs *[]secretRef) {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			walkNodeForRefs(child, path, refs)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			walkNodeForRefs(node.Content[i+1], childPath, refs)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			walkNodeForRefs(child, fmt.Sprintf("%s[%d]", path, i), refs)
		}
	case yaml.ScalarNode:
		v := node.Value
		if strings.HasPrefix(v, "awssm://") || v == "replace-me" {
			*refs = append(*refs, secretRef{configPath: path, value: v})
		}
	}
}

func checkSecretRef(ctx context.Context, configPath, yamlPropertyValue string, sm SecretManager) (SecretCheckResult, error) {
	fail := func(msg string) SecretCheckResult {
		return SecretCheckResult{
			ConfigPath: configPath,
			Value:      yamlPropertyValue,
			OK:         false,
			Message:    msg,
		}
	}

	if yamlPropertyValue == "replace-me" {
		return fail(`plain placeholder "replace-me"`), nil
	}

	resolved, err := sm.AccessSecretIfEligible(ctx, yamlPropertyValue)
	if err != nil {
		if errors.Is(err, ErrAWSAccessDenied) {
			return SecretCheckResult{}, fmt.Errorf("cannot verify %s: %w", yamlPropertyValue, err)
		}
		return fail(fmt.Sprintf("%s — %s", yamlPropertyValue, err.Error())), nil
	}

	switch resolved {
	case "":
		return fail(fmt.Sprintf("%s — yaml property value is empty", yamlPropertyValue)), nil
	case "replace-me":
		return fail(fmt.Sprintf(`%s — yaml property value is placeholder "replace-me"`, yamlPropertyValue)), nil
	}

	return SecretCheckResult{
		ConfigPath: configPath,
		Value:      yamlPropertyValue,
		OK:         true,
		Message:    yamlPropertyValue,
	}, nil
}
