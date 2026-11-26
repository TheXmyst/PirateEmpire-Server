//go:build js

package game

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"syscall/js"
)

// platform_js.go : la version navigateur (WebAssembly).
//
// Pour que le jeu se charge vite, on n'embarque ici que les images que le jeu
// affiche vraiment. La musique (la plus lourde) et toute ressource absente de
// cette liste sont téléchargées depuis le serveur à la demande, dans le
// dossier /play/resources/... : rien ne manque, c'est juste un peu plus lent.
// Si tu ajoutes une image utilisée au démarrage, ajoute-la aussi ici.

//go:embed resources/shaders resources/data resources/ui resources/sea
//go:embed resources/images/ui_9slice
//go:embed web_resources
//go:embed resources/images/login_bg.jpg resources/images/logo.png
//go:embed resources/images/icon_wood.png resources/images/icon_stone.png resources/images/icon_rum.png
//go:embed resources/images/icon_gold.png resources/images/icon_blueprint_btn.png resources/images/icon_hammer_1.png
//go:embed resources/assets/UI/icon_build_btn.png
//go:embed resources/assets/townhall resources/assets/sawmill resources/assets/gold_mine
//go:embed resources/assets/stone_quarry resources/assets/warehouse resources/assets/distillery
//go:embed resources/assets/shipyard resources/assets/academy resources/assets/Tavern resources/assets/militia
var assetsFS embed.FS

// IsWebBuild vaut true seulement dans la version navigateur.
const IsWebBuild = true

// storagePrefix évite de mélanger nos données avec celles d'un autre site.
const storagePrefix = "seadogs/"

// pageBaseURL renvoie l'adresse du dossier de la page, ex.
// "https://seadogs.duckdns.org/play/".
func pageBaseURL() string {
	loc := js.Global().Get("location")
	base := loc.Get("origin").String() + loc.Get("pathname").String()
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[:i+1]
	}
	return base
}

// readAsset lit une ressource : d'abord dans le jeu, sinon sur le serveur.
func readAsset(path string) ([]byte, error) {
	// web_resources/ contient des versions allégées pour le navigateur (ex.
	// l'île en 2500x1750 au lieu de 5000x3500, 4x moins lourde pour les
	// téléphones). Elles remplacent l'original du même nom.
	if light, ok := strings.CutPrefix(path, "resources/"); ok {
		if data, err := fs.ReadFile(assetsFS, "web_resources/"+light); err == nil {
			return data, nil
		}
	}
	if data, err := fs.ReadFile(assetsFS, path); err == nil {
		return data, nil
	}
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	resp, err := http.Get(pageBaseURL() + strings.Join(parts, "/"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("téléchargement de %s : %s", path, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func localStorage() js.Value {
	return js.Global().Get("localStorage")
}

// readUserFile lit une donnée du joueur gardée par le navigateur (localStorage).
func readUserFile(name string) (data []byte, err error) {
	defer func() {
		// localStorage peut être bloqué (navigation privée, cookies refusés).
		if r := recover(); r != nil {
			data, err = nil, fmt.Errorf("stockage du navigateur indisponible: %v", r)
		}
	}()
	v := localStorage().Call("getItem", storagePrefix+name)
	if v.IsNull() || v.IsUndefined() {
		return nil, fs.ErrNotExist
	}
	return []byte(v.String()), nil
}

// writeUserFile garde une donnée du joueur dans le navigateur.
func writeUserFile(name string, data []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("stockage du navigateur indisponible: %v", r)
		}
	}()
	localStorage().Call("setItem", storagePrefix+name, string(data))
	return nil
}

// loadPlatformConfig : dans le navigateur, le serveur de jeu est le site qui
// a envoyé la page (https://seadogs.duckdns.org). Pas de config.json.
func loadPlatformConfig(cfg *Config) {
	cfg.APIURL = js.Global().Get("location").Get("origin").String()
}
