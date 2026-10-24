package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	owner = "martinshaw"
	repo  = "environment-manager"

	defaultAPI = "https://api.github.com"
	maxArchive = 64 << 20
	maxBinary  = 64 << 20
)

// Options controls a check or install of the latest GitHub release.
type Options struct {
	Current   string
	GOOS      string
	GOARCH    string
	Binary    string
	CheckOnly bool
	APIBase   string
	HTTP      *http.Client
	Stdout    io.Writer
}

// Result is what Run found and whether it replaced the binary.
type Result struct {
	Current string
	Latest  string
	Updated bool
	Path    string
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Run compares the current version with the latest GitHub release.
// Unless CheckOnly is set, a newer release replaces the current binary
// after its archive matches checksums.txt.
func Run(ctx context.Context, opt Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if opt.GOOS == "" {
		opt.GOOS = runtime.GOOS
	}
	if opt.GOARCH == "" {
		opt.GOARCH = runtime.GOARCH
	}
	if opt.APIBase == "" {
		opt.APIBase = defaultAPI
	}
	if opt.Stdout == nil {
		opt.Stdout = os.Stdout
	}
	client := opt.HTTP
	if client == nil {
		client = &http.Client{CheckRedirect: refuseInsecureRedirect}
	}

	rel, err := fetchLatest(ctx, client, opt.APIBase)
	if err != nil {
		return Result{}, err
	}
	current := displayVersion(opt.Current)
	latest := displayVersion(rel.TagName)
	res := Result{Current: current, Latest: latest}
	cmp, err := compareVersions(current, latest)
	if err != nil {
		return res, err
	}
	switch {
	case cmp > 0:
		fmt.Fprintf(opt.Stdout, "envm %s is newer than the latest GitHub release %s\n", current, latest)
		return res, nil
	case cmp == 0:
		fmt.Fprintf(opt.Stdout, "envm %s is up to date\n", current)
		return res, nil
	case opt.CheckOnly:
		fmt.Fprintf(opt.Stdout, "envm %s is available (installed %s)\n", latest, current)
		return res, nil
	}

	name, err := assetName(latest, opt.GOOS, opt.GOARCH)
	if err != nil {
		return res, err
	}
	archiveURL, err := findAsset(rel, name)
	if err != nil {
		return res, err
	}
	sumURL, err := findAsset(rel, "checksums.txt")
	if err != nil {
		return res, err
	}
	if err := validateAssetURL(opt.APIBase, archiveURL); err != nil {
		return res, err
	}
	if err := validateAssetURL(opt.APIBase, sumURL); err != nil {
		return res, err
	}

	dest, err := binaryPath(opt.Binary)
	if err != nil {
		return res, err
	}
	if err := canReplace(dest); err != nil {
		return res, err
	}

	fmt.Fprintf(opt.Stdout, "Downloading %s\n", name)
	archive, err := getBytes(ctx, client, archiveURL, maxArchive)
	if err != nil {
		return res, err
	}
	sums, err := getBytes(ctx, client, sumURL, 1<<20)
	if err != nil {
		return res, err
	}
	want, err := checksumFor(string(sums), name)
	if err != nil {
		return res, err
	}
	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != want {
		return res, errors.New("archive checksum does not match checksums.txt")
	}
	payload, err := extractBinary(archive, name, binaryFile(opt.GOOS))
	if err != nil {
		return res, err
	}
	if err := installBinary(dest, payload); err != nil {
		return res, err
	}
	res.Updated = true
	res.Path = dest
	fmt.Fprintf(opt.Stdout, "Updated %s from %s to %s\n", dest, current, latest)
	return res, nil
}

func fetchLatest(ctx context.Context, client *http.Client, apiBase string) (ghRelease, error) {
	url := strings.TrimRight(apiBase, "/") + "/repos/" + owner + "/" + repo + "/releases/latest"
	body, err := getBytes(ctx, client, url, 1<<20)
	if err != nil {
		return ghRelease{}, err
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return ghRelease{}, fmt.Errorf("read GitHub release: %w", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return ghRelease{}, errors.New("GitHub release has no tag")
	}
	return rel, nil
}

func findAsset(rel ghRelease, name string) (string, error) {
	for _, asset := range rel.Assets {
		if asset.Name == name {
			if asset.BrowserDownloadURL == "" {
				return "", fmt.Errorf("release asset %s has no download URL", name)
			}
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("release has no %s", name)
}

func validateAssetURL(apiBase, raw string) error {
	if raw == "" {
		return errors.New("empty asset URL")
	}
	if strings.HasPrefix(strings.TrimRight(apiBase, "/"), "https://") && !strings.HasPrefix(raw, "https://") {
		return fmt.Errorf("release asset is not https: %s", raw)
	}
	return nil
}

func getBytes(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "envm")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		text := strings.TrimSpace(string(msg))
		if text == "" {
			return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
		}
		return nil, fmt.Errorf("GET %s: %s: %s", url, resp.Status, text)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response from %s is larger than %d bytes", url, limit)
	}
	return body, nil
}

func refuseInsecureRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("too many redirects")
	}
	if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect from https to %s", req.URL.Scheme)
	}
	return nil
}

