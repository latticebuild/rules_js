package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/latticebuild/graceproc"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type output struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.Write(p)
}
func (o *output) text() string { o.mu.Lock(); defer o.mu.Unlock(); return o.buffer.String() }

func TestStorybookServer(t *testing.T) {
	path, err := runfiles.Rlocation(os.Getenv("SERVER"))
	if err != nil {
		t.Fatal(err)
	}
	address := "127.0.0.1:46137"
	socket, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	var logs output
	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(), "BOUND_CACHE=0")
	cmd.Stdout, cmd.Stderr = &logs, &logs
	signals := make(chan os.Signal, 1)
	exited := make(chan error, 1)
	go func() { _, err := graceproc.Run(cmd, signals); exited <- err }()
	t.Cleanup(func() {
		signals <- os.Interrupt
		if err := <-exited; err != nil {
			t.Errorf("server cleanup: %v\n%s", err, logs.text())
		}
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Errorf("server retained port: %v", err)
		} else if err := listener.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := &http.Client{Timeout: time.Second}
	var last error
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/index.json", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if err == nil {
			var index struct {
				Entries map[string]struct {
					Title string `json:"title"`
				} `json:"entries"`
			}
			err = json.NewDecoder(response.Body).Decode(&index)
			closeErr := response.Body.Close()
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			if response.StatusCode == http.StatusOK && err == nil {
				entry, ok := index.Entries["fixture-button--ready"]
				if !ok || entry.Title != "Fixture/Button" {
					t.Fatalf("story index = %#v", index)
				}
				inventory, err := runfiles.Rlocation(os.Getenv("BROWSER_INVENTORY"))
				if err != nil {
					t.Fatal(err)
				}
				contents, err := os.ReadFile(inventory)
				if err != nil {
					t.Fatal(err)
				}
				var files struct {
					Marker string `json:"marker"`
				}
				if err := json.Unmarshal(contents, &files); err != nil {
					t.Fatal(err)
				}
				marker, err := runfiles.Rlocation(files.Marker)
				if err != nil {
					t.Fatal(err)
				}
				probe, err := runfiles.Rlocation(os.Getenv("BROWSER_PROBE"))
				if err != nil {
					t.Fatal(err)
				}
				scratch, err := os.MkdirTemp("", "storybook-browser-")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(scratch); err != nil {
						t.Errorf("browser scratch cleanup: %v", err)
					}
				})
				// Give rendering its own budget after the cold server has become ready.
				renderCtx, renderCancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer renderCancel()
				browser := exec.Command(probe, "http://"+address)
				browser.Env = append(os.Environ(), "BOUND_CACHE=0", "DEBUG=pw:browser,pw:api", "HOME="+scratch, "USERPROFILE="+scratch, "TMPDIR="+scratch, "TMP="+scratch, "TEMP="+scratch, "PLAYWRIGHT_BROWSERS_PATH="+filepath.Dir(filepath.Dir(marker)))
				var rendered output
				browser.Stdout, browser.Stderr = &rendered, &rendered
				probeSignals := make(chan os.Signal, 1)
				probeDone := make(chan struct{})
				go func() {
					select {
					case <-renderCtx.Done():
						probeSignals <- os.Interrupt
					case <-probeDone:
					}
				}()
				status, err := graceproc.Run(browser, probeSignals)
				close(probeDone)
				if err != nil || status != 0 || renderCtx.Err() != nil {
					t.Fatalf("rendered story: status %d, error %v, deadline %v\n%s\n%s", status, err, renderCtx.Err(), rendered.text(), logs.text())
				}
				return
			}
			last = fmt.Errorf("status %d, index decode: %v", response.StatusCode, err)
		} else {
			last = err
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			t.Fatalf("server never became ready: %v\n%s", last, logs.text())
		case <-timer.C:
		}
	}
}
