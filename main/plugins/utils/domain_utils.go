package utils

import (
	"net/url"
	"strings"
)

// ExtractDomain 从 URL 中提取域名
func ExtractDomain(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return parsed.Hostname(), nil
}

// IsAllowedDomain 检查 URL 是否在允许的域名列表中
func IsAllowedDomain(rawURL string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true
	}

	hostname, err := ExtractDomain(rawURL)
	if err != nil {
		return false
	}

	hostname = strings.ToLower(hostname)

	for _, domain := range allowedDomains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}

		domain = strings.ToLower(domain)

		// 精确匹配
		if hostname == domain {
			return true
		}

		// 子域名匹配（如 *.example.com）
		if strings.HasPrefix(domain, "*.") {
			rootDomain := strings.TrimPrefix(domain, "*.")
			if strings.HasSuffix(hostname, rootDomain) {
				// 确保是子域名（如 a.example.com 匹配 *.example.com，但 example.com 不匹配）
				parts := strings.Split(hostname, ".")
				if len(parts) > 2 {
					// 有子域名，检查根域名
					root := strings.Join(parts[len(parts)-2:], ".")
					if root == rootDomain {
						return true
					}
				}
			}
		}

		// 后缀匹配（如 example.com 匹配 sub.example.com）
		if strings.HasSuffix(hostname, domain) {
			// 确保是完整的域名或子域名
			if hostname == domain || strings.HasSuffix(hostname, "."+domain) {
				return true
			}
		}
	}

	return false
}

// ParseAllowedDomains 解析允许的域名配置（逗号分隔的字符串）
func ParseAllowedDomains(domainStr string) []string {
	domainStr = strings.TrimSpace(domainStr)
	if domainStr == "" {
		return []string{}
	}

	domains := strings.Split(domainStr, ",")
	result := make([]string, 0, len(domains))

	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain != "" {
			result = append(result, domain)
		}
	}

	return result
}

// IsValidURL 检查字符串是否为有效的 URL
func IsValidURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// 检查协议
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	// 检查主机
	if parsed.Host == "" {
		return false
	}

	return true
}