func assetName(version, goos, goarch string) (string, error) {
	switch goos {
	case "darwin", "linux", "windows":
	default:
		return "", fmt.Errorf("no release binary for %s/%s", goos, goarch)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("no release binary for %s/%s", goos, goarch)
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("envm_%s_%s_%s%s", version, goos, goarch, ext), nil
}

func binaryFile(goos string) string {
	if goos == "windows" {
		return "envm.exe"
	}
	return "envm"
}

func binaryPath(path string) (string, error) {
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		path = exe
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		if isTempBuild(path) {
			return "", errors.New("envm is running from a temporary build. Install a release binary, then run envm upgrade")
		}
		return path, nil
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}
	return path, nil
}

func isTempBuild(path string) bool {
	clean := filepath.Clean(path)
	tmp := filepath.Clean(os.TempDir())
	if tmp != "" && (clean == tmp || strings.HasPrefix(clean, tmp+string(os.PathSeparator))) {
		return true
	}
	return strings.Contains(clean, string(os.PathSeparator)+"go-build")
}

func canReplace(dest string) error {
	f, err := os.CreateTemp(filepath.Dir(dest), ".envm-upgrade-*")
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("cannot write %s\nRe-run as a user who can write that file, for example: sudo envm upgrade", dest)
		}
		return fmt.Errorf("cannot write %s: %w", dest, err)
	}
	name := f.Name()
	f.Close()
	if err := os.Remove(name); err != nil {
		return err
	}
	return nil
}

func installBinary(dest string, data []byte) error {
	if err := canReplace(dest); err != nil {
		return err
	}
	mode := os.FileMode(0o755)
	if info, err := os.Stat(dest); err == nil {
		mode = info.Mode().Perm()
		if mode&0o111 == 0 {
			mode |= 0o755
		}
	}
	f, err := os.CreateTemp(filepath.Dir(dest), ".envm-new-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := replaceFile(tmp, dest); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func replaceFile(src, dest string) error {
	if runtime.GOOS == "windows" {
		old := dest + ".old"
		_ = os.Remove(old)
		if _, err := os.Stat(dest); err == nil {
			if err := os.Rename(dest, old); err != nil {
				return fmt.Errorf("cannot replace %s: %w", dest, err)
			}
		}
		if err := os.Rename(src, dest); err != nil {
			_ = os.Rename(old, dest)
			return fmt.Errorf("cannot replace %s: %w", dest, err)
		}
		_ = os.Remove(old)
		return nil
	}
	if err := os.Rename(src, dest); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("cannot replace %s: %w\nRe-run as a user who can write that file, for example: sudo envm upgrade", dest, err)
		}
		return fmt.Errorf("cannot replace %s: %w", dest, err)
	}
	return nil
}

