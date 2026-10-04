package torx

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/victorbillyph/fvremote/internal/config"
)

// torVersion é a versão do bundle "expert" do Tor baixada pelo app.
const torVersion = "15.0.24"

func detect() (goos, arch string) {
	goos = runtime.GOOS
	arch = runtime.GOARCH
	switch arch {
	case "arm64":
		arch = "aarch64"
	case "amd64":
		arch = "x86_64"
	}
	return
}

func candidateURLs(goos, arch string) []string {
	base := "https://dist.torproject.org/torbrowser/" + torVersion
	file := func(os, a string) string {
		return fmt.Sprintf("%s/tor-expert-bundle-%s-%s-%s.tar.gz", base, os, a, torVersion)
	}
	switch goos {
	case "windows":
		return []string{file("windows", arch), file("windows", "x86_64")}
	case "linux":
		return []string{file("linux", arch), file("linux", "x86_64")}
	case "darwin":
		return []string{file("macos", arch), file("macos", "x86_64")}
	default:
		return nil
	}
}

// Ensure baixa e extrai o Tor standalone apenas se ainda não existir.
// O conteúdo fica exclusivamente em config.TorDir().
func Ensure(progress func(int64, int64)) error {
	if _, err := os.Stat(config.TorBin()); err == nil {
		_ = os.Chmod(config.TorBin(), 0o755)
		return nil
	}
	if err := config.EnsureBase(); err != nil {
		return err
	}
	if err := os.MkdirAll(config.TorDir(), 0o755); err != nil {
		return err
	}

	goos, arch := detect()
	urls := candidateURLs(goos, arch)
	if len(urls) == 0 {
		return fmt.Errorf("plataforma não suportada: %s/%s", goos, arch)
	}

	tmp := filepath.Join(os.TempDir(), "fvremote-tor.tar.gz")
	var lastErr error
	for _, u := range urls {
		if err := download(u, tmp, progress); err != nil {
			lastErr = err
			continue
		}
		if err := extractTarGz(tmp, config.TorDir()); err != nil {
			lastErr = err
			continue
		}
		_ = os.Remove(tmp)
		if _, err := os.Stat(config.TorBin()); err != nil {
			lastErr = fmt.Errorf("binário do tor não encontrado após extrair de %s", u)
			continue
		}
		_ = os.Chmod(config.TorBin(), 0o755)
		return nil
	}
	return fmt.Errorf("falha ao obter tor standalone: %v", lastErr)
}

func download(url, dst string, progress func(int64, int64)) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "fvremote/"+torVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	total := resp.ContentLength
	var written int64
	buf := make([]byte, 128*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if progress != nil {
				progress(written, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// extractTarGz extrai o bundle removendo o prefixo "tor/".
func extractTarGz(src, out string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			continue
		}
		name := strings.TrimPrefix(h.Name, "./")
		name = strings.TrimPrefix(name, "tor/")
		if name == "" || strings.HasPrefix(name, "..") {
			continue
		}
		if strings.HasPrefix(name, "pluggable_transports/") {
			continue
		}
		dst := filepath.Join(out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		w, err := os.Create(dst)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, tr); err != nil {
			w.Close()
			return err
		}
		w.Close()
		if strings.HasSuffix(name, "tor") || strings.HasSuffix(name, "tor.exe") {
			_ = os.Chmod(dst, 0o755)
		}
	}
	return nil
}
