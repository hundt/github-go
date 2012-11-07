package main

import (
    "errors"
    "flag"
    "fmt"
    "github"
    "io/ioutil"
    "os"
    "os/exec"
    "strings"
)

var pushFirst = flag.Bool("p", false, "push to origin before making pull request")

var debug = flag.Bool("d", false, "show debug output for network requests")

var issue = flag.Int("i", -1, "existing issue to use instead of opening a new one")

var reviewers = flag.String("r", "", "comma-separated list of reviewers to assign immediately")

func getEditor() (string, error) {
    data, err := exec.Command("git", "config", "core.editor").Output()
    if err != nil {
        return "", errors.New("'git config core.editor' failed")
    }
    return strings.TrimSpace(string(data)), nil
}

func getCommitMessageFromUser(defaultMessage string) (title, body string, err error) {
    editor, err := getEditor()
    if err != nil { return "", "", err }
    fname := ".git/CHRIS_COMMIT_EDITMSG"
    err = ioutil.WriteFile(fname, []byte(defaultMessage), 0644)
    if err != nil { return "", "", err }
    editorPath, err := exec.LookPath(editor)
    if err != nil { return "", "", err }
    pa := &os.ProcAttr{Env: os.Environ(), Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}}
    p, err := os.StartProcess(editorPath, []string{editorPath, fname}, pa)
    if err != nil {
        return "", "", errors.New(fmt.Sprintf("Could not start editor '%s': %s", editorPath, err))
    }
    ps, err := p.Wait()
    if err != nil {
        return "", "", errors.New(fmt.Sprintf("Error returning from editor: %s", err))
    }
    if !ps.Success() {
        return "", "", errors.New("Editor returned non-zero exit status")
    }
    data, err := ioutil.ReadFile(fname)
    if err != nil {
        return "", "", errors.New(fmt.Sprintf("Could not read commit message file: %s", err))
    }
    pieces := strings.SplitN(string(data), "\n", 2)
    if len(pieces) == 0 || len(pieces[0]) == 0 {
        return "", "", errors.New("No title in description")
    }
    title = pieces[0]
    body = ""
    if len(pieces) > 1 && len(pieces[1]) > 0 {
        body = strings.TrimSpace(pieces[1])
    }
    
    return title, body, nil
}

func getRevList(branch string) ([]string, error) {
    output, err := exec.Command(
        "git", "log", "--oneline", "origin/master..." + branch).Output()
    if err != nil {
        return nil, errors.New(
            fmt.Sprintf(
                "Error running 'git log --oneline origin/master...%s'\n", branch))
    }
    revs := strings.Split(strings.TrimSpace(string(output)), "\n")
    for i, rev := range revs {
        pieces := strings.SplitN(rev, " ", 2)
        if len(pieces) != 2 {
            return nil, errors.New("Invalid log line: " + rev)
        }
        revs[i] = pieces[0]
    }
    return revs, nil
}

func getCommitMessage(sha string) (string, error) {
    data, err := exec.Command("git", "show", "-s", "--format=%w(78,0,0)%s%n%+b", sha).Output()
    if err != nil {
        return "", errors.New("'git show -s --format=\"%w(78,0,0)%s%n%+b\" " + sha + "' failed")
    }
    return strings.TrimSpace(string(data)), nil
}

func getBranch() (string, error) {
    data, err := exec.Command("git", "branch").Output()
    if err != nil {
        return "", errors.New("'git branch' failed")
    }
    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if len(line) > 0 && line[0] == '*' {
            return strings.TrimSpace(line[1:]), nil
        }
    }
    return "", errors.New("Could not determine current branch")
}

func push(branch string) error {
    fmt.Println("Pushing to origin...")
    output, err := exec.Command("git", "push", "origin", branch).CombinedOutput()
    if err != nil {
        return errors.New(fmt.Sprintf("Error pushing:\n%s", output))
    }
    fmt.Print(string(output))
    return nil
}

func showError(err error) {
    fmt.Printf("Error: %s\n", err)
    os.Exit(1)
}

func main() {
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "Usage: %s [-d] [-p] [-i issue] [-r reviewers]\n\n", os.Args[0])
        fmt.Print("The pull request will be\n  FROM the remote branch with the same name " +
            "as your local branch\n  TO master\n\n")
        fmt.Print("Options:\n")
        flag.PrintDefaults()
    }
    flag.Parse()

    var err error

    // Create API client
    c, err := github.ApiClientFromHubCredentials()
    if err != nil { showError(err) }
    c.Debug = *debug

    // Determine branch.
    branch, err := getBranch()
    if (err != nil) { showError(err) }
    
    // Get repo.
    user, repo, err := github.GetUserAndRepo()
    if (err != nil) { showError(err) }
    
    revs, err := getRevList(branch)
    if (err != nil) { showError(err) }
    
    if *pushFirst {
        // Push to origin.
        err = push(branch)
        if (err != nil) { showError(err) }
    }
    
    var pull github.PullRequest
    if *issue >= 0 {
        pull = c.CreatePullRequestFromIssue(user, repo, *issue, branch, "master")
    } else {
        defaultMsg := ""
        if len(revs) == 1 {
            defaultMsg, err = getCommitMessage(revs[0])
            if (err != nil) { showError(err) } 
        }
        
        title, body, err := getCommitMessageFromUser(defaultMsg)
        if (err != nil) { showError(err) }
        
        pull = c.CreatePullRequest(user, repo, title, body, branch, "master")
    }
    
    if pull.Errors != nil {
        showError(errors.New(fmt.Sprintf("error creating PR: %s", pull.Errors)))
    }
    
    if pull.Number == 0 {
        showError(errors.New("Unknown error creating pull request"))
    }

    fmt.Printf("%s\n", pull.IssueUrl)
    
    if *reviewers != "" {
        for _, reviewer := range strings.Split(*reviewers, ",") {
            comment := c.CommentOnPullRequest(user, repo, pull.Number,
                fmt.Sprintf("@%s Please review at your leisure", reviewer))
            if comment.Errors != nil {
                showError(errors.New(fmt.Sprintf("error adding comment: %s", comment.Errors)))
            }
        }
    }
}