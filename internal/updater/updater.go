// Package updater finds the latest GitHub release of the game and downloads
// its Windows build, checking it against the published SHA-256.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Asset names the release workflow publishes (.github/workflows/release.yml).
const (
	ExeAsset = "twitchtd.exe"
	SumAsset = "twitchtd.exe.sha256"
)

// Release is the latest published release.
type Release struct {
	Version string // without the leading v
	ExeURL  string
	SumURL  string
}

// Client talks to GitHub for one repository ("owner/name"). Token is only
// needed for a private repository.
type Client struct {
	Repo  string
	Token string
	API   string // https://api.github.com unless a test points it elsewhere
	HTTP  *http.Client
}

func (c Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "twitchtd-updater")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if strings.Contains(url, "/releases/assets/") {
		req.Header.Set("Accept", "application/octet-stream")
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 2 * time.Minute}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return resp, nil
}

// Latest returns the newest release that carries the Windows build.
func (c Client) Latest(ctx context.Context) (Release, error) {
	api := c.API
	if api == "" {
		api = "https://api.github.com"
	}
	resp, err := c.get(ctx, api+"/repos/"+c.Repo+"/releases/latest")
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	var body struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"url"` // the API URL works for private repositories too
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, err
	}
	r := Release{Version: strings.TrimPrefix(body.Tag, "v")}
	for _, a := range body.Assets {
		switch a.Name {
		case ExeAsset:
			r.ExeURL = a.URL
		case SumAsset:
			r.SumURL = a.URL
		}
	}
	if r.ExeURL == "" || r.SumURL == "" {
		return Release{}, fmt.Errorf("release %s has no %s with its checksum", body.Tag, ExeAsset)
	}
	return r, nil
}

// Download saves the release's build to path, but only if its SHA-256
// matches the published one; a bad download leaves nothing behind.
func (c Client) Download(ctx context.Context, r Release, path string) error {
	resp, err := c.get(ctx, r.SumURL)
	if err != nil {
		return err
	}
	sum, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	if err != nil {
		return err
	}
	want := strings.ToLower(strings.Fields(string(sum) + " ")[0])

	resp, err = c.get(ctx, r.ExeURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != want {
		err = fmt.Errorf("checksum mismatch")
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// Newer reports whether version a (X.Y.Z) is later than b.
func Newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parts(v string) [3]int {
	var out [3]int
	for i, s := range strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3) {
		out[i], _ = strconv.Atoi(s)
	}
	return out
}
