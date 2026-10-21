package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.12.1", "0.12.0", true},
		{"0.13.0", "0.12.9", true},
		{"1.0.0", "0.99.99", true},
		{"0.12.0", "0.12.0", false},
		{"0.11.5", "0.12.0", false},
		{"v0.12.1", "0.12.0", true},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%s, %s) = %v", c.a, c.b, got)
		}
	}
}

// fakeGitHub serves a release whose build is exe and whose published
// checksum is sum.
func fakeGitHub(t *testing.T, exe []byte, sum string) Client {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/me/td/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.12.1","assets":[{"name":"twitchtd.exe","url":"%[1]s/releases/assets/1"},{"name":"twitchtd.exe.sha256","url":"%[1]s/releases/assets/2"}]}`, srv.URL)
		case "/releases/assets/1":
			w.Write(exe)
		case "/releases/assets/2":
			fmt.Fprintln(w, sum)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return Client{Repo: "me/td", API: srv.URL}
}

func TestDownloadChecksTheBuild(t *testing.T) {
	exe := []byte("MZ new build")
	h := sha256.Sum256(exe)
	c := fakeGitHub(t, exe, hex.EncodeToString(h[:]))
	r, err := c.Latest(context.Background())
	if err != nil || r.Version != "0.12.1" {
		t.Fatalf("latest release: %+v %v", r, err)
	}
	path := filepath.Join(t.TempDir(), "twitchtd.next.exe")
	if err := c.Download(context.Background(), r, path); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != string(exe) {
		t.Fatal("the build is saved as published")
	}

	bad := fakeGitHub(t, exe, "deadbeef")
	r, _ = bad.Latest(context.Background())
	path2 := filepath.Join(t.TempDir(), "twitchtd.next.exe")
	if err := bad.Download(context.Background(), r, path2); err == nil {
		t.Fatal("a checksum mismatch is refused")
	}
	if _, err := os.Stat(path2); err == nil {
		t.Fatal("and leaves nothing behind")
	}
}
