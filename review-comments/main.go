package main

import (
    "flag"
    "fmt"
    "github"
    "io/ioutil"
    "os"
    "os/exec"
    "sort"
    "strconv"
    "strings"
)

var width = flag.Int("W", 120, "max diff output width")

var user = "judicata"

var c *github.ApiClient

func diff(file1, file2 string) string {
    data, _ := exec.Command("colordiff", "-y", "-t", "-W", fmt.Sprintf("%d", *width), file1, file2).Output()
    return string(data)
}

func fetch(path, sha, output_fname string) {
    data, err := exec.Command("git", "show", fmt.Sprintf("%s:%s", sha, path)).Output()
    if err != nil {
        panic(err)
    }
    if err := ioutil.WriteFile(output_fname, data, 0644); err != nil {
        panic(err)
    }
}

func guessDiffColumn(diffLines []string) int {
    if len(diffLines) == 0 || len(diffLines[0]) <= 4 {
        return 0
    }
    for guess := len(diffLines[0])/2 - 2; guess < len(diffLines[0]) - 1; guess++ {
        good_guess := true
        for i, line := range diffLines {
            if i == len(diffLines) - 1 && len(line) == 0 {
                continue
            }
            if (len(line) > guess + 1 && (
                (line[guess] != ' ' ||
                (line[guess + 1] != ' ' &&
                line[guess + 1] != '>' &&
                line[guess + 1] != '<' &&
                line[guess + 1] != '|')))) {
                good_guess = false
                break
            }
        }
        if good_guess {
            return guess + 1
        }
    }
    return 0
}

func max(x, y int) int {
    if x > y { return x }
    return y
}

func min(x, y int) int {
    if x < y { return x }
    return y
}

func lineMap(left, diff string) []int {
    ll := strings.Split(left, "\n")
    dl := strings.Split(diff, "\n")
    dc := guessDiffColumn(dl)
    m := make([]int, len(ll))
    di := 0
    for li, _ := range ll {
        for len(dl[di]) > dc && dl[di][dc] == '>' {
            di++
        }
        m[li] = di
        di++
    }
    return m
}

func read(fname string) string {
    data, err := ioutil.ReadFile(fname)
    if err != nil {
        panic(err)
    }
    return string(data)
}

func showComments(pull github.PullRequest, comments github.CommentList) {
    sort.Sort(comments)
    head_sha := pull.Head.SHA
    ci := 0
    for ci < len(comments) {
        fmt.Println("\n\n=============================================")
        path, line, sha := comments[ci].Path, comments[ci].Line, comments[ci].CommitId
        for ci < len(comments) && comments[ci].Path == path && comments[ci].Line == line {
            fmt.Printf("\033[1;30m%s\033[0m: %s\n", comments[ci].User.Login, comments[ci].Body)
            ci++
        }
        if line == 0 {
            continue
        }
        fetch(path, sha, "/tmp/before")
        fetch(path, head_sha, "/tmp/after")
        d := diff("/tmp/before", "/tmp/after")
        before := read("/tmp/before")
        dl := strings.Split(d, "\n")
        bl := strings.Split(before, "\n")
        m := lineMap(before, d)
        fmt.Printf("%s:%d\n", path, line)
        for i := max(0, m[line] - 3); i < min(m[line] + 4, len(bl)); i++ {
            if i == m[line] {
                //fmt.Printf("\033[1;30m*\033[0m:")
            } else {
                //fmt.Printf(" ")
            }
            fmt.Println(dl[i])
        }
    }
}

func gitLog(sha1, sha2 string) []string {
    data, err := exec.Command("git", "log", "--format=format:%H", fmt.Sprintf("%s..%s", sha1, sha2)).CombinedOutput()
    if err != nil {
        fmt.Print(string(data))
        panic(err)
    }
    return strings.Split(string(data), "\n")
}

func getAllComments(pull github.PullRequest) {
    log := gitLog(pull.Base.SHA, pull.Head.SHA)
    project := pull.Head.Repo.Name
    comments := c.GetPullRequestComments(user, project, pull.Number)
    for _, sha := range log {
        for _, comment := range c.GetCommitComments(user, project, sha) {
            comments = append(comments, comment)
        }
    }
    showComments(pull, comments)
}

func showError(err error) {
    fmt.Printf("Error: %s\n", err)
    os.Exit(1)
}

func main() {
    flag.Parse()
    pull, err := strconv.ParseInt(flag.Arg(0), 10, 64)
    if err != nil { showError(err) }

    c, err = github.ApiClientFromHubCredentials()
    if err != nil { showError(err) }

    user, repo, err := github.GetUserAndRepo()
    if (err != nil) { showError(err) }

    getAllComments(c.GetPullRequest(user, repo, int(pull)))
}