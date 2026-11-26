package webclient

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
)

func setup(t *testing.T) (*echo.Echo, string) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.html":              "<html>jeu</html>",
		"game.wasm":               "WASM",
		"game.wasm.gz":            "GZ",
		"resources/Music/a b.mp3": "MP3",
		"../secret.txt":           "SECRET",
	}
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := echo.New()
	Register(e, dir)
	return e, dir
}

func get(e *echo.Echo, target string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestPlayServesIndexAndFiles(t *testing.T) {
	e, _ := setup(t)

	if rec := get(e, "/play", nil); rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/play/" {
		t.Fatalf("/play: code %d, location %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get(e, "/play/", nil); rec.Code != http.StatusOK || rec.Body.String() != "<html>jeu</html>" {
		t.Fatalf("/play/: code %d body %q", rec.Code, rec.Body.String())
	}
	if rec := get(e, "/play/resources/Music/a%20b.mp3", nil); rec.Code != http.StatusOK || rec.Body.String() != "MP3" {
		t.Fatalf("mp3: code %d body %q", rec.Code, rec.Body.String())
	}
	if rec := get(e, "/play/resources/Music/", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("listing de dossier: code %d, attendu 404", rec.Code)
	}
}

func TestPlayWasmGzip(t *testing.T) {
	e, _ := setup(t)

	rec := get(e, "/play/game.wasm?v=1", map[string]string{"Accept-Encoding": "gzip, br"})
	if rec.Body.String() != "GZ" || rec.Header().Get("Content-Encoding") != "gzip" || rec.Header().Get("Content-Type") != "application/wasm" {
		t.Fatalf("gzip: body %q, headers %v", rec.Body.String(), rec.Header())
	}
	rec = get(e, "/play/game.wasm", nil)
	if rec.Body.String() != "WASM" || rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Content-Type") != "application/wasm" {
		t.Fatalf("sans gzip: body %q, headers %v", rec.Body.String(), rec.Header())
	}
}

func TestPlayCannotEscapeDir(t *testing.T) {
	e, _ := setup(t)
	for _, target := range []string{"/play/../secret.txt", "/play/%2e%2e/secret.txt", "/play/..%2fsecret.txt"} {
		rec := get(e, target, nil)
		if rec.Body.String() == "SECRET" {
			t.Fatalf("%s a permis de sortir du dossier", target)
		}
	}
}

func TestPlayNotInstalled(t *testing.T) {
	e := echo.New()
	Register(e, filepath.Join(t.TempDir(), "absent"))
	if rec := get(e, "/play/", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("code %d, attendu 404", rec.Code)
	}
}
