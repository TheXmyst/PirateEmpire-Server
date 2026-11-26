package webclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/labstack/echo/v4"
)

// Client PC (Windows) téléchargeable et mises à jour automatiques.
//
// deploy/update.sh compile le jeu pour Windows et dépose dans DOWNLOAD_DIR
// (défaut "download", en production /opt/seadogs/download) :
//   - SeaDogs.exe ;
//   - version.json : {"version": "...", "file": "SeaDogs.exe", "sha256": "..."}.
//
// Routes :
//   - GET /version            -> ce que le client PC compare à sa version ;
//   - GET /download/SeaDogs.exe -> le jeu (aussi un lien à donner aux joueurs).

// DefaultDownloadDir : dossier utilisé si DOWNLOAD_DIR n'est pas défini.
const DefaultDownloadDir = "download"

// DownloadDir renvoie le dossier des téléchargements.
func DownloadDir() string {
	if v := os.Getenv("DOWNLOAD_DIR"); v != "" {
		return v
	}
	return DefaultDownloadDir
}

// ReleaseInfo : contenu de version.json.
type ReleaseInfo struct {
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
}

var safeFileName = regexp.MustCompile(`^[A-Za-z0-9._-]+\.exe$`)
var hexSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ReadRelease lit version.json et vérifie qu'il est cohérent.
func ReadRelease(dir string) (ReleaseInfo, error) {
	var r ReleaseInfo
	b, err := os.ReadFile(filepath.Join(dir, "version.json"))
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	if r.Version == "" || !safeFileName.MatchString(r.File) || !hexSHA256.MatchString(r.SHA256) {
		return r, fmt.Errorf("version.json incomplet ou invalide")
	}
	if _, err := os.Stat(filepath.Join(dir, r.File)); err != nil {
		return r, err
	}
	return r, nil
}

// RegisterDownloads ajoute /version et /download/<fichier>.
func RegisterDownloads(e *echo.Echo, dir string) {
	e.GET("/version", func(c echo.Context) error {
		r, err := ReadRelease(dir)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "aucune version PC publiée"})
		}
		c.Response().Header().Set("Cache-Control", "no-cache")
		// c.Scheme() lit X-Forwarded-Proto : derrière Caddy, c'est bien https.
		url := fmt.Sprintf("%s://%s/download/%s", c.Scheme(), c.Request().Host, r.File)
		return c.JSON(http.StatusOK, map[string]string{
			"version": r.Version,
			"url":     url,
			"sha256":  r.SHA256,
		})
	})
	e.GET("/download/:file", func(c echo.Context) error {
		r, err := ReadRelease(dir)
		if err != nil || c.Param("file") != r.File {
			return echo.ErrNotFound
		}
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.Attachment(filepath.Join(dir, r.File), r.File)
	})
}
