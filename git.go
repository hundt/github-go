package github

import (
    "errors"
    "os/exec"
    "regexp"
    "strings"
)

var repoRE *regexp.Regexp

func init() {
    var err error
    repoRE, err = regexp.Compile("github\\.com/([^/]+)/([^/]+?)(?:\\.git)?$")
    if (err != nil) { panic(err) }
}

func GetUserAndRepo() (string, string, error) {
    data, err := exec.Command("git", "config", "remote.origin.url").Output()
    if err != nil {
        return "", "", errors.New("'git config remote.origin.url' failed")
    }
    url := strings.TrimSpace(string(data))
    matches := repoRE.FindStringSubmatch(url)
    if matches == nil {
        return "", "", errors.New("Could not understand remote origin url: " + url)
    }
    return matches[1], matches[2], nil
}