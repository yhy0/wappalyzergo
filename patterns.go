package wappalyzer

import (
	"fmt"
	"github.com/dlclark/regexp2"
	regexp "github.com/wasilibs/go-re2"
	"strconv"
	"strings"
)

// ParsedPattern encapsulates a regular expression with
// additional metadata for confidence and version extraction.
type ParsedPattern struct {
	regex  *regexp.Regexp
	regex2 *regexp2.Regexp // 一些正则的高级特性支持

	Confidence int
	Version    string
	SkipRegex  bool
}

// ParsePattern extracts information from a pattern, supporting both regex and simple patterns
func ParsePattern(pattern string) (*ParsedPattern, error) {
	parts := strings.Split(pattern, "\\;")
	p := &ParsedPattern{Confidence: 100}

	if parts[0] == "" {
		p.SkipRegex = true
	}
	for i, part := range parts {
		if i == 0 {
			if p.SkipRegex {
				continue
			}
			regexPattern := part

			regexPattern = strings.ReplaceAll(regexPattern, "/", "\\/")

			// 替换后有些正则就匹配不到了，应该哪里有问题，这里使用  go-re2 就不用他考虑的性能问题了
			// regexPattern = strings.ReplaceAll(regexPattern, "\\+", "__escapedPlus__")
			// regexPattern = strings.ReplaceAll(regexPattern, "+", "{1,250}")
			// regexPattern = strings.ReplaceAll(regexPattern, "*", "{0,250}")
			// regexPattern = strings.ReplaceAll(regexPattern, "__escapedPlus__", "\\+")

			var err error
			regexPattern = "(?i)" + regexPattern

			// re2 和 go不支持一些高级特性，使用 github.com/dlclark/regexp2 来处理
			if strings.Contains(regexPattern, "(?!") || strings.Contains(regexPattern, "(?<") {
				p.regex2, err = regexp2.Compile(regexPattern, 0)
			} else {
				p.regex, err = regexp.Compile("(?i)" + regexPattern)
			}

			if err != nil {
				return nil, err
			}
		} else {
			keyValue := strings.SplitN(part, ":", 2)
			if len(keyValue) < 2 {
				continue
			}

			switch keyValue[0] {
			case "confidence":
				conf, err := strconv.Atoi(keyValue[1])
				if err != nil {
					// If conversion fails, keep default confidence
					p.Confidence = 100
				} else {
					p.Confidence = conf
				}
			case "version":
				p.Version = keyValue[1]
			}
		}
	}
	return p, nil
}

func (p *ParsedPattern) Evaluate(target string) (bool, string) {
	if p.SkipRegex {
		return true, ""
	}
	if p.regex == nil && p.regex2 == nil {
		return false, ""
	} else if p.regex == nil && p.regex2 != nil {
		submatches, err := p.regex2.FindStringMatch(target)
		if err != nil {
			return false, ""
		}
		var matches []string

		for submatches != nil {
			matches = append(matches, submatches.String())
			submatches, _ = p.regex2.FindNextMatch(submatches)
		}
		extractedVersion, _ := p.extractVersion(matches)
		return true, extractedVersion
	} else {
		submatches := p.regex.FindStringSubmatch(target)
		if len(submatches) == 0 {
			return false, ""
		}
		extractedVersion, _ := p.extractVersion(submatches)
		return true, extractedVersion
	}
}

// extractVersion uses the provided pattern to extract version information from a target string.
func (p *ParsedPattern) extractVersion(submatches []string) (string, error) {
	if len(submatches) == 0 {
		return "", nil // No matches found
	}

	result := p.Version
	for i, match := range submatches[1:] { // Start from 1 to skip the entire match
		placeholder := fmt.Sprintf("\\%d", i+1)
		result = strings.ReplaceAll(result, placeholder, match)
	}

	// Evaluate any ternary expressions in the result
	result, err := evaluateVersionExpression(result, submatches[1:])
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// evaluateVersionExpression handles ternary expressions in version strings.
func evaluateVersionExpression(expression string, submatches []string) (string, error) {
	if strings.Contains(expression, "?") {
		parts := strings.Split(expression, "?")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid ternary expression: %s", expression)
		}

		trueFalseParts := strings.Split(parts[1], ":")
		if len(trueFalseParts) != 2 {
			return "", fmt.Errorf("invalid true/false parts in ternary expression: %s", expression)
		}

		if trueFalseParts[0] != "" { // Simple existence check
			if len(submatches) == 0 {
				return trueFalseParts[1], nil
			}
			return trueFalseParts[0], nil
		}
		if trueFalseParts[1] == "" {
			if len(submatches) == 0 {
				return "", nil
			}
			return trueFalseParts[0], nil
		}
		return trueFalseParts[1], nil
	}

	return expression, nil
}
