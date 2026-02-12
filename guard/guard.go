package guard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/urfave/cli/v3"
	"tangled.org/core/log"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:   "guard",
		Usage:  "role-based access control for git over ssh (not for manual use)",
		Action: Run,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "user",
				Usage:    "allowed git user",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "git-dir",
				Usage: "base directory for git repos",
				Value: "/home/git",
			},
			&cli.StringFlag{
				Name:  "log-path",
				Usage: "path to log file",
				Value: "/home/git/guard.log",
			},
			&cli.StringFlag{
				Name:  "internal-api",
				Usage: "internal API endpoint",
				Value: "http://localhost:5444",
			},
			&cli.StringFlag{
				Name:  "motd-file",
				Usage: "path to message of the day file",
				Value: "/home/git/motd",
			},
		},
	}
}

func Run(ctx context.Context, cmd *cli.Command) error {
	l := log.FromContext(ctx)

	incomingUser := cmd.String("user")
	gitDir := cmd.String("git-dir")
	logPath := cmd.String("log-path")
	endpoint := cmd.String("internal-api")
	motdFile := cmd.String("motd-file")

	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		l.Error("failed to open log file", "error", err)
		return err
	} else {
		fileHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo})
		l = slog.New(fileHandler)
	}

	var clientIP string
	if connInfo := os.Getenv("SSH_CONNECTION"); connInfo != "" {
		parts := strings.Fields(connInfo)
		if len(parts) > 0 {
			clientIP = parts[0]
		}
	}

	if incomingUser == "" {
		l.Error("access denied: no user specified")
		fmt.Fprintln(os.Stderr, "access denied: no user specified")
		os.Exit(-1)
	}

	sshCommand := os.Getenv("SSH_ORIGINAL_COMMAND")

	l.Info("connection attempt",
		"user", incomingUser,
		"command", sshCommand,
		"client", clientIP)

	// TODO: greet user with their resolved handle instead of did
	if sshCommand == "" {
		l.Info("access denied: no interactive shells", "user", incomingUser)
		fmt.Fprintf(os.Stderr, "Hi @%s! You've successfully authenticated.\n", incomingUser)
		os.Exit(-1)
	}

	cmdParts := strings.Fields(sshCommand)
	if len(cmdParts) < 2 {
		l.Error("invalid command format", "command", sshCommand)
		fmt.Fprintln(os.Stderr, "invalid command format")
		os.Exit(-1)
	}

	if cmdParts[0] == "pijul" {
		if cmdParts[1] != "protocol" {
			l.Error("access denied: invalid pijul command", "command", sshCommand)
			fmt.Fprintln(os.Stderr, "access denied: invalid pijul command")
			return fmt.Errorf("access denied: invalid pijul command")
		}

		repoPath, version, err := parsePijulProtocolArgs(cmdParts[2:])
		if err != nil {
			l.Error("invalid pijul protocol args", "command", sshCommand, "err", err)
			fmt.Fprintln(os.Stderr, "invalid pijul protocol args")
			return err
		}
		if version != "" && version != "3" {
			l.Error("unsupported pijul protocol version", "version", version)
			fmt.Fprintln(os.Stderr, "unsupported pijul protocol version")
			return fmt.Errorf("unsupported pijul protocol version")
		}

		qualifiedRepoPath, err := guardAndQualifyRepo(l, endpoint, incomingUser, repoPath, "pijul-protocol")
		if err != nil {
			l.Error("failed to run guard", "err", err)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fullPath, _ := securejoin.SecureJoin(gitDir, qualifiedRepoPath)
		args := []string{"protocol", "--repository", fullPath}
		if version != "" {
			args = append(args, "--version", version)
		}

		l.Info("processing command",
			"user", incomingUser,
			"command", "pijul protocol",
			"repo", repoPath,
			"fullPath", fullPath,
			"client", clientIP)

		pijulCmd := exec.Command("pijul", args...)
		pijulCmd.Stdout = os.Stdout
		pijulCmd.Stderr = os.Stderr
		pijulCmd.Stdin = os.Stdin

		if err := pijulCmd.Run(); err != nil {
			l.Error("command failed", "error", err)
			fmt.Fprintf(os.Stderr, "command failed: %v\n", err)
			return fmt.Errorf("command failed: %v", err)
		}

		l.Info("command completed",
			"user", incomingUser,
			"command", "pijul protocol",
			"repo", repoPath,
			"success", true)

		return nil
	}

	gitCommand := cmdParts[0]
	repoPath := cmdParts[1]

	validCommands := map[string]bool{
		"git-receive-pack":   true,
		"git-upload-pack":    true,
		"git-upload-archive": true,
	}
	if !validCommands[gitCommand] {
		l.Error("access denied: invalid git command", "command", gitCommand)
		fmt.Fprintln(os.Stderr, "access denied: invalid git command")
		return fmt.Errorf("access denied: invalid git command")
	}

	// qualify repo path from internal server which holds the knot config
	qualifiedRepoPath, err := guardAndQualifyRepo(l, endpoint, incomingUser, repoPath, gitCommand)
	if err != nil {
		l.Error("failed to run guard", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fullPath, _ := securejoin.SecureJoin(gitDir, qualifiedRepoPath)

	l.Info("processing command",
		"user", incomingUser,
		"command", gitCommand,
		"repo", repoPath,
		"fullPath", fullPath,
		"client", clientIP)

	var motdReader io.Reader
	if reader, err := os.Open(motdFile); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			l.Error("failed to read motd file", "error", err)
		}
		motdReader = strings.NewReader("Welcome to this knot!\n")
	} else {
		motdReader = reader
	}
	if gitCommand == "git-upload-pack" {
		io.WriteString(os.Stderr, "\x02")
	}
	io.Copy(os.Stderr, motdReader)

	gitCmd := exec.Command(gitCommand, fullPath)
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr
	gitCmd.Stdin = os.Stdin
	gitCmd.Env = append(os.Environ(),
		fmt.Sprintf("GIT_USER_DID=%s", incomingUser),
	)

	if err := gitCmd.Run(); err != nil {
		l.Error("command failed", "error", err)
		fmt.Fprintf(os.Stderr, "command failed: %v\n", err)
		return fmt.Errorf("command failed: %v", err)
	}

	l.Info("command completed",
		"user", incomingUser,
		"command", gitCommand,
		"repo", repoPath,
		"success", true)

	return nil
}

