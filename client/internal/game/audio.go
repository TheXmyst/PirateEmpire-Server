package game

import (
	"bytes"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

const sampleRate = 48000

type AudioManager struct {
	audioContext *audio.Context
	// mu protège players/currentTrack/wantedTrack : les musiques peuvent
	// finir de se charger en arrière-plan pendant que le jeu tourne.
	mu           sync.Mutex
	players      map[string]*audio.Player
	currentTrack string
	// wantedTrack est la musique demandée par le jeu, même si elle n'est pas
	// encore chargée : elle démarre dès qu'elle arrive.
	wantedTrack string
	// waitingLogged évite de répéter le message "pas encore chargée" à
	// chaque image.
	waitingLogged string
}

func NewAudioManager() *AudioManager {
	ctx := audio.NewContext(sampleRate)
	return &AudioManager{
		audioContext: ctx,
		players:      make(map[string]*audio.Player),
		currentTrack: "",
	}
}

// LoadMusicAsync charge une musique en arrière-plan, sans bloquer le jeu.
// Dans le navigateur, elle est téléchargée depuis le serveur.
func (am *AudioManager) LoadMusicAsync(name, path string) {
	go func() {
		if err := am.LoadMusic(name, path); err != nil {
			log.Printf("[AUDIO] Failed to load %s: %v", name, err)
		}
	}()
}

// LoadMusic loads an MP3 file and stores it as a player
func (am *AudioManager) LoadMusic(name, path string) error {
	data, err := readAsset(path)
	if err != nil {
		log.Printf("Failed to read audio file %s: %v", path, err)
		return err
	}

	// Decode MP3
	stream, err := mp3.DecodeWithoutResampling(bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to decode MP3 %s: %v", path, err)
		return err
	}

	// Create looping stream
	loopStream := audio.NewInfiniteLoop(stream, stream.Length())

	// Create player
	player, err := am.audioContext.NewPlayer(loopStream)
	if err != nil {
		log.Printf("Failed to create player for %s: %v", path, err)
		return err
	}

	// Set volume to 100%
	player.SetVolume(1.0)

	am.mu.Lock()
	am.players[name] = player
	startNow := am.wantedTrack == name && am.currentTrack != name
	am.mu.Unlock()
	log.Printf("[AUDIO] Loaded music: %s from %s (volume: 1.0)", name, path)

	// Le jeu avait déjà demandé cette musique : on la lance maintenant.
	if startNow {
		am.PlayMusic(name)
	}
	return nil
}

// PlayMusic plays the specified track (stops current if different)
func (am *AudioManager) PlayMusic(name string) {
	if audioMuted() {
		return
	}
	am.mu.Lock()
	defer am.mu.Unlock()
	am.wantedTrack = name

	if am.currentTrack == name {
		// Already playing this track, ensure it's playing
		if player, ok := am.players[name]; ok && !player.IsPlaying() {
			player.Play()
		}
		return
	}

	// Stop current track
	if am.currentTrack != "" {
		if player, ok := am.players[am.currentTrack]; ok {
			player.Pause()
			player.Rewind()
		}
	}

	// Play new track
	if player, ok := am.players[name]; ok {
		player.Rewind()
		player.SetVolume(1.0) // Ensure volume is set
		player.Play()
		am.currentTrack = name
		log.Printf("[AUDIO] Now playing: %s (IsPlaying: %v)", name, player.IsPlaying())
	} else {
		// Pas encore chargée (téléchargement en cours) : elle démarrera
		// toute seule à la fin du chargement grâce à wantedTrack.
		am.currentTrack = ""
		if am.waitingLogged != name {
			am.waitingLogged = name
			log.Printf("[AUDIO] Track %s not loaded yet, will start when ready (loaded: %v)", name, am.getTrackNames())
		}
	}
}

// Helper to get track names for debugging
func (am *AudioManager) getTrackNames() []string {
	names := make([]string, 0, len(am.players))
	for name := range am.players {
		names = append(names, name)
	}
	return names
}

// Wants dit si cette musique joue ou attend d'être chargée pour jouer.
func (am *AudioManager) Wants(name string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.currentTrack == name || am.wantedTrack == name
}

// StopAll stops all music
func (am *AudioManager) StopAll() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.wantedTrack = ""
	for _, player := range am.players {
		player.Pause()
		player.Rewind()
	}
	am.currentTrack = ""
}

// Context returns the underlying audio context
func (am *AudioManager) Context() *audio.Context {
	return am.audioContext
}
