package cairn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	fact "github.com/benaskins/axon-fact"
)

// EventTyper is implemented by all Cairn domain event structs.
type EventTyper interface {
	EventType() string
}

func newEvent(stream string, data EventTyper) (fact.Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return fact.Event{}, err
	}
	b := make([]byte, 16)
	rand.Read(b)
	return fact.Event{
		ID:     hex.EncodeToString(b),
		Stream: stream,
		Type:   data.EventType(),
		Data:   raw,
	}, nil
}

// emitEvent appends a domain event to the event store. No-op if es is nil.
func emitEvent(ctx context.Context, es fact.EventStore, stream string, data EventTyper) error {
	if es == nil {
		return nil
	}
	evt, err := newEvent(stream, data)
	if err != nil {
		return err
	}
	return es.Append(ctx, stream, []fact.Event{evt})
}

// Character events

type CharacterCreated struct {
	Character Sheet `json:"character"`
}

func (e CharacterCreated) EventType() string { return "character.created" }

type DamageTaken struct {
	Amount   int `json:"amount"`
	OldHP    int `json:"old_hp"`
	NewHP    int `json:"new_hp"`
	Overflow int `json:"overflow"`
}

func (e DamageTaken) EventType() string { return "character.damage_taken" }

type STRLost struct {
	Amount int    `json:"amount"`
	Source string `json:"source"`
}

func (e STRLost) EventType() string { return "character.str_lost" }

type Healed struct {
	Amount int `json:"amount"`
	OldHP  int `json:"old_hp"`
	NewHP  int `json:"new_hp"`
}

func (e Healed) EventType() string { return "character.healed" }

type ScarReceived struct {
	Scar Scar `json:"scar"`
}

func (e ScarReceived) EventType() string { return "character.scar_received" }

type ItemAdded struct {
	Item  string `json:"item"`
	Bulky bool   `json:"bulky"`
}

func (e ItemAdded) EventType() string { return "character.item_added" }

type ItemDropped struct {
	Item string `json:"item"`
}

func (e ItemDropped) EventType() string { return "character.item_dropped" }

type FatigueAdded struct {
	Source string `json:"source"`
}

func (e FatigueAdded) EventType() string { return "character.fatigue_added" }

type FatigueCleared struct{}

func (e FatigueCleared) EventType() string { return "character.fatigue_cleared" }

type CharacterDied struct {
	Cause string `json:"cause"`
}

func (e CharacterDied) EventType() string { return "character.died" }

// Encounter events

type EncounterStarted struct {
	EncounterID  string   `json:"encounter_id"`
	Participants []string `json:"participants"`
}

func (e EncounterStarted) EventType() string { return "encounter.started" }

type AttackResolved struct {
	AttackerID string `json:"attacker_id"`
	TargetID   string `json:"target_id"`
	Die        int    `json:"die"`
	Roll       int    `json:"roll"`
	Armor      int    `json:"armor"`
	Damage     int    `json:"damage"`
}

func (e AttackResolved) EventType() string { return "encounter.attack_resolved" }

type SaveMade struct {
	CharacterID string `json:"character_id"`
	Ability     string `json:"ability"`
	Roll        int    `json:"roll"`
	Score       int    `json:"score"`
	Passed      bool   `json:"passed"`
}

func (e SaveMade) EventType() string { return "encounter.save_made" }

type MoraleChecked struct {
	CreatureType string `json:"creature_type"`
	WIL          int    `json:"wil"`
	Roll         int    `json:"roll"`
	Fled         bool   `json:"fled"`
}

func (e MoraleChecked) EventType() string { return "encounter.morale_checked" }

type EncounterEnded struct {
	Outcome string `json:"outcome"` // "victory", "retreat", "rout"
}

func (e EncounterEnded) EventType() string { return "encounter.ended" }
