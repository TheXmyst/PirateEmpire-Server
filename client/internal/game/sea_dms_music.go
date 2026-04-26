package game

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

type DMSMode int

const (
	DMSCalm DMSMode = iota
	DMSCombat
)

type SeaDMSMusic struct {
	audioManager  *AudioManager
	players       [6]*audio.Player
	volumes       [6]float64 // current volumes
	targetVolumes [6]float64
	lastVolumes   [6]float64 // last applied volumes for optimization
	mode          DMSMode
	isPlaying     bool
	isStopping    bool
	isLoaded      bool
	logCounter    int
	soloIndex     int // -1 for no solo, 0-5 for solo stem
	isDebugSolo   bool
}

func NewSeaDMSMusic(am *AudioManager) *SeaDMSMusic {
	return &SeaDMSMusic{
		audioManager: am,
		mode:         DMSCalm,
		soloIndex:    -1,
	}
}

func (m *SeaDMSMusic) Load() error {
	if m.isLoaded {
		return nil
	}

	stemFiles := []string{
		"resources/Music/sea_DMS/0 Lead Vocals.mp3",
		"resources/Music/sea_DMS/1 Drums.mp3",
		"resources/Music/sea_DMS/2 Bass.mp3",
		"resources/Music/sea_DMS/3 Guitar.mp3",
		"resources/Music/sea_DMS/4 Percussion.mp3",
		"resources/Music/sea_DMS/5 Synth.mp3",
	}

	ctx := m.audioManager.Context()
	if ctx == nil {
		return fmt.Errorf("audio context is nil")
	}

	loadedCount := 0
	for i, path := range stemFiles {
		// Use assetsFS from game package
		file, err := assetsFS.Open(path)
		if err != nil {
			log.Printf("[DMS] Failed to open stem %d (%s): %v", i, path, err)
			continue
		}

		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			log.Printf("[DMS] Failed to read stem %d: %v", i, err)
			continue
		}

		// Decode
		stream, err := mp3.DecodeWithoutResampling(bytes.NewReader(data))
		if err != nil {
			log.Printf("[DMS] Failed to decode stem %d: %v", i, err)
			continue
		}

		// Loop
		loopStream := audio.NewInfiniteLoop(stream, stream.Length())

		// Player
		player, err := ctx.NewPlayer(loopStream)
		if err != nil {
			log.Printf("[DMS] Failed to create player for stem %d: %v", i, err)
			continue
		}

		m.players[i] = player
		// Initialize volume to 0
		m.players[i].SetVolume(0)
		loadedCount++
	}

	m.isLoaded = true
	log.Printf("[DMS] init loaded=%d path=resources/Music/sea_DMS", loadedCount)
	return nil
}

func (m *SeaDMSMusic) Start() {
	// 1. Strict Idempotency Guard
	if m.isPlaying && !m.isStopping {
		log.Printf("[DMS] start IGNORED (already running) inst=%p", m)
		return
	}

	if !m.isLoaded {
		if err := m.Load(); err != nil {
			log.Printf("[DMS] start FAILED (load error): %v", err)
			return
		}
	}

	m.isPlaying = true
	m.isStopping = false
	m.updateTargets()

	// 2. Sample-Sync Start
	// We rewind and play ALL stems in the same tick.
	for i, p := range m.players {
		if p != nil {
			p.Pause()
			p.Rewind()
			// Continuous Playback Strategy:
			// Start at volume 0.0, but PLAYING.
			// Silence is handled strictly by volume in Update().
			p.SetVolume(0)
			m.volumes[i] = 0
			m.lastVolumes[i] = -1 // Force SetVolume in first Update
			p.Play()
			log.Printf("[DMS] play_start stem=%d ptr=%p inst=%p", i, p, m)
		}
	}

	log.Printf("[DMS] start view=sea mode=%d inst=%p", m.mode, m)
}

func (m *SeaDMSMusic) Stop() {
	if !m.isPlaying && !m.isStopping {
		log.Printf("[DMS] stop IGNORING (not running) inst=%p", m)
		return
	}

	// Trigger fade out. The players will final-pause in Update() when volumes reach 0.
	m.isStopping = true
	m.isPlaying = false
	m.updateTargets()

	log.Printf("[DMS] stop fade_out_begin inst=%p", m)
}

func (m *SeaDMSMusic) SetMode(mode DMSMode, reason string) {
	if m.mode == mode {
		return
	}
	m.mode = mode
	m.updateTargets()

	modeStr := "calm"
	if m.mode == DMSCombat {
		modeStr = "combat"
	}

	log.Printf("[DMS] mode=%s reason=%s inst=%p", modeStr, reason, m)
}

