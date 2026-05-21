package notify

import (
	"net/url"
	"path"
	"strings"
)

// MaskNotifyUrl replaces the username and password in a URL's userinfo
// with "***" so credentials are not exposed to the frontend.
//
// Examples:
//
//	smtp://user:password@host:587  ->  smtp://***:***@host:587
//	discord://token@webhookid      ->  discord://***@webhookid
//	https://hooks.example.com/xyz  ->  https://hooks.example.com/xyz (unchanged)
func MaskNotifyUrl(rawUrl string) string {
	if IsAppriseURL(rawUrl) {
		return AppriseURLPrefix + maskAppriseURL(StripApprisePrefix(rawUrl))
	}
	return maskParsedNotifyUrl(rawUrl)
}

func maskAppriseURL(rawUrl string) string {
	switch {
	case strings.HasPrefix(rawUrl, "mailto://"), strings.HasPrefix(rawUrl, "mailtos://"):
		parsed, err := url.Parse(rawUrl)
		if err != nil {
			return rawUrl
		}
		_, hasPassword := parsed.User.Password()
		if hasPassword {
			parsed.User = url.UserPassword("***", "***")
		} else if parsed.User != nil && parsed.User.Username() != "" {
			parsed.User = url.User("***")
		}
		query := parsed.Query()
		if query.Get("user") != "" {
			query.Set("user", "***")
		}
		if query.Get("pass") != "" {
			query.Set("pass", "***")
		}
		parsed.RawQuery = query.Encode()
		return normalizeMaskedURL(parsed.String())
	case strings.HasPrefix(rawUrl, "tgram://"):
		remainder := strings.TrimPrefix(rawUrl, "tgram://")
		parts := strings.SplitN(remainder, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			return rawUrl
		}
		masked := "***"
		if len(parts) == 1 {
			return "tgram://" + masked
		}
		return "tgram://" + masked + "/" + parts[1]
	case strings.HasPrefix(rawUrl, "https://discord.com/api/webhooks/"), strings.HasPrefix(rawUrl, "https://discordapp.com/api/webhooks/"):
		parsed, err := url.Parse(rawUrl)
		if err != nil {
			return rawUrl
		}
		parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
		if len(parts) >= 4 {
			parts[3] = "***"
			parsed.Path = "/" + path.Join(parts...)
		}
		return normalizeMaskedURL(parsed.String())
	case strings.HasPrefix(rawUrl, "https://hooks.slack.com/services/"):
		parsed, err := url.Parse(rawUrl)
		if err != nil {
			return rawUrl
		}
		parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
		if len(parts) >= 4 {
			parts[1] = "***"
			parts[2] = "***"
			parts[3] = "***"
			parsed.Path = "/" + path.Join(parts...)
		}
		return normalizeMaskedURL(parsed.String())
	default:
		return maskParsedNotifyUrl(rawUrl)
	}
}

func normalizeMaskedURL(rawUrl string) string {
	return strings.ReplaceAll(rawUrl, "%2A%2A%2A", "***")
}

func maskParsedNotifyUrl(rawUrl string) string {
	parsed, err := url.Parse(rawUrl)
	if err != nil || parsed.User == nil {
		return rawUrl
	}

	_, hasPassword := parsed.User.Password()
	if hasPassword {
		parsed.User = url.UserPassword("***", "***")
	} else if parsed.User.Username() != "" {
		parsed.User = url.User("***")
	}

	return normalizeMaskedURL(parsed.String())
}
