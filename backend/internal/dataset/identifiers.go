package dataset

import (
	"fmt"
	"path"
	"strings"
)

// sanitizeIdentifier turns arbitrary text into a valid QueryLens identifier.
//
// It is intentionally lossy: characters outside [A-Za-z0-9_] become underscores.
// The import preview shows the result, so the user can rename before committing.
func sanitizeIdentifier(raw, fallback string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}

	name := strings.Trim(builder.String(), "_")
	if name == "" {
		return fallback
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "_" + name
	}
	return name
}

// dedupeNames makes column names unique case-insensitively by appending _2, _3, ...
func dedupeNames(names []string) []string {
	used := make(map[string]struct{}, len(names))
	result := make([]string, len(names))

	for i, name := range names {
		candidate := name
		for suffix := 2; ; suffix++ {
			key := strings.ToLower(candidate)
			if _, taken := used[key]; !taken {
				used[key] = struct{}{}
				break
			}
			candidate = fmt.Sprintf("%s_%d", name, suffix)
		}
		result[i] = candidate
	}

	return result
}

// tableNameFor picks the table name from an override, then the filename, then a
// fixed fallback.
func tableNameFor(override, filename string) string {
	if strings.TrimSpace(override) != "" {
		return sanitizeIdentifier(override, "imported_table")
	}

	base := BaseFilename(filename)
	stem := strings.TrimSuffix(base, path.Ext(base))
	return sanitizeIdentifier(stem, "imported_table")
}

// BaseFilename strips any directory component a client may have supplied.
func BaseFilename(name string) string {
	normalized := strings.ReplaceAll(name, `\`, "/")
	base := path.Base(normalized)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func slug(name string) string {
	var builder strings.Builder
	previousDash := false

	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			previousDash = false
		default:
			if !previousDash && builder.Len() > 0 {
				builder.WriteByte('-')
				previousDash = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}

func formatBytes(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(size)/1024)
	default:
		return fmt.Sprintf("%.1f MiB", float64(size)/(1024*1024))
	}
}
