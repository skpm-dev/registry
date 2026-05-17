package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrBranchExists is returned by CreateBranch when the branch already exists on GitHub.
var ErrBranchExists = errors.New("branch already exists")

// RepoClient performs GitHub API operations against a specific repository.
type RepoClient struct {
	token string
	owner string
	repo  string
}

func NewRepoClient(token, owner, repo string) *RepoClient {
	return &RepoClient{token: token, owner: owner, repo: repo}
}

type fileResponse struct {
	SHA     string `json:"sha"`
	Content string `json:"content"`
}

type refResponse struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

// GetFileSHA returns the SHA of an existing file, or empty string if the file does not exist.
func (c *RepoClient) GetFileSHA(path string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", c.owner, c.repo, path)

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	c.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not fetch file SHA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github returned %d fetching %s", resp.StatusCode, path)
	}

	var f fileResponse
	if err := json.NewDecoder(resp.Body).Decode(&f); err != nil {
		return "", err
	}

	return f.SHA, nil
}

// GetBranchSHA returns the HEAD commit SHA of a branch.
func (c *RepoClient) GetBranchSHA(branch string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", c.owner, c.repo, branch)

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	c.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not fetch branch SHA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github returned %d fetching branch %s", resp.StatusCode, branch)
	}

	var ref refResponse
	if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
		return "", err
	}

	return ref.Object.SHA, nil
}

// CreateBranch creates a new branch pointing at fromSHA.
// Returns ErrBranchExists if the branch already exists on GitHub; callers that
// want to replace an existing branch should call DeleteBranch first.
func (c *RepoClient) CreateBranch(name, fromSHA string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs", c.owner, c.repo)

	body := map[string]string{
		"ref": "refs/heads/" + name,
		"sha": fromSHA,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not create branch %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		return nil
	}
	if resp.StatusCode == http.StatusUnprocessableEntity {
		return ErrBranchExists
	}

	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("could not create branch %s: github returned %d: %s", name, resp.StatusCode, string(b))
}

// HasOpenPR returns true if there is an open pull request for the given branch.
func (c *RepoClient) HasOpenPR(branch string) (bool, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls?head=%s:%s&state=open",
		c.owner, c.repo, c.owner, branch,
	)

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	c.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("github returned %d listing PRs: %s", resp.StatusCode, string(b))
	}

	var prs []struct{}
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return false, err
	}

	return len(prs) > 0, nil
}

// DeleteBranch deletes a branch.
func (c *RepoClient) DeleteBranch(name string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", c.owner, c.repo, name)

	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	c.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github returned %d deleting branch %s: %s", resp.StatusCode, name, string(b))
	}

	return nil
}

// CommitFile creates or updates a file on a branch.
func (c *RepoClient) CommitFile(branch, path, content, message string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", c.owner, c.repo, path)

	existingSHA, err := c.GetFileSHA(path)
	if err != nil {
		return err
	}

	body := map[string]string{
		"message": message,
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
		"branch":  branch,
	}

	if existingSHA != "" {
		body["sha"] = existingSHA
	}

	if err := c.doRequest(http.MethodPut, url, body, http.StatusOK, http.StatusCreated); err != nil {
		return fmt.Errorf("could not commit %s: %w", path, err)
	}

	return nil
}

// ListDirectory returns the paths of all files directly inside a directory.
// Returns nil if the directory does not exist.
func (c *RepoClient) ListDirectory(path string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", c.owner, c.repo, path)

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	c.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not list directory: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned %d listing %s", resp.StatusCode, path)
	}

	var items []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	var paths []string
	for _, item := range items {
		if item.Type == "file" {
			paths = append(paths, item.Path)
		}
	}
	return paths, nil
}

// DeleteFile deletes a file on the default branch. Returns nil if the file does not exist.
func (c *RepoClient) DeleteFile(path, message string) error {
	sha, err := c.GetFileSHA(path)
	if err != nil {
		return err
	}
	if sha == "" {
		return nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", c.owner, c.repo, path)
	body := map[string]string{
		"message": message,
		"sha":     sha,
	}
	return c.doRequest(http.MethodDelete, url, body, http.StatusOK)
}

// OpenPR opens a pull request from branch into base.
func (c *RepoClient) OpenPR(branch, base, title, body string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", c.owner, c.repo)

	payload := map[string]string{
		"title": title,
		"body":  body,
		"head":  branch,
		"base":  base,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not open PR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github returned %d opening PR: %s", resp.StatusCode, string(b))
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.HTMLURL, nil
}

func (c *RepoClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func (c *RepoClient) doRequest(method, url string, body map[string]string, expectedStatuses ...int) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, _ := http.NewRequest(method, url, bytes.NewReader(data))
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for _, s := range expectedStatuses {
		if resp.StatusCode == s {
			return nil
		}
	}

	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("github returned %d: %s", resp.StatusCode, string(b))
}
