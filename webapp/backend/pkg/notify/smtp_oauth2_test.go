package notify

import (
	"net/smtp"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXOAUTH2Auth(t *testing.T) {
	auth := newXOAUTH2Auth("user@example.com", "token", "smtp.example.com")

	mech, resp, err := auth.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: true})
	require.NoError(t, err)
	require.Equal(t, "XOAUTH2", mech)
	require.Equal(t, "user=user@example.com\x01auth=Bearer token\x01\x01", string(resp))

	next, err := auth.Next([]byte(`{"status":"401"}`), true)
	require.NoError(t, err)
	require.Equal(t, []byte{}, next, "a 334 challenge gets an empty response so the server can send 535")

	_, _, err = auth.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: false})
	require.ErrorIs(t, err, errSMTPUnencryptedConnection, "the token must not be sent in plaintext")

	_, _, err = auth.Start(&smtp.ServerInfo{Name: "other.example.com", TLS: true})
	require.ErrorIs(t, err, errSMTPWrongHostName)

	local := newXOAUTH2Auth("user", "token", "localhost")
	_, _, err = local.Start(&smtp.ServerInfo{Name: "localhost", TLS: false})
	require.NoError(t, err, "localhost may authenticate without TLS")
}

func TestSMTPAuthForConfigOAuth2(t *testing.T) {
	n := &Notify{}
	serviceURL, err := url.Parse("smtp://user%40example.com:token@smtp.example.com:587/?auth=OAuth2&fromaddress=a%40example.com&toaddresses=b+tag%40example.com")
	require.NoError(t, err)
	config, err := n.buildSMTPConfig(serviceURL)
	require.NoError(t, err)
	require.Equal(t, []string{"b+tag@example.com"}, config.ToAddresses, "plus-tagged addresses survive URL parsing")

	auth, err := smtpAuthForConfig(config)
	require.NoError(t, err)
	require.IsType(t, &xoauth2Auth{}, auth)
}
