// Package unwall integrates with Unwall: https://github.com/WinTone01/Unwall
// Everything here is optional, nothing is changed if Unwall is not installed or if the user does not ask for it
package unwall

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/exp/slog"
)

// ctlPaths are the locations where unwallctl can be installed
// We do not search $PATH because the command is executed as root
var ctlPaths = []string{"/usr/bin/unwallctl", "/usr/local/bin/unwallctl"}

// commandTimeout is the maximum time for a single unwallctl command
// This includes the time that the user needs to enter a password
const commandTimeout = 5 * time.Minute

// provider is an encrypted DNS provider that Unwall supports
type provider struct {
	name string
	// address is an IP, such that DNS is not needed to test if it is reachable
	address string
	// host is the name that the certificate must be valid for
	host string
}

var providers = []provider{
	{name: "quad9", address: "9.9.9.9", host: "dns.quad9.net"},
	{name: "cloudflare", address: "1.1.1.1", host: "cloudflare-dns.com"},
	{name: "google", address: "8.8.8.8", host: "dns.google"},
}

// Client configures Unwall for the current network
type Client struct {
	// run executes unwallctl, as root if root is true
	run func(ctx context.Context, root bool, args ...string) (string, error)
	// reachable returns if the encrypted DNS provider can be reached
	reachable func(ctx context.Context, p provider) bool
	// resolves returns if the system can resolve names
	resolves func(ctx context.Context) bool
}

// Find returns a client if Unwall is installed, if it is not available it returns nil
func Find() *Client {
	pkexec, err := exec.LookPath("pkexec")
	if err != nil {
		return nil
	}
	for _, p := range ctlPaths {
		if trusted(p) {
			return &Client{
				run:       runner(pkexec, p),
				reachable: reachable,
				resolves:  resolves,
			}
		}
	}
	return nil
}

// trusted returns if the path is an executable that only root can change
func trusted(p string) bool {
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 || fi.Mode().Perm()&0o022 != 0 {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && st.Uid == 0
}

func runner(pkexec string, ctl string) func(context.Context, bool, ...string) (string, error) {
	return func(ctx context.Context, root bool, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, commandTimeout)
		defer cancel()
		argv := append([]string{ctl}, args...)
		if root {
			argv = append([]string{pkexec}, argv...)
		}
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		// unwallctl parses the output of localized tools,
		// with the C locale a network is identified the same way as when it runs as root
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		var stdout, stderr bytes.Buffer
		cmd.Stdin = os.Stdin
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			return stdout.String(), nil
		}
		slog.Error("unwallctl failed", "args", args, "error", err, "stderr", stderr.String())
		var exit *exec.ExitError
		// see the return value section in man pkexec
		if root && errors.As(err, &exit) && (exit.ExitCode() == 126 || exit.ExitCode() == 127) {
			return "", errors.New("not authorized to change the Unwall settings")
		}
		return "", fmt.Errorf("unwallctl %s failed: %w", strings.Join(args, " "), err)
	}
}

// reachable tests the DNS over HTTPS endpoint of a provider
func reachable(ctx context.Context, p provider) bool {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// this is a query for the name servers of the root, the answer does not matter
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+p.host+"/dns-query?dns=AAABAAABAAAAAAAAAAACAAE", nil)
	if err != nil {
		return false
	}
	transport := &http.Transport{
		ForceAttemptHTTP2: true,
		TLSClientConfig:   &tls.Config{ServerName: p.host, MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, _ string, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(p.address, "443"))
		},
	}
	defer transport.CloseIdleConnections()
	res, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		slog.Debug("encrypted DNS provider is not reachable", "provider", p.name, "error", err)
		return false
	}
	defer res.Body.Close() //nolint:errcheck
	return res.StatusCode == http.StatusOK
}

// resolves tests if names can be resolved, it tries a few times because the resolver could still be starting
func resolves(ctx context.Context) bool {
	for i := 0; i < 5; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(2 * time.Second):
			}
		}
		tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		addrs, err := net.DefaultResolver.LookupHost(tctx, "example.com")
		cancel()
		if err == nil && len(addrs) > 0 {
			return true
		}
	}
	return false
}

// parse parses the key=value output of unwallctl
func parse(out string) map[string]string {
	ret := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			ret[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return ret
}

func (c *Client) query(ctx context.Context, args ...string) (map[string]string, error) {
	out, err := c.run(ctx, false, args...)
	if err != nil {
		return nil, err
	}
	return parse(out), nil
}

// enableDNS turns on encrypted DNS, it is turned off again if names can no longer be resolved
func (c *Client) enableDNS(ctx context.Context) error {
	var chosen string
	for _, p := range providers {
		if c.reachable(ctx, p) {
			chosen = p.name
			break
		}
	}
	// without a reachable provider nothing would resolve anymore
	if chosen == "" {
		return errors.New("none of the encrypted DNS providers can be reached on this network")
	}
	_, err := c.run(ctx, true, "dns", "enable", chosen)
	if err == nil && c.resolves(ctx) {
		return nil
	}
	// if we could not get the status we do not know if it is enabled, so then we also turn it off
	st, serr := c.query(ctx, "dns", "status")
	if serr != nil || st["backend"] != "none" {
		if _, derr := c.run(ctx, true, "dns", "disable"); derr != nil {
			return fmt.Errorf("encrypted DNS does not work and it could not be turned off, run 'sudo unwallctl dns disable': %w", derr)
		}
	}
	if err != nil {
		return err
	}
	return errors.New("encrypted DNS does not work on this network, it has been turned off again")
}

// Setup configures Unwall for the network that we are currently connected to:
// - Encrypted DNS is turned on, if it is not already
// - The engine is started, if it is not already
// - The strategy is saved for this network, if there is none saved yet
// progress is called with a text that can be shown to the user
func (c *Client) Setup(ctx context.Context, progress func(string)) error {
	progress("Checking Unwall...")
	dns, err := c.query(ctx, "dns", "status")
	if err != nil {
		return err
	}
	// this also covers the case where it is enabled, but not working
	if dns["encrypted"] != "1" {
		progress("Turning on encrypted DNS...")
		if err := c.enableDNS(ctx); err != nil {
			return err
		}
	}
	status, err := c.query(ctx, "status")
	if err != nil {
		return err
	}
	if status["running"] != "1" {
		progress("Starting the Unwall engine...")
		if _, err := c.run(ctx, true, "start"); err != nil {
			return err
		}
	}
	prof, err := c.query(ctx, "profile", "show")
	if err != nil {
		return err
	}
	if prof["saved"] == "" {
		progress("Saving the strategy for this network...")
		if _, err := c.run(ctx, true, "profile", "save"); err != nil {
			return err
		}
	}
	return nil
}