func (m *SeaDMSMusic) updateTargets() {
	// Defaults 0
	for i := range m.targetVolumes {
		m.targetVolumes[i] = 0.0
	}

	if !m.isPlaying {
		return // All target 0 for fade out
	}

	if m.isDebugSolo && m.soloIndex >= 0 && m.soloIndex < 6 {
		m.targetVolumes[m.soloIndex] = 1.0
		return
	}

	// Calm: 0, 3, 4, 5 at 1.0.  1, 2 at 0.0
	// Combat: 0, 3, 4, 5 at 0.85. 1, 2 at 1.0
	switch m.mode {
	case DMSCalm:
		m.targetVolumes[0] = 1.0
		m.targetVolumes[3] = 1.0
		m.targetVolumes[4] = 1.0
		m.targetVolumes[5] = 1.0
		m.targetVolumes[1] = 0.0
		m.targetVolumes[2] = 0.0
	case DMSCombat:
		m.targetVolumes[0] = 0.85
		m.targetVolumes[3] = 0.85
		m.targetVolumes[4] = 0.85
		m.targetVolumes[5] = 0.85
		m.targetVolumes[1] = 1.0
		m.targetVolumes[2] = 1.0
	}
}

func (m *SeaDMSMusic) Update(dt float64) {
	// Handle Debug Input (if available) - F9/F10
	// Note: We need inpututil here, but it's already imported.
	// Actually, Update is called from Game, we can move input check here
	// if we import inpututil.

	// Guards for total silence and stop
	if !m.isPlaying && !m.isStopping {
		return
	}

	fadeActive := false
	allSilent := true

	for i := 0; i < 6; i++ {
		target := m.targetVolumes[i]
		current := m.volumes[i]

		// 1. Fade Logic (Linear 1.0s)
		if math.Abs(current-target) > 0.0001 {
			fadeActive = true
			diff := target - current
			step := 1.0 * dt // 1.0s to go from 0 to 1

			if math.Abs(diff) <= step {
				m.volumes[i] = target
			} else if diff > 0 {
				m.volumes[i] += step
			} else {
				m.volumes[i] -= step
			}
		}

		// 2. Volume-Based Playback (NO PAUSE while active)
		p := m.players[i]
		if p != nil {
			// Apply volume ONLY if it changed (Optimization + Root Cause Fix)
			// This ensures we aren't fighting other systems if any.
			if m.volumes[i] != m.lastVolumes[i] {
				p.SetVolume(m.volumes[i])
				m.lastVolumes[i] = m.volumes[i]
			}

			// Ensure it stays playing while in SeaView (Sync Recovery)
			if (m.isPlaying || m.isStopping) && !p.IsPlaying() {
				p.Play()
				log.Printf("[DMS] play_recovery stem=%d ptr=%p inst=%p", i, p, m)
			}
		}

		if m.volumes[i] > 0.001 {
			allSilent = false
		}
	}

	// 3. Final Cleanup at end of fade-out
	if m.isStopping && !fadeActive && allSilent {
		m.isStopping = false
		for i, p := range m.players {
			if p != nil {
				p.Pause()
				p.Rewind()
				log.Printf("[DMS] pause_final stem=%d ptr=%p inst=%p", i, p, m)
			}
		}
		log.Printf("[DMS] stop fade_out_complete inst=%p", m)
	}

	// 4. Trace mode changes in debug log
	m.logCounter++
	if m.logCounter >= 300 {
		m.logCounter = 0
		log.Printf("[DMS TRACE] inst=%p mode=%d isPlaying=%v isStopping=%v volumes=%.2f", m, m.mode, m.isPlaying, m.isStopping, m.volumes)
	}
}

// HandleDebugKeys should be called by Game.Update if possible, or we check here
func (m *SeaDMSMusic) HandleDebugKeys(isKeyJustPressed func(ebiten.Key) bool, F9, F10 ebiten.Key) {
	if isKeyJustPressed(F9) {
		m.isDebugSolo = true
		m.soloIndex = (m.soloIndex + 1) % 6
		m.updateTargets()
		log.Printf("[DMS DEBUG] Solo Stem %d", m.soloIndex)
	}
	if isKeyJustPressed(F10) {
		m.isDebugSolo = false
		m.soloIndex = -1
		m.updateTargets()
		log.Printf("[DMS DEBUG] Reset Mix")
	}
}
