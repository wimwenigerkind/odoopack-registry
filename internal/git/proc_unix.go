//go:build unix

package git

import (
	"os/exec"
	"syscall"
)

// killProcessGroup makes cancellation kill git together with the helpers it
// spawns (git-remote-https, fetch-pack). Killing only git would leave them
// running against the remote and writing into a temp directory that the
// caller has already removed.
func killProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
