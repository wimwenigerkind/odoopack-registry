package git

import (
	"bufio"
	"bytes"
	"fmt"
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

func LsRemote(url string, env []string) ([]Ref, error) {
	if err := checkRemote(url); err != nil {
		return nil, err
	}
	out, err := outputGit("", env, "ls-remote", "--tags", "--heads", url)
	if err != nil {
		return nil, fmt.Errorf("git ls-remote: %w", err)
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
