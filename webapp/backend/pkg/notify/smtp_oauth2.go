package notify

import (
	"errors"
	"net/smtp"
)

var (
	errSMTPUnencryptedConnection = errors.New("smtp oauth2: unencrypted connection")
	errSMTPWrongHostName         = errors.New("smtp oauth2: wrong host name")
)

// xoauth2Auth implements SASL XOAUTH2 (https://developers.google.com/gmail/imap/xoauth2-protocol).
// shoutrrr stopped exporting its implementation in v0.19, so the sender keeps its own. The password
// is used as a static access token; token refresh is not supported.
type xoauth2Auth struct {
	username    string
	accessToken string
	host        string
}

func newXOAUTH2Auth(username, accessToken, host string) smtp.Auth {
	return &xoauth2Auth{username: username, accessToken: accessToken, host: host}
}

// Start sends the credentials only over TLS or to localhost, and only to the configured host,
// matching smtp.PlainAuth.
func (a *xoauth2Auth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !isLocalhost(server.Name) {
		return "", nil, errSMTPUnencryptedConnection
	}
	if server.Name != a.host {
		return "", nil, errSMTPWrongHostName
	}
	return "XOAUTH2", []byte("user=" + a.username + "\x01auth=Bearer " + a.accessToken + "\x01\x01"), nil
}

// Next answers a 334 challenge (an error status from the server) with an empty response, so the
// server can follow with 535 instead of net/smtp treating the exchange as successful.
func (a *xoauth2Auth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return []byte{}, nil
	}
	return nil, nil
}

func isLocalhost(name string) bool {
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}
