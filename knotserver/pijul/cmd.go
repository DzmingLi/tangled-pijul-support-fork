package pijul

import (
	"bytes"
	"fmt"
	"os/exec"
)

const (
	fieldSeparator  = "\x1f" // ASCII Unit Separator
	recordSeparator = "\x1e" // ASCII Record Separator
)

// runPijulCmd executes a pijul command in the repository directory
func (p *PijulRepo) runPijulCmd(command string, extraArgs ...string) ([]byte, error) {
	var args []string
	args = append(args, command)
	args = append(args, extraArgs...)

	cmd := exec.Command("pijul", args...)
	cmd.Dir = p.path

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w, stderr: %s", exitErr, stderr.String())
		}
		return nil, fmt.Errorf("pijul %s: %w", command, err)
	}

	return stdout.Bytes(), nil
}

// runPijulCmdWithStdin executes a pijul command with stdin input
func (p *PijulRepo) runPijulCmdWithStdin(stdin []byte, command string, extraArgs ...string) ([]byte, error) {
	var args []string
	args = append(args, command)
	args = append(args, extraArgs...)

	cmd := exec.Command("pijul", args...)
	cmd.Dir = p.path
	cmd.Stdin = bytes.NewReader(stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w, stderr: %s", exitErr, stderr.String())
		}
		return nil, fmt.Errorf("pijul %s: %w", command, err)
	}

	return stdout.Bytes(), nil
}

// log runs pijul log with arguments
func (p *PijulRepo) log(extraArgs ...string) ([]byte, error) {
	return p.runPijulCmd("log", extraArgs...)
}

// channelCmd runs pijul channel with arguments
func (p *PijulRepo) channelCmd(extraArgs ...string) ([]byte, error) {
	return p.runPijulCmd("channel", extraArgs...)
}

// diff runs pijul diff with arguments
func (p *PijulRepo) diff(extraArgs ...string) ([]byte, error) {
	return p.runPijulCmd("diff", extraArgs...)
}

// change runs pijul change (show change details) with arguments
func (p *PijulRepo) change(extraArgs ...string) ([]byte, error) {
	return p.runPijulCmd("change", extraArgs...)
}
