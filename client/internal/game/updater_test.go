//go:build !js

package game

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdaterCheckAndDownload(t *testing.T) {
	exe := []byte("NOUVEAU JEU")
	sum := sha256.Sum256(exe)
	sha := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version":
			fmt.Fprintf(w, `{"version":"v2","url":"http://%s/download/SeaDogs.exe","sha256":"%s"}`, r.Host, sha)
		case "/download/SeaDogs.exe":
			w.Write(exe)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	oldEnabled, oldVersion := updatesEnabled, ClientVersion
	defer func() { updatesEnabled, ClientVersion = oldEnabled, oldVersion }()
	updatesEnabled = func() bool { return true }

	ClientVersion = "v2"
	if avail, _, err := CheckForUpdates(srv.URL); err != nil || avail {
		t.Fatalf("même version: avail=%v err=%v", avail, err)
	}

	ClientVersion = "v1"
	avail, info, err := CheckForUpdates(srv.URL)
	if err != nil || !avail || info.Version != "v2" {
		t.Fatalf("nouvelle version attendue: avail=%v info=%+v err=%v", avail, info, err)
	}

	dst := filepath.Join(t.TempDir(), "new.exe")
	var last float64
	if err := downloadVerified(info, dst, func(p float64) { last = p }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != string(exe) || last != 1 {
		t.Fatalf("contenu %q, progression %v", b, last)
	}

	// Empreinte fausse : refusé et fichier effacé.
	info.SHA256 = sha[:63] + "0"
	if sha[63] == '0' {
		info.SHA256 = sha[:63] + "1"
	}
	if err := downloadVerified(info, dst, nil); err == nil {
		t.Fatal("empreinte fausse acceptée")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("le fichier abîmé devrait être effacé")
	}
}
