package unwall

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// fake is an unwallctl that records the commands that are executed as root
type fake struct {
	status, dns, profile string
	fail                 string
	root                 []string
}

func (f *fake) run(_ context.Context, root bool, args ...string) (string, error) {
	cmd := strings.Join(args, " ")
	if root {
		f.root = append(f.root, cmd)
		if f.fail != "" && strings.HasPrefix(cmd, f.fail) {
			return "", errors.New("failed: " + cmd)
		}
		if strings.HasPrefix(cmd, "dns enable") {
			f.dns = "backend=dnscrypt\nencrypted=1\n"
		}
		return "", nil
	}
	switch cmd {
	case "status":
		return f.status, nil
	case "dns status":
		return f.dns, nil
	case "profile show":
		return f.profile, nil
	}
	return "", errors.New("unknown command: " + cmd)
}

func TestSetup(t *testing.T) {
	const off = "backend=none\nencrypted=0\n"
	const on = "backend=dnscrypt\nencrypted=1\n"
	cases := map[string]struct {
		f        fake
		blocked  []string
		resolves bool
		wantRoot []string
		wantErr  string
	}{
		"everything off": {
			f:        fake{status: "running=0\n", dns: off, profile: "network=wifi:eduroam\nsaved=\n"},
			resolves: true,
			wantRoot: []string{"dns enable quad9", "start", "profile save"},
		},
		"nothing to do": {
			f:        fake{status: "running=1\n", dns: on, profile: "saved=kablonet\n"},
			resolves: true,
		},
		"first provider not reachable": {
			f:        fake{status: "running=1\n", dns: off, profile: "saved=x\n"},
			blocked:  []string{"quad9"},
			resolves: true,
			wantRoot: []string{"dns enable cloudflare"},
		},
		"no provider reachable": {
			f:       fake{status: "running=0\n", dns: off},
			blocked: []string{"quad9", "cloudflare", "google"},
			wantErr: "none of the encrypted DNS providers can be reached on this network",
		},
		"does not resolve": {
			f:        fake{status: "running=1\n", dns: off},
			wantRoot: []string{"dns enable quad9", "dns disable"},
			wantErr:  "encrypted DNS does not work on this network, it has been turned off again",
		},
		"not authorized": {
			f:        fake{status: "running=1\n", dns: off, fail: "dns enable"},
			wantRoot: []string{"dns enable quad9"},
			wantErr:  "failed: dns enable quad9",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := c.f
			client := Client{
				run:       f.run,
				reachable: func(_ context.Context, p provider) bool { return !slices.Contains(c.blocked, p.name) },
				resolves:  func(context.Context) bool { return c.resolves },
			}
			got := ""
			if err := client.Setup(context.Background(), func(string) {}); err != nil {
				got = err.Error()
			}
			if got != c.wantErr {
				t.Fatalf("error not equal. Got: %q, want: %q", got, c.wantErr)
			}
			if !reflect.DeepEqual(f.root, c.wantRoot) {
				t.Fatalf("root commands not equal. Got: %v, want: %v", f.root, c.wantRoot)
			}
		})
	}
}

func TestTrusted(t *testing.T) {
	p := filepath.Join(t.TempDir(), "unwallctl")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o777); err != nil {
		t.Fatal(err)
	}
	if trusted(p) {
		t.Fatal("a file that others can write should not be trusted")
	}
	if trusted(p + "-missing") {
		t.Fatal("a missing file should not be trusted")
	}
}
