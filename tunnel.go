package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// TunnelRuntime owns every live tunnel. Each tunnel is an SSH connection of
// its own plus a local listener, so one tunnel failing or being stopped never
// affects the others.
//
// This talks SSH directly (golang.org/x/crypto/ssh) rather than shelling out
// to the `ssh` binary: the binary can only read a password from a real
// terminal, which a GUI app doesn't have. Doing it in-process also means
// teammates don't need an OpenSSH client installed at all.
type TunnelRuntime struct {
	mu      sync.Mutex
	running map[string]*liveTunnel

	onExit func(tunnelID string, err error)
	onLog  func(tunnelID string, line string)
}

type liveTunnel struct {
	listener net.Listener
	client   *ssh.Client
	stopping bool

	mu      sync.Mutex
	pinged  bool
	latency time.Duration
	pingErr error

	done     chan struct{}
	doneOnce sync.Once
}

// stopPinging ends this tunnel's heartbeat loop. Safe to call more than
// once (Stop and the client.Wait goroutine can both reach it).
func (l *liveTunnel) stopPinging() {
	l.doneOnce.Do(func() { close(l.done) })
}

// pingInterval is how often a running tunnel's SSH connection gets a
// heartbeat request to measure round-trip latency to the bastion.
const pingInterval = 5 * time.Second

// TunnelHealth is a snapshot of a running tunnel's most recent heartbeat.
type TunnelHealth struct {
	Running bool
	Pinged  bool // false until the first heartbeat has come back (or failed)
	Latency time.Duration
	Err     error
}

// Health reports the latest heartbeat reading for a tunnel, or a zero-value
// TunnelHealth if it isn't running.
func (r *TunnelRuntime) Health(id string) TunnelHealth {
	r.mu.Lock()
	live, ok := r.running[id]
	r.mu.Unlock()
	if !ok {
		return TunnelHealth{}
	}

	live.mu.Lock()
	defer live.mu.Unlock()
	return TunnelHealth{Running: true, Pinged: live.pinged, Latency: live.latency, Err: live.pingErr}
}

func NewTunnelRuntime(onExit func(string, error), onLog func(string, string)) *TunnelRuntime {
	return &TunnelRuntime{
		running: make(map[string]*liveTunnel),
		onExit:  onExit,
		onLog:   onLog,
	}
}

func (r *TunnelRuntime) IsRunning(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.running[id]
	return ok
}

func portOrDefault(p string) string {
	if p == "" {
		return "22"
	}
	return p
}

func (r *TunnelRuntime) log(tunnelID, format string, args ...any) {
	if r.onLog != nil {
		r.onLog(tunnelID, fmt.Sprintf(format, args...))
	}
}

// Start dials the project's bastion, opens the local port, and forwards every
// connection on it through that SSH session to the tunnel's target.
func (r *TunnelRuntime) Start(t TunnelConfig, p Project, auth []ssh.AuthMethod) error {
	r.mu.Lock()
	if _, ok := r.running[t.ID]; ok {
		r.mu.Unlock()
		return fmt.Errorf("tunnel already running")
	}
	r.mu.Unlock()

	bastion := net.JoinHostPort(p.BastionHost, portOrDefault(p.BastionPort))
	r.log(t.ID, "connecting to %s as %s", bastion, p.BastionUser)

	client, err := r.dial(t.ID, bastion, p, auth)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("localhost", t.LocalPort))
	if err != nil {
		client.Close()
		return fmt.Errorf("listen on local port %s: %w", t.LocalPort, err)
	}

	live := &liveTunnel{listener: listener, client: client, done: make(chan struct{})}
	r.mu.Lock()
	r.running[t.ID] = live
	r.mu.Unlock()

	target := net.JoinHostPort(t.TargetHost, t.TargetPort)
	r.log(t.ID, "listening on localhost:%s → %s", t.LocalPort, target)

	go r.acceptLoop(t.ID, live, target)
	go r.pingLoop(live)

	// Notice the SSH session dropping (network loss, server restart) even
	// while nothing is using the local port.
	go func() {
		waitErr := client.Wait()

		r.mu.Lock()
		// Compare by identity, not just presence: a fast Stop()+Start() on
		// the same tunnel ID can already have a *new* liveTunnel registered
		// under t.ID by the time this old client's Wait() returns. Checking
		// only "is something tracked at this ID" would delete that new
		// tunnel's entry and report its exit using this stale waitErr.
		current, stillTracked := r.running[t.ID]
		sameInstance := stillTracked && current == live
		intentional := sameInstance && current.stopping
		if sameInstance {
			delete(r.running, t.ID)
		}
		r.mu.Unlock()

		live.stopPinging()
		listener.Close()

		if sameInstance && !intentional && r.onExit != nil {
			r.onExit(t.ID, waitErr)
		}
	}()

	return nil
}

