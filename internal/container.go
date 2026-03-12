package internal

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ContainerFile struct {
	Path               string
	Mode               fs.FileMode
	Lines              []string
	HasTrailingNewline bool

	Image     string
	ImageLine int

	ContainerName    string
	PublishPorts     []string
	Requires         []string
	EnvironmentFiles []string
}

func ContainerFilePath(containerDir, app string) string {
	return filepath.Join(containerDir, app+".container")
}

func LoadContainerFile(path string) (*ContainerFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read container file %s: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat container file %s: %w", path, err)
	}

	text := string(content)
	hasTrailingNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if hasTrailingNewline {
		lines = lines[:len(lines)-1]
	}

	result := &ContainerFile{
		Path:               path,
		Mode:               info.Mode().Perm(),
		Lines:              lines,
		HasTrailingNewline: hasTrailingNewline,
		ImageLine:          -1,
	}

	imageCount := 0
	for index, line := range lines {
		key, value, ok := parseUnitAssignment(line)
		if !ok {
			continue
		}

		normalized := stripInlineComment(value)

		switch key {
		case "Image":
			imageCount++
			result.Image = normalized
			result.ImageLine = index
		case "ContainerName":
			result.ContainerName = normalized
		case "PublishPort":
			if normalized != "" {
				result.PublishPorts = append(result.PublishPorts, normalized)
			}
		case "Requires":
			result.Requires = append(result.Requires, strings.Fields(normalized)...)
		case "EnvironmentFile":
			result.EnvironmentFiles = append(result.EnvironmentFiles, strings.Fields(normalized)...)
		}
	}

	if imageCount == 0 {
		return nil, fmt.Errorf("container file %s does not contain Image=", path)
	}
	if imageCount > 1 {
		return nil, fmt.Errorf("container file %s has multiple Image= lines", path)
	}
	if strings.TrimSpace(result.Image) == "" {
		return nil, fmt.Errorf("container file %s has an empty Image= value", path)
	}

	return result, nil
}

func UpdateContainerImageAtomic(file *ContainerFile, image string) error {
	if file == nil {
		return fmt.Errorf("container file cannot be nil")
	}
	if file.ImageLine < 0 || file.ImageLine >= len(file.Lines) {
		return fmt.Errorf("image line index is out of range")
	}

	updated := append([]string(nil), file.Lines...)
	updated[file.ImageLine] = "Image=" + image

	content := strings.Join(updated, "\n")
	if file.HasTrailingNewline {
		content += "\n"
	}

	if err := AtomicWriteFile(file.Path, []byte(content), file.Mode); err != nil {
		return fmt.Errorf("atomic update %s: %w", file.Path, err)
	}

	return nil
}

func NetworkDependencyUnits(requires []string) []string {
	seen := map[string]struct{}{}
	units := make([]string, 0, len(requires))

	for _, unit := range requires {
		trimmed := strings.TrimSpace(unit)
		if trimmed == "" {
			continue
		}
		if !strings.HasSuffix(trimmed, ".service") {
			continue
		}

		lower := strings.ToLower(trimmed)
		if !strings.Contains(lower, "network") {
			continue
		}

		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		units = append(units, trimmed)
	}

	return units
}

func ResolveEnvironmentFilePath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "-")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)

	if value == "" {
		return "", fmt.Errorf("environment file value is empty")
	}

	home, err := os.UserHomeDir()
	if err == nil {
		value = strings.ReplaceAll(value, "%h", home)
	}

	if value == "~" || strings.HasPrefix(value, "~/") {
		value, err = ExpandPath(value)
		if err != nil {
			return "", err
		}
	}

	return value, nil
}

func parseUnitAssignment(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", "", false
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return "", "", false
	}

	index := strings.Index(trimmed, "=")
	if index <= 0 {
		return "", "", false
	}

	key := strings.TrimSpace(trimmed[:index])
	value := strings.TrimSpace(trimmed[index+1:])
	if key == "" {
		return "", "", false
	}

	return key, value, true
}

func stripInlineComment(value string) string {
	inSingleQuotes := false
	inDoubleQuotes := false

	for idx := 0; idx < len(value); idx++ {
		switch value[idx] {
		case '\'':
			if !inDoubleQuotes {
				inSingleQuotes = !inSingleQuotes
			}
		case '"':
			if !inSingleQuotes {
				inDoubleQuotes = !inDoubleQuotes
			}
		case '#', ';':
			if inSingleQuotes || inDoubleQuotes {
				continue
			}
			if idx == 0 || isSpaceByte(value[idx-1]) {
				return strings.TrimSpace(value[:idx])
			}
		}
	}

	return strings.TrimSpace(value)
}

func isSpaceByte(character byte) bool {
	return character == ' ' || character == '\t' || character == '\r'
}
