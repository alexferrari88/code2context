package gitutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"http url", "http://example.com/repo.git", true},
		{"https url", "https://example.com/repo.git", true},
		{"git@ url", "git@github.com:user/repo.git", true},
		{"ssh url", "ssh://git@github.com/user/repo.git", true},
		{"url with .git suffix", "https://example.com/repo.git", true},
		{"url without .git suffix but common prefix https", "https://example.com/repo", true},
		{"url without .git suffix but common prefix http", "http://example.com/repo", true},
		{"local path with .git", "/path/to/myrepo.git", false}, // Corrected: No longer true based on IsGitURL change
		{"local path without .git", "/path/to/myrepo", false},
		{"scp like short path", "user@host:project.git", false},
		{"empty string", "", false},
		{"just .git", ".git", false}, // Corrected: No longer true
		{"random string", "randomstring", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := IsGitURL(tc.path)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestGetRepoNameFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"https url with .git", "https://example.com/foo/myrepo.git", "myrepo"},
		{"https url without .git", "https://example.com/foo/myotherrepo", "myotherrepo"},
		{"http url with .git", "http://example.com/bar/another.git", "another"},
		{"git@ url", "git@github.com:user/project.git", "project"},
		{"url with trailing slash", "https://example.com/myrepo/", "myrepo"},
		{"url with multiple .git parts", "https://example.com/my.repo.git/actual.git", "actual"},
		{"url with no slashes and .git", "myrepository.git", "myrepository"},
		{"url with no slashes no .git", "myrepository", "myrepository"},
		{"empty url", "", "repository"}, 
		{"url with only domain https", "https://example.com", "example.com"},
		{"url with only domain http", "http://example.com", "example.com"},
		{"url git@ with host only", "git@github.com:", "repository"}, 
		{"url git@ with host and slash", "git@github.com:/", "repository"}, 
		{"url git@ with host and user", "git@github.com:user", "user"}, 
		{"url with just slashes", "///", "repository"}, 
		{"url with http and slashes", "http://///", "repository"}, 
		{"url with https and slashes", "https://///", "repository"}, 
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := getRepoNameFromURL(tc.url)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