// dial connects to the bastion, working around a known rough edge: Go does
// not tell the server which host key types we have on file, so a server that
// offers ed25519 while known_hosts only holds an older type looks like a key
// mismatch. When that happens we retry, this time asking for exactly the
// algorithms known_hosts has for that host.
func (r *TunnelRuntime) dial(tunnelID, bastion string, p Project, auth []ssh.AuthMethod) (*ssh.Client, error) {
	newConfig := func(cb ssh.HostKeyCallback, algos []string) *ssh.ClientConfig {
		return &ssh.ClientConfig{
			User:              p.BastionUser,
			Auth:              auth,
			HostKeyCallback:   cb,
			HostKeyAlgorithms: algos,
			Timeout:           15 * time.Second,
		}
	}

	if p.SkipHostKeyCheck {
		r.log(tunnelID, "host key verification is off for this project")
		client, err := ssh.Dial("tcp", bastion, newConfig(ssh.InsecureIgnoreHostKey(), nil))
		if err != nil {
			return nil, fmt.Errorf("connect %s: %w", bastion, err)
		}
		return client, nil
	}

	var mismatch *knownhosts.KeyError
	cb, err := hostKeyChecker(r, tunnelID, &mismatch)
	if err != nil {
		return nil, err
	}

	client, err := ssh.Dial("tcp", bastion, newConfig(cb, nil))
	if err == nil {
		return client, nil
	}

	// Retry once with the key types known_hosts actually holds for this host.
	if mismatch != nil && len(mismatch.Want) > 0 {
		var algos []string
		seen := map[string]bool{}
		for _, k := range mismatch.Want {
			if typ := k.Key.Type(); !seen[typ] {
				seen[typ] = true
				algos = append(algos, typ)
			}
		}
		r.log(tunnelID, "retrying with the host key types in known_hosts: %s", strings.Join(algos, ", "))

		mismatch = nil
		client, retryErr := ssh.Dial("tcp", bastion, newConfig(cb, algos))
		if retryErr == nil {
			return client, nil
		}
		if mismatch != nil && len(mismatch.Want) > 0 {
			return nil, fmt.Errorf("host key for %s does not match ~/.ssh/known_hosts. "+
				"If this server was rebuilt, remove its old line with: ssh-keygen -R %s — "+
				"or tick \"Skip host key check\" in this project's settings", bastion, p.BastionHost)
		}
		return nil, fmt.Errorf("connect %s: %w", bastion, retryErr)
	}

	return nil, fmt.Errorf("connect %s: %w", bastion, err)
}

func (r *TunnelRuntime) acceptLoop(tunnelID string, live *liveTunnel, target string) {
	for {
		local, err := live.listener.Accept()
		if err != nil {
			return // listener closed — Stop() or the session dropped
		}

		go func() {
			defer local.Close()

			remote, err := live.client.Dial("tcp", target)
			if err != nil {
				r.log(tunnelID, "cannot reach %s: %v", target, err)
				return
			}
			defer remote.Close()

			done := make(chan struct{}, 2)
			go func() { io.Copy(remote, local); done <- struct{}{} }()
			go func() { io.Copy(local, remote); done <- struct{}{} }()
			<-done // either side closing ends this connection
		}()
	}
}

// pingLoop measures round-trip latency to the bastion every pingInterval,
// for as long as the tunnel stays in r.running. It reads the very first
// value immediately, so the UI doesn't sit blank for a full interval after
// Start.
func (r *TunnelRuntime) pingLoop(live *liveTunnel) {
	r.ping(live)

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-live.done:
			return
		case <-ticker.C:
			r.ping(live)
		}
	}
}

// ping sends a no-op SSH global request and times the reply. It never
// touches the tunnel's actual target (the database, API, ...) — only the
// bastion connection itself — so it can't disturb whatever's listening
// there.
func (r *TunnelRuntime) ping(live *liveTunnel) {
	start := time.Now()
	_, _, err := live.client.SendRequest("keepalive@openssh.com", true, nil)
	rtt := time.Since(start)

	live.mu.Lock()
	live.pinged = true
	live.latency = rtt
	live.pingErr = err
	live.mu.Unlock()
}

func (r *TunnelRuntime) Stop(id string) error {
	r.mu.Lock()
	live, ok := r.running[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("tunnel is not running")
	}
	live.stopping = true // tells the client.Wait goroutine this exit was deliberate
	delete(r.running, id)
	r.mu.Unlock()

	live.stopPinging()
	live.listener.Close()
	return live.client.Close()
}

func (r *TunnelRuntime) StopAll() {
	r.mu.Lock()
	ids := make([]string, 0, len(r.running))
	for id := range r.running {
		ids = append(ids, id)
	}
	r.mu.Unlock()

	for _, id := range ids {
		_ = r.Stop(id)
	}
}

// ---------- auth helpers ----------

// PasswordAuth answers both plain password auth and keyboard-interactive,
// since many servers offer only the latter for what users think of as "the
// password".
func PasswordAuth(password string) []ssh.AuthMethod {
	return []ssh.AuthMethod{
		ssh.Password(password),
		ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = password
			}
			return answers, nil
		}),
	}
}

// KeyAuth loads a private key file. passphrase may be empty.
func KeyAuth(path, passphrase string) ([]ssh.AuthMethod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key %s: %w", path, err)
	}

	var signer ssh.Signer
	if passphrase == "" {
		signer, err = ssh.ParsePrivateKey(data)
		if err != nil {
			if _, ok := err.(*ssh.PassphraseMissingError); ok {
				return nil, fmt.Errorf("key %s is passphrase-protected — use password auth or an unprotected key", filepath.Base(path))
			}
			return nil, fmt.Errorf("parse key %s: %w", filepath.Base(path), err)
		}
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("parse key %s: %w", filepath.Base(path), err)
		}
	}

	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

// hostKeyChecker verifies against ~/.ssh/known_hosts when the host is listed
// there, and accepts (with a log line) the first time a host is seen — the
// same trade-off as ssh's StrictHostKeyChecking=accept-new. Mismatches are
// reported through mismatch so the caller can retry with matching algorithms.
func hostKeyChecker(r *TunnelRuntime, tunnelID string, mismatch **knownhosts.KeyError) (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		r.log(tunnelID, "could not find home directory — accepting host keys without verification")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	path := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(path); err != nil {
		r.log(tunnelID, "no known_hosts file — accepting host key on first use")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	verify, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("read known_hosts: %w", err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := verify(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) == 0 {
				r.log(tunnelID, "host %s not in known_hosts — accepting on first use", hostname)
				return nil
			}
			*mismatch = keyErr // a real mismatch: let dial() decide what to do
		}
		return err
	}, nil
}
