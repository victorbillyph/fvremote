package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const (
	repoBase  = "https://github.com/victorbillyph/fvremote/releases"
	latestURL = repoBase + "/latest"
)

// Info descreve o resultado da checagem de atualização.
type Info struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Notes     string `json:"notes"`
	AssetURL  string `json:"-"`
	AssetName string `json:"-"`
}

func client(socksAddr string) *http.Client {
	if socksAddr == "" {
		return &http.Client{Timeout: 90 * time.Second}
	}
	d, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return &http.Client{Timeout: 90 * time.Second}
	}
	return &http.Client{
		Timeout:   90 * time.Second,
		Transport: &http.Transport{Dial: d.Dial},
	}
}

// Check descobre a última release via redirect de /releases/latest (sem API)
// e monta a URL do asset pelo padrão de nomes.
func Check(current, socksAddr string) (*Info, error) {
	req, _ := http.NewRequest(http.MethodGet, latestURL, nil)
	req.Header.Set("User-Agent", "fvremote/"+current)
	resp, err := client(socksAddr).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	final := ""
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.Path
	}
	info := &Info{Current: current}
	idx := strings.LastIndex(final, "/tag/")
	if idx < 0 {
		return info, nil // sem release ainda
	}
	tag := strings.TrimSpace(final[idx+len("/tag/"):])
	info.Latest = strings.TrimPrefix(tag, "v")
	if isNewer(info.Latest, current) {
		info.Available = true
		info.AssetName = assetName()
		info.AssetURL = fmt.Sprintf("%s/download/%s/%s", repoBase, tag, info.AssetName)
	}
	return info, nil
}

func assetName() string {
	if runtime.GOOS == "windows" {
		return "fvremote-windows-amd64.zip"
	}
	return "fvremote-linux-amd64.tar.gz"
}

// Apply baixa o asset da release, extrai o binário e substitui o executável atual.
func Apply(info *Info, socksAddr string) error {
	if info == nil || info.AssetURL == "" {
		return fmt.Errorf("nenhum asset de atualização disponível")
	}
	req, _ := http.NewRequest(http.MethodGet, info.AssetURL, nil)
	req.Header.Set("User-Agent", "fvremote/"+info.Current)
	resp, err := client(socksAddr).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download respondeu %s", resp.Status)
	}

	tmpDir, err := os.MkdirTemp("", "fvremote-update")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archive := filepath.Join(tmpDir, info.AssetName)
	if err := writeFile(archive, resp.Body); err != nil {
		return err
	}

	var binPath string
	if strings.HasSuffix(info.AssetName, ".zip") {
		binPath, err = extractZip(archive, tmpDir)
	} else {
		binPath, err = extractTarGz(archive, tmpDir)
	}
	if err != nil {
		return err
	}
	return Replace(binPath)
}

func writeFile(path string, r io.Reader) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func wantBin(name string) bool {
	base := filepath.Base(name)
	return base == "fvremote" || base == "fvremote.exe"
}

func extractTarGz(src, dir string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if h.Typeflag != tar.TypeReg || !wantBin(h.Name) {
			continue
		}
		out := filepath.Join(dir, "fvremote.new")
		w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(w, tr); err != nil {
			w.Close()
			return "", err
		}
		w.Close()
		return out, nil
	}
	return "", fmt.Errorf("binário não encontrado no tar")
}

func extractZip(src, dir string) (string, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() || !wantBin(zf.Name) {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return "", err
		}
		out := filepath.Join(dir, "fvremote.new.exe")
		w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, err = io.Copy(w, rc)
		w.Close()
		rc.Close()
		if err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("binário não encontrado no zip")
}

// Replace coloca o novo binário no lugar do executável atual.
func Replace(newBin string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	_ = os.Chmod(newBin, 0o755)
	return replaceExecutable(newBin, exe)
}

// isNewer compara versões semver simples (x.y.z).
func isNewer(a, b string) bool {
	pa, pb := parseVer(a), parseVer(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parseVer(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(strings.TrimSpace(parts[i]))
		out[i] = n
	}
	return out
}
