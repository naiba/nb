package cmd

import (
	"testing"
)

func TestGetGitDirectoryName(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{"ssh with .git", "git@github.com:user/xxx.git", "xxx", false},
		{"https with .git", "https://github.com/user/xxx.git", "xxx", false},
		{"ssh without .git", "git@github.com:user/xxx", "xxx", false},
		{"https without .git", "https://github.com/user/xxx", "xxx", false},
		{"bare with .git", "xxx.git", "xxx", false},
		{"bare without .git", "xxx", "xxx", false},
		{"nested path", "https://gitlab.com/group/sub/repo.git", "repo", false},
		{"ssh nested", "git@gitlab.com:group/sub/repo", "repo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := getGitDirectoryName.FindStringSubmatch(tt.url)
			if tt.wantErr {
				if len(matched) >= 2 {
					t.Errorf("expected no match but got %q", matched[1])
				}
				return
			}
			if len(matched) < 2 {
				t.Fatalf("expected match for %q but got none", tt.url)
			}
			if matched[1] != tt.want {
				t.Errorf("got %q, want %q", matched[1], tt.want)
			}
		})
	}
}

func TestGitSalonDirName(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"url only", []string{"git@github.com:user/xxx.git"}, "xxx"},
		{"url with custom dir", []string{"git@github.com:user/xxx.git", "xxx-yyy"}, "xxx-yyy"},
		{"bare url with custom dir", []string{"xxx.git", "my-dir"}, "my-dir"},
		{"https with custom dir", []string{"https://github.com/user/repo.git", "local-name"}, "local-name"},
		{"url without .git with custom dir", []string{"git@github.com:user/repo", "custom"}, "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nonFlagArgs []string
			for _, arg := range tt.args {
				if len(arg) > 0 && arg[0] != '-' {
					nonFlagArgs = append(nonFlagArgs, arg)
				}
			}

			var dirName string
			if len(nonFlagArgs) >= 2 {
				dirName = nonFlagArgs[len(nonFlagArgs)-1]
			} else if len(nonFlagArgs) == 1 {
				matched := getGitDirectoryName.FindStringSubmatch(nonFlagArgs[0])
				if len(matched) < 2 {
					t.Fatalf("failed to parse dir name from %q", nonFlagArgs[0])
				}
				dirName = matched[1]
			}

			if dirName != tt.want {
				t.Errorf("got %q, want %q", dirName, tt.want)
			}
		})
	}
}
