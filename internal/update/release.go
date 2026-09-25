// Package update checks public GitHub releases and stages verified app updates.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const Repository = "https://github.com/fireph/ForeverDubbed"
const latestURL = "https://api.github.com/repos/fireph/ForeverDubbed/releases/latest"
const maxDownload = int64(2 << 30)

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}
type Release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
	Archive    Asset   `json:"-"`
	Checksums  Asset   `json:"-"`
}
type Progress struct {
	Stage       string
	Done, Total int64
}
type Reporter func(Progress)

func report(f Reporter, stage string, done, total int64) {
	if f != nil {
		f(Progress{stage, done, total})
	}
}

type Client struct {
	HTTP      *http.Client
	LatestURL string
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("update redirect must use HTTPS")
		}
		return nil
	}}, LatestURL: latestURL}
}

// Version parses stable release versions only. Prereleases must never replace a
// stable install, and integer comparison avoids treating 0.10 as older than 0.9.
func Version(s string) ([3]uint64, error) {
	var result [3]uint64
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return result, fmt.Errorf("invalid release version %q", s)
	}
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return result, fmt.Errorf("invalid release version %q", s)
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return result, fmt.Errorf("invalid release version %q", s)
			}
		}
		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return result, err
		}
		result[i] = v
	}
	return result, nil
}
func newer(candidate, current string) bool {
	a, err := Version(candidate)
	if err != nil {
		return false
	}
	b, err := Version(current)
	if err != nil {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
func (c *Client) get(ctx context.Context, address string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ForeverDubbed-Updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GitHub returned HTTP %d", resp.StatusCode)
	}
	return resp, nil
}
func (c *Client) Check(ctx context.Context, current, goos, arch string) (*Release, error) {
	if _, err := Version(current); err != nil {
		return nil, err
	}
	resp, err := c.get(ctx, c.LatestURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return nil, err
	}
	if r.Draft || r.Prerelease || !newer(r.Tag, current) {
		return nil, nil
	}
	var names []string
	switch goos {
	case "windows":
		names = []string{"ForeverDubbed-windows-" + arch + "-portable.zip"}
	case "darwin":
		names = []string{"ForeverDubbed-mac-" + arch + ".zip"}
	default:
		return nil, nil
	}
	for _, name := range names {
		for _, a := range r.Assets {
			if a.Name == name {
				r.Archive = a
				break
			}
		}
		if r.Archive.Name != "" {
			break
		}
	}
	for _, a := range r.Assets {
		if a.Name == "SHA256SUMS.txt" {
			r.Checksums = a
		}
	}
	if r.Archive.Name == "" || r.Checksums.Name == "" {
		return nil, fmt.Errorf("release %s has no complete update package for %s/%s", r.Tag, goos, arch)
	}
	for _, a := range []Asset{r.Archive, r.Checksums} {
		if a.Size <= 0 || a.Size > maxDownload {
			return nil, fmt.Errorf("invalid release asset size")
		}
		u, err := url.Parse(a.URL)
		if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/fireph/ForeverDubbed/releases/download/"+r.Tag+"/"+a.Name {
			return nil, fmt.Errorf("unexpected release download URL")
		}
	}
	return &r, nil
}
func checksum(data []byte, name string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	found := ""
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 2 || strings.TrimPrefix(parts[1], "*") != name {
			continue
		}
		raw, err := hex.DecodeString(parts[0])
		if err != nil || len(raw) != sha256.Size || found != "" {
			return "", fmt.Errorf("invalid or duplicate checksum for %s", name)
		}
		found = strings.ToLower(parts[0])
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("missing checksum for %s", name)
	}
	return found, nil
}
func (c *Client) Download(ctx context.Context, r *Release, destination string, progress Reporter) error {
	report(progress, "Downloading checksums", 0, 0)
	resp, err := c.get(ctx, r.Checksums.URL)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	resp.Body.Close()
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return fmt.Errorf("checksum file too large")
	}
	expected, err := checksum(data, r.Archive.Name)
	if err != nil {
		return err
	}
	resp, err = c.get(ctx, r.Archive.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.ContentLength >= 0 && resp.ContentLength != r.Archive.Size {
		return fmt.Errorf("release download size changed")
	}
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(destination)
		}
	}()
	hash := sha256.New()
	w := io.MultiWriter(f, hash)
	reader := io.LimitReader(resp.Body, r.Archive.Size+1)
	buf := make([]byte, 128<<10)
	var count int64
	last := time.Time{}
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if _, err = w.Write(buf[:n]); err != nil {
				return err
			}
			count += int64(n)
		}
		if time.Since(last) > 100*time.Millisecond {
			report(progress, "Downloading update", count, r.Archive.Size)
			last = time.Now()
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if count != r.Archive.Size {
		return fmt.Errorf("incomplete update download")
	}
	report(progress, "Verifying download", count, count)
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return fmt.Errorf("update checksum does not match; nothing was installed")
	}
	if err = f.Close(); err != nil {
		return err
	}
	success = true
	return nil
}
