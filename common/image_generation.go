package common

import "strings"

var ImageGenerationPageEnabled = false
var ImageGenerationPageGroups = []string{"default"}
var ImageGenerationPageModels = []string{"gpt-image-2"}

func ImageGenerationPageGroups2JSONString() string {
	return stringSlice2JSONString(ImageGenerationPageGroups)
}

func ImageGenerationPageModels2JSONString() string {
	return stringSlice2JSONString(ImageGenerationPageModels)
}

func UpdateImageGenerationPageGroupsByJSONString(jsonStr string) error {
	values, err := parseStringSliceJSONString(jsonStr)
	if err != nil {
		return err
	}
	ImageGenerationPageGroups = values
	return nil
}

func UpdateImageGenerationPageModelsByJSONString(jsonStr string) error {
	values, err := parseStringSliceJSONString(jsonStr)
	if err != nil {
		return err
	}
	ImageGenerationPageModels = values
	return nil
}

func ImageGenerationPageGroupAllowed(group string) bool {
	return stringInSlice(strings.TrimSpace(group), ImageGenerationPageGroups)
}

func ImageGenerationPageModelAllowed(model string) bool {
	return stringInSlice(strings.TrimSpace(model), ImageGenerationPageModels)
}

func stringSlice2JSONString(values []string) string {
	jsonBytes, err := Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func parseStringSliceJSONString(jsonStr string) ([]string, error) {
	var values []string
	if err := UnmarshalJsonStr(jsonStr, &values); err != nil {
		return nil, err
	}
	return normalizeStringSlice(values), nil
}

func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func stringInSlice(value string, values []string) bool {
	if value == "" {
		return false
	}
	for _, item := range values {
		if strings.TrimSpace(item) == value {
			return true
		}
	}
	return false
}
