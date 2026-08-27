package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type RefType string

const (
	RefTag    RefType = "tag"
	RefBranch RefType = "branch"
)

type Ref struct {
	Type RefType
	Name string
	SHA  string
}

func LsRemote(url string) ([]Ref, error) {
	cmd := exec.Command("git", "ls-remote", "--tags", "--heads", url)
	cmd.Env = append(os.Environ(),
		"GIT_ALLOW_PROTOCOL=https:ssh",
		"GIT_PROTOCOL_FROM_USER=0",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	type tagEntry struct {
		direct string
		peeled string
	}
	tags := map[string]*tagEntry{}
	var refs []Ref

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sha, ref := fields[0], fields[1]
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			refs = append(refs, Ref{
				Type: RefBranch,
				Name: strings.TrimPrefix(ref, "refs/heads/"),
				SHA:  sha,
			})
		case strings.HasPrefix(ref, "refs/tags/"):
			name := strings.TrimPrefix(ref, "refs/tags/")
			if before, ok := strings.CutSuffix(name, "^{}"); ok {
				name = before
				e, ok := tags[name]
				if !ok {
					e = &tagEntry{}
					tags[name] = e
				}
				e.peeled = sha
			} else {
				e, ok := tags[name]
				if !ok {
					e = &tagEntry{}
					tags[name] = e
				}
				e.direct = sha
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for name, e := range tags {
		sha := e.peeled
		if sha == "" {
			sha = e.direct
		}
		refs = append(refs, Ref{Type: RefTag, Name: name, SHA: sha})
	}
	return refs, nil
}
