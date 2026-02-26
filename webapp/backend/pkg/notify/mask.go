package notify

import "net/url"

// MaskNotifyUrl replaces the username and password in a URL's userinfo
// with "***" so credentials are not exposed to the frontend.
//
// Examples:
//
//	smtp://user:password@host:587  ->  smtp://***:***@host:587
//	discord://token@webhookid      ->  discord://***@webhookid
//	https://hooks.example.com/xyz  ->  https://hooks.example.com/xyz (unchanged)
func MaskNotifyUrl(rawUrl string) string {
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

	return parsed.String()
}
