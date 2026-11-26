// Package webclient sert la version navigateur du jeu sur /play.
//
// Le dossier servi est produit par client/web/build.sh (deploy/update.sh le
// fait tout seul). Comme la page vient du même site que l'API, le navigateur
// n'a pas besoin de règles CORS.
package webclient

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

// DefaultDir est le dossier utilisé si WEB_DIR n'est pas défini : "web",
// à côté du serveur (en production : /opt/seadogs/web).
const DefaultDir = "web"

// Dir renvoie le dossier de la version web (variable WEB_DIR ou défaut).
func Dir() string {
	if v := os.Getenv("WEB_DIR"); v != "" {
		return v
	}
	return DefaultDir
}

// Installed dit si la version web a été compilée dans dir.
func Installed(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil
}

// Register ajoute les routes /play et /play/* au serveur.
func Register(e *echo.Echo, dir string) {
	e.GET("/play", func(c echo.Context) error {
		// La page charge ses fichiers en relatif : il faut le "/" final.
		return c.Redirect(http.StatusMovedPermanently, "/play/")
	})
	e.GET("/play/*", func(c echo.Context) error {
		return serve(c, dir, c.Param("*"))
	})
	e.HEAD("/play/*", func(c echo.Context) error {
		return serve(c, dir, c.Param("*"))
	})
}

func serve(c echo.Context, dir, rel string) error {
	if !Installed(dir) {
		return c.String(http.StatusNotFound, "La version navigateur n'est pas encore installée sur ce serveur.")
	}

	// path.Clean("/"+...) empêche de sortir du dossier avec "../".
	clean := strings.TrimPrefix(path.Clean("/"+rel), "/")
	if clean == "" {
		clean = "index.html"
	}
	full := filepath.Join(dir, filepath.FromSlash(clean))

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return echo.ErrNotFound
	}

	w := c.Response()
	req := c.Request()
	switch {
	case clean == "index.html" || clean == "build-info.json":
		// Toujours redemander : c'est ce qui annonce une nouvelle version.
		w.Header().Set("Cache-Control", "no-cache")
	case clean == "game.wasm":
		// L'adresse contient la version (?v=...), donc on peut garder le
		// fichier longtemps : une nouvelle version aura une autre adresse.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Vary", "Accept-Encoding")
		if acceptsGzip(req) {
			if gz, err := os.Stat(full + ".gz"); err == nil && !gz.IsDir() {
				return serveFile(c, full+".gz", "application/wasm", "gzip")
			}
		}
		return serveFile(c, full, "application/wasm", "")
	default:
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	return serveFile(c, full, "", "")
}

func serveFile(c echo.Context, file, contentType, encoding string) error {
	f, err := os.Open(file)
	if err != nil {
		return echo.ErrNotFound
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", file, err)
	}
	w := c.Response()
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if encoding != "" {
		w.Header().Set("Content-Encoding", encoding)
	}
	// ServeContent gère le type de fichier, les requêtes partielles et le
	// cache du navigateur (If-Modified-Since).
	http.ServeContent(w, c.Request(), filepath.Base(file), info.ModTime(), f)
	return nil
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(strings.SplitN(part, ";", 2)[0]) == "gzip" {
			return true
		}
	}
	return false
}
