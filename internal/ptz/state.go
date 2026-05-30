// Package ptz implements a digital PTZ state machine for virtual pan/tilt/zoom.
// It manages continuous movement, absolute/relative positioning, and preset storage.
// Position ranges: Pan [-1,1] (left to right), Tilt [-1,1] (down to up), Zoom [0,1] (full frame to max zoom).
package ptz

import (
	"context"
	"math"
	"sync"
	"time"
)

// Position represents current PTZ coordinates.
type Position struct {
	Pan  float64 // -1.0 to 1.0 (left to right)
	Tilt float64 // -1.0 to 1.0 (down to up)
	Zoom float64 // 0.0 to 1.0 (full frame to max zoom)
}

// Velocity represents PTZ movement speed.
type Velocity struct {
	Pan  float64 // -1.0 to 1.0
	Tilt float64 // -1.0 to 1.0
	Zoom float64 // 0.0 to 1.0
}

// Preset stores a named PTZ position.
type Preset struct {
	Name     string
	Position Position
}

// State manages digital PTZ state with background movement.
type State struct {
	mu       sync.RWMutex
	position Position
	velocity Velocity
	presets  map[string]Preset
	moving   bool
	cancel   context.CancelFunc
}

// NewState creates a new PTZ state initialized at center position.
func NewState() *State {
	return &State{
		position: Position{Pan: 0, Tilt: 0, Zoom: 0},
		presets:  make(map[string]Preset),
	}
}

// clampf restricts v to [lo, hi].
func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clampPosition restricts all axes to valid ranges.
func clampPosition(p Position) Position {
	return Position{
		Pan:  clampf(p.Pan, -1, 1),
		Tilt: clampf(p.Tilt, -1, 1),
		Zoom: clampf(p.Zoom, 0, 1),
	}
}

// ContinuousMove starts velocity-based movement.
// The position is updated at 50ms tick intervals using: pos += velocity * 0.01
// Any previous movement is stopped before starting the new one.
func (s *State) ContinuousMove(vel Velocity) {
	s.stopInternal()

	s.mu.Lock()
	s.velocity = vel
	s.moving = true
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	go s.runMovement(ctx)
}

// runMovement updates position at 50ms intervals until context is cancelled.
func (s *State) runMovement(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			s.position.Pan = clampf(s.position.Pan+s.velocity.Pan*0.01, -1, 1)
			s.position.Tilt = clampf(s.position.Tilt+s.velocity.Tilt*0.01, -1, 1)
			s.position.Zoom = clampf(s.position.Zoom+s.velocity.Zoom*0.01, 0, 1)

			// Stop if we've hit a boundary and velocity would push further
			stopped := (s.velocity.Pan != 0 && s.position.Pan == clampf(s.velocity.Pan, -1, 1)) ||
				(s.velocity.Tilt != 0 && s.position.Tilt == clampf(s.velocity.Tilt, -1, 1)) ||
				(s.velocity.Zoom != 0 && s.position.Zoom == clampf(s.velocity.Zoom, 0, 1))
			// Actually let it continue — ContinuousMove stays active until Stop() is called,
			// even if at boundary. The position just won't change past the boundary.
			_ = stopped
			s.mu.Unlock()
		}
	}
}

// AbsoluteMove moves to exact position with exponential easing.
// Any previous movement is stopped.
func (s *State) AbsoluteMove(pos Position) {
	s.stopInternal()

	target := clampPosition(pos)

	s.mu.Lock()
	start := s.position
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.moving = true
	s.mu.Unlock()

	go s.runAbsoluteMove(ctx, start, target)
}

// runAbsoluteMove animates from start to target with exponential easing.
func (s *State) runAbsoluteMove(ctx context.Context, start, target Position) {
	const stepDuration = 50 * time.Millisecond
	const totalSteps = 20 // ~1 second total
	const easeFactor = 0.15

	ticker := time.NewTicker(stepDuration)
	defer ticker.Stop()

	for i := 0; i < totalSteps; i++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			s.position.Pan += (target.Pan - s.position.Pan) * easeFactor
			s.position.Tilt += (target.Tilt - s.position.Tilt) * easeFactor
			s.position.Zoom += (target.Zoom - s.position.Zoom) * easeFactor

			// Snap to target when close enough
			if math.Abs(s.position.Pan-target.Pan) < 0.001 {
				s.position.Pan = target.Pan
			}
			if math.Abs(s.position.Tilt-target.Tilt) < 0.001 {
				s.position.Tilt = target.Tilt
			}
			if math.Abs(s.position.Zoom-target.Zoom) < 0.001 {
				s.position.Zoom = target.Zoom
			}

			done := s.position == target
			s.mu.Unlock()

			if done {
				s.mu.Lock()
				s.moving = false
				s.velocity = Velocity{}
				s.mu.Unlock()
				return
			}
		}
	}

	// Ensure we reach exact target after all steps
	s.mu.Lock()
	s.position = target
	s.moving = false
	s.velocity = Velocity{}
	s.mu.Unlock()
}

// RelativeMove applies relative movement from current position.
// The delta is applied immediately (not animated).
func (s *State) RelativeMove(delta Velocity) {
	s.mu.Lock()
	s.position.Pan = clampf(s.position.Pan+delta.Pan, -1, 1)
	s.position.Tilt = clampf(s.position.Tilt+delta.Tilt, -1, 1)
	s.position.Zoom = clampf(s.position.Zoom+delta.Zoom, 0, 1)
	s.mu.Unlock()
}

// Stop halts all movement immediately.
func (s *State) Stop() {
	s.stopInternal()
}

// stopInternal stops movement without lock contention (caller handles locking).
func (s *State) stopInternal() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.velocity = Velocity{}
	s.moving = false
	s.mu.Unlock()
}

// GetPosition returns current position (thread-safe).
func (s *State) GetPosition() Position {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.position
}

// GetStatus returns movement status: "IDLE" or "MOVING".
func (s *State) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.moving {
		return "MOVING"
	}
	return "IDLE"
}

// SetPreset stores current position as a named preset.
func (s *State) SetPreset(token string, presetName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presets[token] = Preset{
		Name:     presetName,
		Position: s.position,
	}
	return nil
}

// GetPresets returns list of preset tokens.
func (s *State) GetPresets() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tokens := make([]string, 0, len(s.presets))
	for token := range s.presets {
		tokens = append(tokens, token)
	}
	return tokens
}

// GotoPreset moves to stored preset position using AbsoluteMove.
func (s *State) GotoPreset(token string) error {
	s.mu.RLock()
	preset, ok := s.presets[token]
	s.mu.RUnlock()

	if !ok {
		return ErrPresetNotFound
	}

	s.AbsoluteMove(preset.Position)
	return nil
}

// RemovePreset deletes a named preset.
func (s *State) RemovePreset(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.presets[token]; !ok {
		return ErrPresetNotFound
	}

	delete(s.presets, token)
	return nil
}

// GetPresetPosition returns the position for a preset token, or error if not found.
func (s *State) GetPresetPosition(token string) (Position, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	preset, ok := s.presets[token]
	if !ok {
		return Position{}, ErrPresetNotFound
	}
	return preset.Position, nil
}
