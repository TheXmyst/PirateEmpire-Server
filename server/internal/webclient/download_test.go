package webclient

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestDownloads(t *testing.T) {
	dir := t.TempDir()
	e := echo.New()
	RegisterDownloads(e, dir)

	// Rien de publié : 404.
	if rec := get(e, "/version", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("/version sans fichier: %d", rec.Code)
	}

	sha := strings.Repeat("ab", 32)
	os.WriteFile(filepath.Join(dir, "SeaDogs.exe"), []byte("EXE"), 0o644)
	os.WriteFile(filepath.Join(dir, "version.json"), []byte(`{"version":"abc123","file":"SeaDogs.exe","sha256":"`+sha+`"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "autre.exe"), []byte("NON"), 0o644)

	rec := get(e, "/version", map[string]string{"X-Forwarded-Proto": "https"})
	if rec.Code != http.StatusOK {
		t.Fatalf("/version: %d", rec.Code)
	}
	var v map[string]string
	json.Unmarshal(rec.Body.Bytes(), &v)
	if v["version"] != "abc123" || v["sha256"] != sha || v["url"] != "https://example.com/download/SeaDogs.exe" {
		t.Fatalf("réponse inattendue: %v", v)
	}

	if rec := get(e, "/download/SeaDogs.exe", nil); rec.Code != http.StatusOK || rec.Body.String() != "EXE" {
		t.Fatalf("téléchargement: %d %q", rec.Code, rec.Body.String())
	}
	for _, bad := range []string{"/download/autre.exe", "/download/version.json", "/download/..%2fsecret"} {
		if rec := get(e, bad, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%s devrait être refusé: %d", bad, rec.Code)
		}
	}
}
