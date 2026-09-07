package notify

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const smtpTestTimeout = 100 * time.Millisecond
const transportTestAllowance = 2 * time.Second

func TestSMTPTimeoutConfiguration(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    time.Duration
		invalid bool
	}{
		{"", 10 * time.Second, false}, {"30s", 30 * time.Second, false},
		{"0s", 0, true}, {"-1s", 0, true}, {"invalid", 0, true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			u, err := url.Parse("smtp://localhost/?fromaddress=sender@example.com&toaddresses=recipient@example.com")
			require.NoError(t, err)
			if tc.value != "" {
				q := u.Query()
				q.Set("timeout", tc.value)
				u.RawQuery = q.Encode()
			}
			n := Notify{}
			cfg, err := n.buildSMTPConfig(u)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.Timeout)
		})
	}
}

func TestSMTPDeadlineBoundsConversation(t *testing.T) {
	for _, phase := range []string{"greeting", "data"} {
		t.Run(phase, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			stalled := make(chan struct{})
			closed := make(chan struct{})
			go func() {
				defer close(closed)
				conn, acceptErr := listener.Accept()
				if acceptErr != nil {
					return
				}
				defer conn.Close()
				// Safety deadline makes a broken implementation fail instead of hanging the suite.
				_ = conn.SetDeadline(time.Now().Add(2 * transportTestAllowance))
				if phase == "greeting" {
					close(stalled)
					_, _ = io.Copy(io.Discard, conn)
					return
				}
				_, _ = fmt.Fprint(conn, "220 test ESMTP\r\n")
				reader := bufio.NewReader(conn)
				for {
					line, readErr := reader.ReadString('\n')
					if readErr != nil {
						return
					}
					if strings.HasPrefix(line, "DATA") {
						close(stalled)
						_, _ = io.Copy(io.Discard, conn)
						return
					}
					_, _ = fmt.Fprint(conn, "250 OK\r\n")
				}
			}()
			n := Notify{Logger: logrus.New(), Payload: Payload{Subject: "test", Message: "body"}}
			start := time.Now()
			err = n.SendSMTPNotification("smtp://" + listener.Addr().String() + "/?fromaddress=sender@example.com&toaddresses=recipient@example.com&usestarttls=No&auth=None&timeout=" + smtpTestTimeout.String())
			require.Error(t, err)
			require.Less(t, time.Since(start), smtpTestTimeout+transportTestAllowance)
			select {
			case <-stalled:
			default:
				t.Fatal("server never reached the intended stall")
			}
			select {
			case <-closed:
			case <-time.After(transportTestAllowance):
				t.Fatal("SMTP connection was not closed")
			}
		})
	}
}
