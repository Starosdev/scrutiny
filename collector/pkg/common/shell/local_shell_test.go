package shell

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestLocalShellCommand(t *testing.T) {
	t.Parallel()

	//setup
	testShell := localShell{}
	//test
	result, err := testShell.Command(logrus.WithField("exec", "test"), "echo", []string{"hello world"}, "", nil)

	//assert
	require.NoError(t, err)
	require.Equal(t, "hello world\n", result)
}

func TestLocalShellCommand_Date(t *testing.T) {
	t.Parallel()

	//setup
	testShell := localShell{}

	//test
	_, err := testShell.Command(logrus.WithField("exec", "test"), "date", []string{}, "", nil)

	//assert
	require.NoError(t, err)
}

//
//func TestExecCmd_Error(t *testing.T) {
//	t.Parallel()
//
//	//setup
//	bc := collector.BaseCollector {}
//
//	//test
//	_, err := bc.ExecCmd("smartctl", []string{"-a", "/dev/doesnotexist"}, "", nil)
//
//	//assert
//	exitError, castOk := err.(*exec.ExitError);
//	require.True(t, castOk)
//	require.Equal(t, 1, exitError.ExitCode())
//
//}
//

func TestLocalShellCommand_InvalidCommand(t *testing.T) {
	t.Parallel()

	//setup
	testShell := localShell{}

	//test
	_, err := testShell.Command(logrus.WithField("exec", "test"), "invalid_binary", []string{}, "", nil)

	//assert
	_, castOk := err.(*exec.ExitError)
	require.False(t, castOk)
}

func TestLocalShellCommandContext_Timeout(t *testing.T) {
	t.Parallel()

	//setup
	testShell := localShell{}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	//test
	_, err := testShell.CommandContext(ctx, logrus.WithField("exec", "test"), "sleep", []string{"5"}, "", nil)

	//assert
	require.Error(t, err)
}
