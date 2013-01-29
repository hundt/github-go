package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var reviewers = flag.String("r", "", "comma-separated list of reviewers")

var verify = flag.Bool("y", false, "verify after pushing")

func run(cmd *exec.Cmd, o io.Writer, e io.Writer) error {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	go io.Copy(o, stdout)
	go io.Copy(e, stderr)
	cmd.Start()
	return cmd.Wait()
}

func branch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name, "origin/master")
	return run(cmd, os.Stdout, os.Stderr)
}

func pull() error {
	cmd := exec.Command("git", "pull", "--rebase")
	return run(cmd, os.Stdout, os.Stderr)
}

func push() error {
	ri, err := getRemote()
	if err != nil {
		return err
	}
	sha, err := getHeadSha()
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "push", "origin", "HEAD:refs/for/master")
	if err := run(cmd, os.Stdout, os.Stderr); err != nil {
		return err
	}
	if *verify {
		cmd := exec.Command("ssh",
			"-p",
			fmt.Sprintf("%d", ri.port),
			fmt.Sprintf("%s@%s", ri.user, ri.host),
			"gerrit",
			"review",
			"--verified",
			"1",
			sha)
		if err := run(cmd, os.Stdout, os.Stderr); err != nil {
			return err
		}
	}
	if *reviewers != "" {
		args := []string {
			"-p",
			fmt.Sprintf("%d", ri.port),
			fmt.Sprintf("%s@%s", ri.user, ri.host),
			"gerrit",
			"set-reviewers",
		}
		for _, reviewer := range strings.Split(*reviewers, ",") {
			args = append(args, "-a")
			args = append(args, reviewer)
		}
		args = append(args, sha)
		cmd := exec.Command("ssh", args...)
		if err := run(cmd, os.Stdout, os.Stderr); err != nil {
			return err
		}
	}
	return nil
}

func getHeadSha() (string, error) {
	cmd := exec.Command("git", "log", "-n", "1", "--format=format:%H")
	buf := new(bytes.Buffer)
	if err := run(cmd, buf, os.Stderr); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

type remoteInfo struct {
	user string
	host string
	port int
}

func index(str, sep string, index int) int {
	str = str[index:]
	i := strings.Index(str, sep)
	if i == -1 {
		return -1
	}
	return i + index
}

func getRemote() (*remoteInfo, error) {
	cmd := exec.Command("git", "config", "remote.origin.url")
	buf := new(bytes.Buffer)
	if err := run(cmd, buf, os.Stderr); err != nil {
		return nil, err
	}
	url := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(url, "ssh://") {
		return nil, errors.New("Invalid remote url: " + url)
	}
	userStart := len("ssh://")
	userEnd := index(url, "@", userStart)
	if userEnd == -1 {
		return nil, errors.New("Invalid remote url: " + url)
	}
	hostStart := userEnd + 1
	hostEnd := index(url, ":", hostStart)
	if userEnd == -1 {
		return nil, errors.New("Invalid remote url: " + url)
	}
	portStart := hostEnd + 1
	portEnd := index(url, "/", portStart)
	port, err := strconv.Atoi(url[portStart:portEnd])
	if err != nil {
		return nil, err
	}
	return &remoteInfo{
		user: url[userStart:userEnd],
		host: url[hostStart:hostEnd],
		port: port,
	}, nil
}

func usageAndQuit() {
	flag.Usage()
	os.Exit(2)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", os.Args[0])
		fmt.Print(
`
Commands:
  branch  create a new branch tracking master
  pull    rebase your branch from master
  push    push new changes from your branch
    -y mark the changes verified
    -r assign a reviewer (or comma-separated list)
`)
	}
	flag.Parse()
	
	if len(os.Args) < 2 {
		usageAndQuit()
	}
	
	command := os.Args[1]
	switch (command) {
	case "branch":
		if len(flag.Args()) < 2 {
			usageAndQuit()
		}
		if err := branch(flag.Arg(1)); err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			os.Exit(1)
		}
	case "pull":
		if err := pull(); err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			os.Exit(1)
		}
	case "push":
		// Reparse flags, ignoring the command.
		newArgs := make([]string, len(os.Args) - 1)
		newArgs[0] = os.Args[0]
		for i := 2; i < len(os.Args); i++ {
			newArgs[i-1] = os.Args[i]
		}
		os.Args = newArgs
		flag.Parse()
		
		if err := push(); err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			os.Exit(1)
		}
	case "remote":
		if ri, err := getRemote(); err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			os.Exit(1)
		} else {
			fmt.Printf("%#v\n", ri)
		}
	default:
		usageAndQuit()
	}
}