func parsePijulProtocolArgs(args []string) (string, string, error) {
	var repo string
	var version string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--repository" || arg == "-r":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("missing --repository value")
			}
			repo = args[i+1]
			i++
		case strings.HasPrefix(arg, "--repository="):
			repo = strings.TrimPrefix(arg, "--repository=")
		case arg == "--version":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("missing --version value")
			}
			version = args[i+1]
			i++
		case strings.HasPrefix(arg, "--version="):
			version = strings.TrimPrefix(arg, "--version=")
		}
	}
	if repo == "" {
		return "", "", fmt.Errorf("missing --repository")
	}
	return repo, version, nil
}

// runs guardAndQualifyRepo logic
func guardAndQualifyRepo(l *slog.Logger, endpoint, incomingUser, repo, gitCommand string) (string, error) {
	u, _ := url.Parse(endpoint + "/guard")
	q := u.Query()
	q.Add("user", incomingUser)
	q.Add("repo", repo)
	q.Add("gitCmd", gitCommand)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	l.Info("Running guard", "url", u.String(), "status", resp.Status)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	text := string(body)

	switch resp.StatusCode {
	case http.StatusOK:
		return text, nil
	case http.StatusForbidden:
		l.Error("access denied: user not allowed", "did", incomingUser, "reponame", text)
		return text, errors.New("access denied: user not allowed")
	default:
		return "", errors.New(text)
	}
}
