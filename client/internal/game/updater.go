//go:build !js

package game

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/TheXmyst/Sea-Dogs/client/internal/client"
)

// Mises à jour automatiques du client PC (Windows).
//
// Au lancement, le jeu demande /version au serveur. Si la version publiée
// est différente de la sienne, la fenêtre « Mise à jour disponible »
// s'affiche. Le clic télécharge le nouveau .exe à côté de l'ancien, vérifie
// son empreinte SHA-256, prend la place de l'ancien et relance le jeu.

// UpdateInfo : ce que le serveur annonce.
type UpdateInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
}

var updateHTTP = &http.Client{Timeout: 10 * time.Second}

// updatesEnabled : seulement les .exe Windows publiés (pas un build de dev).
var updatesEnabled = func() bool {
	return runtime.GOOS == "windows" && ClientVersion != "dev" && ClientVersion != ""
}

// CheckForUpdates demande au serveur s'il y a une nouvelle version.
func CheckForUpdates(apiURL string) (bool, UpdateInfo, error) {
	var res UpdateInfo
	if !updatesEnabled() {
		return false, res, nil
	}
	versionURL := strings.TrimRight(apiURL, "/") + "/version"
	if err := client.CheckSecureURL(versionURL); err != nil {
		return false, res, fmt.Errorf("mise à jour refusée: %w", err)
	}
	resp, err := updateHTTP.Get(versionURL)
	if err != nil {
		return false, res, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, res, nil // le serveur ne publie pas (encore) de client PC
	}
	if resp.StatusCode != http.StatusOK {
		return false, res, fmt.Errorf("status %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return false, res, err
	}
	if res.Version == "" || res.URL == "" || len(res.SHA256) != 64 {
		return false, res, fmt.Errorf("réponse /version incomplète")
	}
	return res.Version != ClientVersion, res, nil
}

// PerformUpdate télécharge la nouvelle version, la vérifie, remplace le jeu
// et le relance. progress reçoit l'avancement (0 à 1) si la taille est connue.
func PerformUpdate(info UpdateInfo, progress func(float64)) error {
	// On télécharge un programme qui sera ensuite lancé : HTTPS obligatoire
	// (sauf serveur local), et l'empreinte doit correspondre.
	if err := client.CheckSecureURL(info.URL); err != nil {
		return fmt.Errorf("mise à jour refusée: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	newExe := filepath.Join(dir, "SeaDogs_update.exe.tmp")
	oldExe := exe + ".old"

	if err := downloadVerified(info, newExe, progress); err != nil {
		return err
	}

	// Windows interdit de supprimer un programme en cours, mais permet de le
	// renommer : l'ancien devient .old (effacé au prochain lancement).
	os.Remove(oldExe)
	if err := os.Rename(exe, oldExe); err != nil {
		os.Remove(newExe)
		return err
	}
	if err := os.Rename(newExe, exe); err != nil {
		os.Rename(oldExe, exe) // on remet l'ancien
		os.Remove(newExe)
		return err
	}

	cmd := exec.Command(exe)
	cmd.Dir, _ = os.Getwd()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mise à jour installée, relance le jeu à la main (%v)", err)
	}
	os.Exit(0)
	return nil
}

// downloadVerified télécharge info.URL dans dst et vérifie son SHA-256.
// En cas d'échec, dst est effacé.
func downloadVerified(info UpdateInfo, dst string, progress func(float64)) error {
	dl := &http.Client{Timeout: 15 * time.Minute}
	resp, err := dl.Get(info.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("téléchargement échoué: %s", resp.Status)
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("impossible d'écrire %s: %w", dst, err)
	}
	h := sha256.New()
	var r io.Reader = resp.Body
	if resp.ContentLength > 0 && progress != nil {
		r = &progressReader{r: resp.Body, total: resp.ContentLength, cb: progress}
	}
	_, err = io.Copy(io.MultiWriter(out, h), r)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dst)
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, info.SHA256) {
		os.Remove(dst)
		return fmt.Errorf("fichier téléchargé abîmé (empreinte différente), réessaie plus tard")
	}
	return nil
}

// CleanupOldUpdate efface l'ancien .exe laissé par la dernière mise à jour.
func CleanupOldUpdate() {
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		os.Remove(exe + ".old")
	}
}

type progressReader struct {
	r     io.Reader
	total int64
	done  int64
	cb    func(float64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	p.cb(float64(p.done) / float64(p.total))
	return n, err
}
