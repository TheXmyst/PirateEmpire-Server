package game

import (
	"bytes"
	"io"
	"log"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

const sampleRate = 48000

type AudioManager struct {
	audioContext *audio.Context
	players      map[string]*audio.Player
	currentTrack string
}

func NewAudioManager() *AudioManager {
	ctx := audio.NewContext(sampleRate)
	return &AudioManager{
		audioContext: ctx,
		players:      make(map[string]*audio.Player),
		currentTrack: "",
	}
}

// LoadMusic loads an MP3 file and stores it as a player
func (am *AudioManager) LoadMusic(name, path string) error {
	// Use assetsFS instead of os.Open for embedded files
	file, err := assetsFS.Open(path)
	if err != nil {
		log.Printf("Failed to open audio file %s: %v", path, err)
		return err
	}
	defer file.Close()

	// Read the entire file
	data, err := io.ReadAll(file)
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

	am.players[name] = player
	log.Printf("[AUDIO] Loaded music: %s from %s (volume: 1.0)", name, path)
	return nil
}

// PlayMusic plays the specified track (stops current if different)
func (am *AudioManager) PlayMusic(name string) {
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
		log.Printf("[AUDIO ERROR] Music track not found in players map: %s", name)
		log.Printf("[AUDIO DEBUG] Available tracks: %v", am.getTrackNames())
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

// StopAll stops all music
func (am *AudioManager) StopAll() {
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