func checksumFor(text, name string) (string, error) {
	var found string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		file := strings.TrimPrefix(fields[len(fields)-1], "*")
		if file != name && path.Base(file) != name {
			continue
		}
		sum := strings.ToLower(fields[0])
		if found != "" && found != sum {
			return "", fmt.Errorf("checksums.txt lists more than one hash for %s", name)
		}
		found = sum
	}
	if found == "" {
		return "", fmt.Errorf("checksums.txt has no entry for %s", name)
	}
	if len(found) != sha256.Size*2 || !isHex(found) {
		return "", fmt.Errorf("checksum for %s is not a sha256 hex string", name)
	}
	return found, nil
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

func extractBinary(archive []byte, archiveName, want string) ([]byte, error) {
	switch {
	case strings.HasSuffix(archiveName, ".tar.gz"):
		return extractTar(archive, want)
	case strings.HasSuffix(archiveName, ".zip"):
		return extractZip(archive, want)
	default:
		return nil, fmt.Errorf("unknown archive type %s", archiveName)
	}
}

func extractTar(archive []byte, want string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var found []byte
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if !safeArchivePath(hdr.Name) {
			return nil, fmt.Errorf("archive path %q is not allowed", hdr.Name)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		if path.Base(filepath.ToSlash(hdr.Name)) != want {
			continue
		}
		if hdr.Size < 0 || hdr.Size > maxBinary {
			return nil, fmt.Errorf("archive entry %s is larger than %d bytes", hdr.Name, maxBinary)
		}
		if found != nil {
			return nil, errors.New("archive contains more than one envm binary")
		}
		body, err := io.ReadAll(io.LimitReader(tr, maxBinary+1))
		if err != nil {
			return nil, err
		}
		if int64(len(body)) > maxBinary {
			return nil, fmt.Errorf("archive entry %s is larger than %d bytes", hdr.Name, maxBinary)
		}
		found = body
	}
	if found == nil {
		return nil, fmt.Errorf("archive does not contain %s", want)
	}
	return found, nil
}

func extractZip(archive []byte, want string) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	var found []byte
	for _, f := range r.File {
		if !safeArchivePath(f.Name) {
			return nil, fmt.Errorf("archive path %q is not allowed", f.Name)
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if path.Base(filepath.ToSlash(f.Name)) != want {
			continue
		}
		if f.UncompressedSize64 > maxBinary {
			return nil, fmt.Errorf("archive entry %s is larger than %d bytes", f.Name, maxBinary)
		}
		if found != nil {
			return nil, errors.New("archive contains more than one envm binary")
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(rc, maxBinary+1))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if uint64(len(body)) > maxBinary {
			return nil, fmt.Errorf("archive entry %s is larger than %d bytes", f.Name, maxBinary)
		}
		found = body
	}
	if found == nil {
		return nil, fmt.Errorf("archive does not contain %s", want)
	}
	return found, nil
}

func safeArchivePath(name string) bool {
	name = filepath.ToSlash(strings.TrimSpace(name))
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return false
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return false
	}
	return true
}

type version struct {
	major, minor, patch int
	pre                 bool
}

func compareVersions(current, latest string) (int, error) {
	l, ok := parseVersion(latest)
	if !ok {
		return 0, fmt.Errorf("cannot read release version %q", latest)
	}
	c, ok := parseVersion(current)
	if !ok {
		return -1, nil
	}
	if c.major != l.major {
		return cmpInt(c.major, l.major), nil
	}
	if c.minor != l.minor {
		return cmpInt(c.minor, l.minor), nil
	}
	if c.patch != l.patch {
		return cmpInt(c.patch, l.patch), nil
	}
	if c.pre != l.pre {
		if c.pre {
			return -1, nil
		}
		return 1, nil
	}
	return 0, nil
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func parseVersion(s string) (version, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return version{}, false
	}
	pre := false
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		if i == 0 {
			return version{}, false
		}
		pre = true
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return version{}, false
	}
	nums := [3]int{}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || (len(part) > 1 && part[0] == '0') {
			return version{}, false
		}
		nums[i] = n
	}
	return version{major: nums[0], minor: nums[1], patch: nums[2], pre: pre}, true
}

func displayVersion(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimPrefix(s, "v")
}
