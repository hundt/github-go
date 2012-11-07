package github

import (
    "bytes"
    "errors"
    "fmt"
    "io/ioutil"
    "encoding/json"
    "net/http"
    "os"
    "strings"
)

type CommentList []Comment

func (l CommentList) Swap(i, j int) {
    t := l[i]
    l[i] = l[j]
    l[j] = t
}

func (l CommentList) Len() int {
    return len(l)
}

func (l CommentList) Less(i, j int) bool {
    if l[i].Path == l[j].Path {
        if l[i].Line == l[j].Line {
            return l[i].Created < l[j].Created
        }
        return l[i].Line < l[j].Line
    }
    return l[i].Path < l[j].Path
}

type Comment struct {
    Errors []Error
    CommitId string `json:"commit_id"`
    Path string
    Position int
    Line int
    Body string `json:"body"`
    Created string `json:"created_at"`
    User User
}

type BodyOnlyComment struct {
    Body string `json:"body"`
}

type User struct {
    Login string
}

type Error struct {
    Message string
}

type PullRequest struct {
    Errors []Error
	Head Commit
	Base Commit
	Number int
    IssueUrl string `json:"issue_url"`
}

type Commit struct {
	SHA string
	Repo Repo
}

type Repo struct {
	Name string
}

type createPullRequestRequest struct {
    Title string `json:"title"`
    Body string `json:"body"`
    Base string `json:"base"`
    Head string `json:"head"`
}

type createPullRequestFromIssueRequest struct {
    Issue int `json:"issue"`
    Base string `json:"base"`
    Head string `json:"head"`
}

/*
var (
	oauth_token = "beb5bfdb2d10fdddbeb10b48d02db1b5c46dd274"
	user = "judicata"
)
*/

type ApiClient struct {
    OAuthToken, User string
    Debug bool
}

func ApiClientFromHubCredentials() (*ApiClient, error) {
    fname := os.Getenv("HOME") + "/.config/hub"
    data, err := ioutil.ReadFile(fname)
    if err != nil {
        return nil, errors.New("Could not read " + fname)
    }
    var user, token string
    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if i := strings.Index(line, "user: "); i != -1 {
            user = line[i + len("user: "):]
        } else if i := strings.Index(line, "oauth_token: "); i != -1 {
            token = line[i + len("oauth_token: "):]
        }
    }
    if user == "" || token == "" {
        return nil, errors.New("Could not read user and token")
    }
    return &ApiClient{OAuthToken: token, User: user}, nil
}

func (c *ApiClient) load(url string, data []byte) ([]byte, error) {
    method := "GET"
    if data != nil {
        method = "POST"
    }
	if c.Debug {fmt.Print("DEBUG: REQUEST: ", url, "\n", string(data), "\n")}
	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(data))
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.OAuthToken))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if c.Debug {fmt.Print("DEBUG: RESPONSE: ", string(body), "\n")}
	return body, nil
}

func (c *ApiClient) GetOpenPullRequests(user, project string) []PullRequest {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls?status=open",
		user,
		project)
	body, _ := c.load(url, nil)
	var pulls []PullRequest
	json.Unmarshal(body, &pulls)
	return pulls
}

func (c *ApiClient) GetPullRequest(user, project string, id int) PullRequest {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls/%d",
		user,
		project,
		id)
	body, _ := c.load(url, nil)
	var pull PullRequest
	json.Unmarshal(body, &pull)
	return pull
}

func (c *ApiClient) GetPullRequestComments(user, project string, pull int) CommentList {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/issues/%d/comments",
		user,
		project,
		pull)
	body, _ := c.load(url, nil)
	var comments CommentList
	json.Unmarshal(body, &comments)
	return comments
}

func (c *ApiClient) GetCommitComments(user, project, sha string) CommentList {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/commits/%s/comments",
		user,
		project,
		sha)
	body, _ := c.load(url, nil)
	var comments []Comment
	json.Unmarshal(body, &comments)
	return comments
}

func (c *ApiClient) CommentOnPullRequest(user, project string, pull int, body string) Comment {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/issues/%d/comments",
		user,
		project,
		pull)
	req := &BodyOnlyComment{Body: body}
	var err error
	var data []byte
	if data, err = json.Marshal(req); err != nil {
	    panic(err)
	}
	result, _ := c.load(url, data)
	var comment Comment
	json.Unmarshal(result, &comment)
	return comment
}

func (c *ApiClient) CreatePullRequest(user, project, title, body, head, base string) PullRequest {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls",
		user,
		project)
	req := &createPullRequestRequest{
	    Title: title,
	    Body: body,
	    Base: base,
	    Head: head,
	}
	var err error
	var data []byte
	if data, err = json.Marshal(req); err != nil {
	    panic(err)
	}
	result, _ := c.load(url, data)
	var pull PullRequest
	json.Unmarshal(result, &pull)
	return pull
}

func (c *ApiClient) CreatePullRequestFromIssue(user, project string, issue int, head, base string) PullRequest {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls",
		user,
		project)
	req := &createPullRequestFromIssueRequest{
	    Issue: issue,
	    Base: base,
	    Head: head,
	}
	var err error
	var data []byte
	if data, err = json.Marshal(req); err != nil {
	    panic(err)
	}
	result, _ := c.load(url, data)
	var pull PullRequest
	json.Unmarshal(result, &pull)
	return pull
}