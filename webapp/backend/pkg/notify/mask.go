package notify

import (
	"net/url"
	"path"
	"strings"
)

const telegramNotifyPrefix = "tgram://"

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
		return maskMailtoURL(rawUrl)
	case strings.HasPrefix(rawUrl, telegramNotifyPrefix):
		return maskTelegramURL(rawUrl)
	case strings.HasPrefix(rawUrl, "https://discord.com/api/webhooks/"), strings.HasPrefix(rawUrl, "https://discordapp.com/api/webhooks/"):
		return maskURLPathSegments(rawUrl, 3)
	case strings.HasPrefix(rawUrl, "https://hooks.slack.com/services/"):
		return maskURLPathSegments(rawUrl, 1, 2, 3)
	default:
		return maskParsedNotifyUrl(rawUrl)
	}
}

// maskMailtoURL masks userinfo credentials and user/pass query params in an apprise mailto URL.
func maskMailtoURL(rawUrl string) string {
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return rawUrl
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
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
}

// maskTelegramURL masks the bot token in a tgram:// URL while preserving any chat path.
func maskTelegramURL(rawUrl string) string {
	remainder := strings.TrimPrefix(rawUrl, telegramNotifyPrefix)
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return rawUrl
	}
	if len(parts) == 1 {
		return telegramNotifyPrefix + "***"
	}
	return telegramNotifyPrefix + "***/" + parts[1]
}

// maskURLPathSegments masks the given path-segment indices (only when at least 4 segments exist).
func maskURLPathSegments(rawUrl string, indices ...int) string {
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return rawUrl
	}
	parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if len(parts) >= 4 {
		for _, i := range indices {
			parts[i] = "***"
		}
		parsed.Path = "/" + path.Join(parts...)
	}
	return normalizeMaskedURL(parsed.String())
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
