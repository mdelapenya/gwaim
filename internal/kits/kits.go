// Package kits discovers Docker Sandbox kits published in
// docker/sbx-kits-contrib so they can be applied to a sandbox via
// `sbx run --kit "git+https://github.com/docker/sbx-kits-contrib.git#dir=<name>"`.
package kits

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

// ContribRepoURL is the git+ URL passed to `sbx --kit` to point at the
// kits-contrib repository.
const ContribRepoURL = "git+https://github.com/docker/sbx-kits-contrib.git"

// Kind values from spec.yaml.
const (
	KindAgent = "agent"
	KindMixin = "mixin"
)

// nonKitDirs are top-level directories that are not kit definitions.
var nonKitDirs = map[string]struct{}{
	".github": {},
	"spec":    {},
	"tck":     {},
}

// Kit is a sandbox kit discovered in docker/sbx-kits-contrib.
// Fields mirror the relevant top-level keys in each kit's spec.yaml.
type Kit struct {
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind"`
	DisplayName string `yaml:"displayName"`
	Description string `yaml:"description"`
	Extends     string `yaml:"extends"`
}

// GitURL returns the value to pass to sbx's --kit flag for this kit.
func (k Kit) GitURL() string {
	return URL(k.Name)
}

// URL returns the --kit reference for a kit by name.
func URL(name string) string {
	return ContribRepoURL + "#dir=" + name
}

// FetchAvailable lists every kit in docker/sbx-kits-contrib and splits them
// into (agents, mixins) by spec.yaml `kind`. Discovery uses `gh api`, which
// reuses the user's existing GitHub authentication (matching how
// internal/provider/github.go talks to GitHub).
//
// Directories without a spec.yaml or with an unknown kind are skipped.
func FetchAvailable(ctx context.Context) (agents, mixins []Kit, err error) {
	dirs, err := listKitDirs(ctx)
	if err != nil {
		return nil, nil, err
	}

	results := make([]Kit, len(dirs))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for i, dir := range dirs {
		idx, name := i, dir
		g.Go(func() error {
			k, ok, ferr := fetchKitSpec(gctx, name)
			if ferr != nil {
				return ferr
			}
			if ok {
				results[idx] = k
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	for _, k := range results {
		switch k.Kind {
		case KindAgent:
			agents = append(agents, k)
		case KindMixin:
			mixins = append(mixins, k)
		}
	}
	sortByDisplay(agents)
	sortByDisplay(mixins)
	return agents, mixins, nil
}

// HeadRef returns the short (7-char) commit SHA of the kits-contrib repo's
// default branch. biomelab captures this when a user installs kits so the
// main card can show "name@<short-sha>" as a version. The install URL
// itself is not pinned — kits are always fetched from `main`.
func HeadRef(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "api",
		"repos/docker/sbx-kits-contrib/commits/main", "--jq", ".sha")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kits HEAD ref: %w (%s)", err, ghStderr(err))
	}
	sha := strings.TrimSpace(string(out))
	if len(sha) > 7 {
		sha = sha[:7]
	}
	return sha, nil
}

// FilterMixinsForAgent returns the subset of mixins compatible with the given
// sandbox agent. A mixin is compatible if its `extends` field is empty or
// equal to the agent name.
func FilterMixinsForAgent(mixins []Kit, agent string) []Kit {
	out := make([]Kit, 0, len(mixins))
	for _, m := range mixins {
		if m.Extends == "" || m.Extends == agent {
			out = append(out, m)
		}
	}
	return out
}

// listKitDirs returns the candidate top-level directory names in the
// kits-contrib repo (everything that's a dir and not in nonKitDirs).
func listKitDirs(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "gh", "api", "repos/docker/sbx-kits-contrib/contents")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list kits: %w (%s)", err, ghStderr(err))
	}
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parse kit listing: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.Type != "dir" {
			continue
		}
		if _, skip := nonKitDirs[e.Name]; skip {
			continue
		}
		dirs = append(dirs, e.Name)
	}
	return dirs, nil
}

// fetchKitSpec downloads and parses spec.yaml for a single kit directory.
// Returns ok=false (without error) if the directory has no spec.yaml.
func fetchKitSpec(ctx context.Context, dir string) (Kit, bool, error) {
	cmd := exec.CommandContext(ctx, "gh", "api",
		"repos/docker/sbx-kits-contrib/contents/"+dir+"/spec.yaml")
	out, err := cmd.Output()
	if err != nil {
		stderr := ghStderr(err)
		if strings.Contains(stderr, "Not Found") || strings.Contains(stderr, "HTTP 404") {
			return Kit{}, false, nil
		}
		return Kit{}, false, fmt.Errorf("fetch %s/spec.yaml: %w (%s)", dir, err, stderr)
	}
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return Kit{}, false, fmt.Errorf("parse %s/spec.yaml response: %w", dir, err)
	}
	raw, err := decodeContent(resp.Content, resp.Encoding)
	if err != nil {
		return Kit{}, false, fmt.Errorf("decode %s/spec.yaml: %w", dir, err)
	}
	k, err := ParseSpec(raw)
	if err != nil {
		return Kit{}, false, fmt.Errorf("parse %s/spec.yaml: %w", dir, err)
	}
	if k.Name == "" {
		k.Name = dir
	}
	return k, true, nil
}

// ParseSpec unmarshals a spec.yaml into a Kit.
func ParseSpec(raw []byte) (Kit, error) {
	var k Kit
	if err := yaml.Unmarshal(raw, &k); err != nil {
		return Kit{}, err
	}
	return k, nil
}

func decodeContent(content, encoding string) ([]byte, error) {
	if encoding != "base64" {
		return []byte(content), nil
	}
	// gh api returns base64 with newlines.
	cleaned := strings.ReplaceAll(content, "\n", "")
	return base64.StdEncoding.DecodeString(cleaned)
}

// ghStderr extracts the stderr of an *exec.ExitError, if any.
func ghStderr(err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return strings.TrimSpace(string(ee.Stderr))
	}
	return ""
}

func sortByDisplay(ks []Kit) {
	sort.Slice(ks, func(i, j int) bool {
		ai := ks[i].DisplayName
		if ai == "" {
			ai = ks[i].Name
		}
		aj := ks[j].DisplayName
		if aj == "" {
			aj = ks[j].Name
		}
		return strings.ToLower(ai) < strings.ToLower(aj)
	})
}
