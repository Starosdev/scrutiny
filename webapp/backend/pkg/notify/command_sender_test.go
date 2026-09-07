package notify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const commandHelperMode = "SCRUTINY_TEST_COMMAND_MODE"
const commandHelperSafety = 10 * time.Second
const commandTestPoll = 10 * time.Millisecond

// Running the test executable as a script avoids platform-specific shell fixtures.
func TestMain(m *testing.M) {
	mode := os.Getenv(commandHelperMode)
	if mode == "" {
		os.Exit(m.Run())
	}
	switch mode {
	case "output":
		for _, key := range []string{"SUBJECT", "DATE", "FAILURE_TYPE", "DEVICE_NAME", "DEVICE_TYPE", "DEVICE_SERIAL", "MESSAGE", "HOST_ID"} {
			fmt.Println(key + "=" + os.Getenv("SCRUTINY_"+key))
		}
		fmt.Println("ready")
		fmt.Fprint(os.Stderr, "trailing stderr")
		deadline := time.Now().Add(commandHelperSafety)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(os.Getenv("SCRUTINY_TEST_RELEASE")); err == nil {
				os.Exit(0)
			}
			time.Sleep(commandTestPoll)
		}
	case "block":
		fmt.Println("ready")
		time.Sleep(commandHelperSafety)
	case "descendant":
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), commandHelperMode+"=block")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			panic(err)
		}
		if err := os.WriteFile(os.Getenv("SCRUTINY_TEST_CHILD_PID"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
			panic(err)
		}
		os.Exit(0)
	case "failure":
		fmt.Fprintln(os.Stderr, "controlled failure")
		os.Exit(1)
	}
	os.Exit(1)
}

func TestScriptStreamsOutputAndPreservesEnvironment(t *testing.T) {
	t.Setenv(commandHelperMode, "output")
	release := filepath.Join(t.TempDir(), "release")
	t.Setenv("SCRUTINY_TEST_RELEASE", release)
	output, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	originalStdout := os.Stdout
	os.Stdout = output
	defer func() { os.Stdout = originalStdout; _ = output.Close() }()
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	n := Notify{Logger: logger, Payload: Payload{Subject: "subject", Date: "date", FailureType: "Temperature", DeviceName: "drive", DeviceType: "ATA", DeviceSerial: "serial", Message: "message", HostId: "host"}}
	ctx, cancel := context.WithTimeout(context.Background(), commandHelperSafety)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- n.sendScriptNotification(ctx, "script://"+os.Args[0]) }()
	require.Eventually(t, func() bool {
		data, _ := os.ReadFile(output.Name())
		return strings.Contains(string(data), "stdout ready")
	}, transportTestAllowance, commandTestPoll)
	select {
	case err := <-done:
		t.Fatalf("script returned before release: %v", err)
	default:
	}
	require.NoError(t, os.WriteFile(release, nil, 0600))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(transportTestAllowance):
		t.Fatal("script did not finish")
	}
	data, err := os.ReadFile(output.Name())
	require.NoError(t, err)
	for _, expected := range []string{" >> ", "stdout SUBJECT=subject", "DATE=date", "FAILURE_TYPE=Temperature", "DEVICE_NAME=drive", "DEVICE_TYPE=ATA", "DEVICE_SERIAL=serial", "MESSAGE=message", "HOST_ID=host", "stderr trailing stderr"} {
		require.Contains(t, string(data), expected)
	}
}

func TestScriptCancellationAndFailures(t *testing.T) {
	for _, scenario := range []string{"timeout", "canceled", "failure", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv(commandHelperMode, "block")
			ctx, cancel := context.WithTimeout(context.Background(), smtpTestTimeout)
			defer cancel()
			path := os.Args[0]
			switch scenario {
			case "canceled":
				cancel()
			case "failure":
				t.Setenv(commandHelperMode, "failure")
			case "missing":
				path = filepath.Join(t.TempDir(), "missing")
			}
			n := Notify{Logger: logrus.New()}
			start := time.Now()
			err := n.sendScriptNotification(ctx, "script://"+path)
			require.Error(t, err)
			require.Less(t, time.Since(start), notificationCommandWaitDelay+transportTestAllowance)
			if scenario == "timeout" {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			}
			if scenario == "canceled" {
				require.ErrorIs(t, err, context.Canceled)
			}
		})
	}
}

func TestNotificationCommandsBoundInheritedPipes(t *testing.T) {
	for _, transport := range []string{"script", "apprise"} {
		t.Run(transport, func(t *testing.T) {
			t.Setenv(commandHelperMode, "descendant")
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			t.Setenv("SCRUTINY_TEST_CHILD_PID", pidFile)
			t.Cleanup(func() {
				data, err := os.ReadFile(pidFile)
				if err != nil {
					return
				}
				pid, err := strconv.Atoi(string(data))
				if err != nil {
					return
				}
				child, err := os.FindProcess(pid)
				if err == nil {
					_ = child.Kill()
					_, _ = child.Wait()
				}
			})
			n := Notify{Logger: logrus.New()}
			var err error
			start := time.Now()
			if transport == "script" {
				err = n.SendScriptNotification("script://" + os.Args[0])
			} else {
				original := execCommandContext
				execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
					return exec.CommandContext(ctx, os.Args[0])
				}
				t.Cleanup(func() { execCommandContext = original })
				err = n.SendAppriseNotification("apprise+logger://")
			}
			require.True(t, errors.Is(err, exec.ErrWaitDelay), "expected pipe cleanup failure, got %v", err)
			require.Less(t, time.Since(start), notificationCommandWaitDelay+transportTestAllowance)
		})
	}
}
