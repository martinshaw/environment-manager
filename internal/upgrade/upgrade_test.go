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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		current, latest string
		want            int
	}{
		{"0.1.0", "0.1.0", 0},
		{"v0.1.0", "v0.1.0", 0},
		{"0.1.0", "0.1.1", -1},
		{"0.2.0", "0.1.9", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.1.0-rc.1", "0.1.0", -1},
		{"0.1.0", "0.1.0-rc.1", 1},
		{"dev", "0.1.0", -1},
	}
	for _, tc := range cases {
		got, err := compareVersions(tc.current, tc.latest)
		if err != nil {
			t.Fatalf("%s vs %s: %v", tc.current, tc.latest, err)
		}
		if got != tc.want {
			t.Fatalf("%s vs %s: got %d want %d", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestRunInstallsRelease(t *testing.T) {
	payload := []byte("new-envm-binary")
	name := assetMust(t, "0.2.0", runtime.GOOS, runtime.GOARCH)
	archive := archiveWith(t, name, binaryFile(runtime.GOOS), payload)
	var downloads int
	srv := releaseServer(t, "v0.2.0", name, archive, &downloads)
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), binaryFile(runtime.GOOS))
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(dest)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Current: "0.1.0",
		Binary:  dest,
		APIBase: srv.URL,
		HTTP:    srv.Client(),
		Stdout:  &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Updated || res.Latest != "0.2.0" || res.Path != resolved {
		t.Fatalf("result: %+v", res)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("binary = %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("mode = %v", info.Mode())
	}
	if !bytes.Contains(out.Bytes(), []byte("Updated "+resolved)) {
		t.Fatalf("output: %s", out.String())
	}
	if downloads != 2 {
		t.Fatalf("downloads = %d", downloads)
	}
}

func TestRunRejectsBadChecksum(t *testing.T) {
	name := assetMust(t, "0.2.0", runtime.GOOS, runtime.GOARCH)
	archive := archiveWith(t, name, binaryFile(runtime.GOOS), []byte("new"))
	sum := sha256.Sum256(archive)
	sum[0] ^= 0xff
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/martinshaw/environment-manager/releases/latest":
			writeRelease(w, "v0.2.0", name, "http://"+r.Host+"/archive", "http://"+r.Host+"/checksums.txt")
		case "/checksums.txt":
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"))
		case "/archive":
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "envm")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Options{
		Current: "0.1.0",
		Binary:  dest,
		APIBase: srv.URL,
		HTTP:    srv.Client(),
		Stdout:  ioDiscard(),
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("checksum")) {
		t.Fatalf("err = %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old" {
		t.Fatalf("binary changed to %q", got)
	}
}

func TestRunCheckDoesNotDownload(t *testing.T) {
	var downloads int
	name := assetMust(t, "0.2.0", runtime.GOOS, runtime.GOARCH)
	srv := releaseServer(t, "v0.2.0", name, []byte("unused"), &downloads)
	defer srv.Close()
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Current:   "0.1.0",
		CheckOnly: true,
		APIBase:   srv.URL,
		HTTP:      srv.Client(),
		Stdout:    &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated || downloads != 0 {
		t.Fatalf("updated=%v downloads=%d", res.Updated, downloads)
	}
	if !bytes.Contains(out.Bytes(), []byte("0.2.0 is available")) {
		t.Fatalf("output: %s", out.String())
	}
}

func TestRunUpToDate(t *testing.T) {
	name := assetMust(t, "0.1.0", runtime.GOOS, runtime.GOARCH)
	var downloads int
	srv := releaseServer(t, "v0.1.0", name, nil, &downloads)
	defer srv.Close()
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Current: "v0.1.0",
		APIBase: srv.URL,
		HTTP:    srv.Client(),
		Stdout:  &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated || !bytes.Contains(out.Bytes(), []byte("up to date")) {
		t.Fatalf("result=%+v out=%s", res, out.String())
	}
}

func TestRunRejectsUnsafeArchive(t *testing.T) {
	name := assetMust(t, "0.2.0", runtime.GOOS, runtime.GOARCH)
	archive := archiveWith(t, name, "../"+binaryFile(runtime.GOOS), []byte("bad"))
	var downloads int
	srv := releaseServer(t, "v0.2.0", name, archive, &downloads)
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "envm")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Options{
		Current: "0.1.0",
		Binary:  dest,
		APIBase: srv.URL,
		HTTP:    srv.Client(),
		Stdout:  ioDiscard(),
	})
	if err == nil {
		t.Fatal("expected unsafe archive to fail")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old" {
		t.Fatalf("binary changed to %q", got)
	}
}

func TestAssetName(t *testing.T) {
	name, err := assetName("0.1.0", "windows", "amd64")
	if err != nil || name != "envm_0.1.0_windows_amd64.zip" {
		t.Fatalf("name=%s err=%v", name, err)
	}
	if _, err := assetName("0.1.0", "darwin", "386"); err == nil {
		t.Fatal("expected unsupported arch")
	}
}

func assetMust(t *testing.T, version, goos, goarch string) string {
	t.Helper()
	name, err := assetName(version, goos, goarch)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func archiveWith(t *testing.T, archiveName, entry string, payload []byte) []byte {
	t.Helper()
	if strings.HasSuffix(archiveName, ".zip") {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, err := zw.Create(entry)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: entry, Mode: 0o755, Size: int64(len(payload))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractZip(t *testing.T) {
	archive := archiveWith(t, "envm_0.2.0_windows_amd64.zip", "envm.exe", []byte("exe-bytes"))
	got, err := extractBinary(archive, "envm_0.2.0_windows_amd64.zip", "envm.exe")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "exe-bytes" {
		t.Fatalf("got %q", got)
	}
}

func TestRejectsInsecureAssetURL(t *testing.T) {
	if err := validateAssetURL("https://api.github.com", "http://example.com/envm.tar.gz"); err == nil {
		t.Fatal("expected http asset to be rejected")
	}
}

func releaseServer(t *testing.T, tag, name string, archive []byte, downloads *int) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(archive)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing user agent on %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/repos/martinshaw/environment-manager/releases/latest":
			writeRelease(w, tag, name, "http://"+r.Host+"/archive", "http://"+r.Host+"/checksums.txt")
		case "/checksums.txt":
			*downloads++
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"))
		case "/archive":
			*downloads++
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
}

func writeRelease(w http.ResponseWriter, tag, name, archiveURL, sumURL string) {
	_ = json.NewEncoder(w).Encode(ghRelease{
		TagName: tag,
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			{Name: name, BrowserDownloadURL: archiveURL},
			{Name: "checksums.txt", BrowserDownloadURL: sumURL},
		},
	})
}

func ioDiscard() *bytes.Buffer { return &bytes.Buffer{} }
