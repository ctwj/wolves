package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// ApplyFileNameRules 应用文件名替换规则
// 规则格式：每一行是 "oldPattern=newPattern" 或 "oldPattern:newPattern"
func ApplyFileNameRules(filename string, rules []string) string {
	result := filename

	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		// 解析规则
		var oldPattern, newPattern string
		if strings.Contains(rule, "=") {
			parts := strings.SplitN(rule, "=", 2)
			oldPattern = strings.TrimSpace(parts[0])
			newPattern = strings.TrimSpace(parts[1])
		} else if strings.Contains(rule, ":") {
			parts := strings.SplitN(rule, ":", 2)
			oldPattern = strings.TrimSpace(parts[0])
			newPattern = strings.TrimSpace(parts[1])
		} else {
			continue
		}

		if oldPattern == "" {
			continue
		}

		// 尝试正则表达式替换
		if matched, err := regexp.MatchString(oldPattern, result); matched && err == nil {
			re := regexp.MustCompile(oldPattern)
			result = re.ReplaceAllString(result, newPattern)
		} else {
			// 普通字符串替换
			result = strings.ReplaceAll(result, oldPattern, newPattern)
		}
	}

	return result
}

// SanitizeFileName 清理文件名，移除非法字符
func SanitizeFileName(filename string) string {
	// Windows 和 Linux 非法字符
	illegalChars := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*", "\n", "\r", "\t"}

	result := filename
	for _, char := range illegalChars {
		result = strings.ReplaceAll(result, char, "_")
	}

	// 移除首尾空格和点
	result = strings.TrimSpace(result)
	result = strings.Trim(result, ".")

	// 如果为空，返回默认文件名
	if result == "" {
		result = "file"
	}

	return result
}

// GetFileExtension 获取文件扩展名（小写）
func GetFileExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext
}

// GetFileNameWithoutExt 获取不带扩展名的文件名
func GetFileNameWithoutExt(filename string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

// IsAllowedExtension 检查文件扩展名是否在允许列表中
func IsAllowedExtension(filename string, allowedExtensions []string) bool {
	if len(allowedExtensions) == 0 {
		return true
	}

	ext := GetFileExtension(filename)
	for _, allowedExt := range allowedExtensions {
		allowedExt = strings.TrimSpace(allowedExt)
		if allowedExt == "" {
			continue
		}
		// 确保 ext 和 allowedExt 都有小数点
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if !strings.HasPrefix(allowedExt, ".") {
			allowedExt = "." + allowedExt
		}
		if ext == allowedExt {
			return true
		}
	}
	return false
}

// ParseFileExtensions 解析文件扩展名配置（逗号分隔的字符串）
func ParseFileExtensions(extStr string) []string {
	extStr = strings.TrimSpace(extStr)
	if extStr == "" {
		return []string{}
	}

	extensions := strings.Split(extStr, ",")
	result := make([]string, 0, len(extensions))

	for _, ext := range extensions {
		ext = strings.TrimSpace(ext)
		if ext != "" {
			result = append(result, ext)
		}
	}

	return result
}