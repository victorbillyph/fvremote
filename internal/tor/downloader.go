package tor

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

// torVersion fixa a versão do Tor baixada. O bundle "expert" é publicado para
// todas as plataformas e contém o binário tor (e geoip), sem navegador.
const torVersion = "13.5.7"

func detect() (goos, arch string) {
	goos = runtime.GOOS
	arch = runtime.GOARCH
	// normaliza nomes usados nos arquivos do Tor Project
	if arch == "arm64" {
		arch = "aarch64"
	}
	return
}

// candidateURLs retorna URLs possíveis para o bundle do Tor na plataforma.
func candidateURLs(goos, arch string) []string {
	base := "https://dist.torproject.org/torbrowser/" + torVersion
	switch goos {
	case "windows":
		return []string{
			fmt.Sprintf("%s/tor-expert-bundle-windows-x86_64-%s.tar.gz", base, torVersion),
			fmt.Sprintf("%s/tor-expert-bundle-windows-%s-%s.tar.gz", base, arch, torVersion),
		}
	case "linux":
		return []string{
			fmt.Sprintf("%s/tor-expert-bundle-linux-%s-%s.tar.gz", base, arch, torVersion),
			fmt.Sprintf("%s/tor-expert-bundle-linux-%s-%s.tar.gz", base, runtime.GOARCH, torVersion),
		}
	case "darwin":
		return []string{
			fmt.Sprintf("%s/tor-expert-bundle-macos-%s-%s.tar.gz", base, arch, torVersion),
			fmt.Sprintf("%s/tor-expert-bundle-macos-x86_64-%s.tar.gz", base, torVersion),
		}
	default:
		return nil
	}
}

// Ensure baixa e extrai o Tor standalone apenas se ainda não existir.
// O binário fica exclusivamente em config.TorDir().
func Ensure() error {
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
		if err := download(u, tmp); err != nil {
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

func download(url, dst string) error {
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
	_, err = io.Copy(f, resp.Body)
	return err
}

// extractTarGz extrai o conteúdo do bundle, removendo o prefixo "tor/".
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
		// ignora transportes plugáveis pesados que não são necessários
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
		_ = os.Chmod(dst, 0o755)
	}
	return nil
}
