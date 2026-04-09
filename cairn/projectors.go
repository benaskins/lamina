package cairn

import (
	"context"
	"encoding/json"
	"fmt"

	fact "github.com/benaskins/axon-fact"
)

// characterReadWriter can both read and write characters. The projector requires this.
type characterReadWriter interface {
	CharacterStore
	CharacterWriter
}

// CharacterProjector updates the character read model from events.
type CharacterProjector struct {
	rw characterReadWriter
}

// NewCharacterProjector creates a projector that writes to the given store.
// The store must implement both CharacterStore (read) and CharacterWriter (write).
func NewCharacterProjector(rw characterReadWriter) *CharacterProjector {
	return &CharacterProjector{rw: rw}
}

// Handle applies an event to the character read model.
func (p *CharacterProjector) Handle(ctx context.Context, evt fact.Event) error {
	switch evt.Type {
	case "character.created":
		var data CharacterCreated
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return fmt.Errorf("unmarshal %s: %w", evt.Type, err)
		}
		c := data.Character
		return p.rw.SaveCharacter(ctx, &c)

	case "character.damage_taken":
		var data DamageTaken
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return fmt.Errorf("unmarshal %s: %w", evt.Type, err)
		}
		return p.updateCharacter(ctx, evt.Stream, func(c *Sheet) {
			c.CurrentHP = data.NewHP
		})

	case "character.str_lost":
		var data STRLost
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return fmt.Errorf("unmarshal %s: %w", evt.Type, err)
		}
		return p.updateCharacter(ctx, evt.Stream, func(c *Sheet) {
			c.STR -= data.Amount
			if c.STR <= 0 {
				c.STR = 0
				c.Dead = true
			}
		})

	case "character.healed":
		var data Healed
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return fmt.Errorf("unmarshal %s: %w", evt.Type, err)
		}
		return p.updateCharacter(ctx, evt.Stream, func(c *Sheet) {
			c.CurrentHP = data.NewHP
		})

	case "character.scar_received":
		var data ScarReceived
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return fmt.Errorf("unmarshal %s: %w", evt.Type, err)
		}
		return p.updateCharacter(ctx, evt.Stream, func(c *Sheet) {
			c.Scars = append(c.Scars, data.Scar)
		})

	case "character.item_added":
		var data ItemAdded
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return fmt.Errorf("unmarshal %s: %w", evt.Type, err)
		}
		return p.updateCharacter(ctx, evt.Stream, func(c *Sheet) {
			c.Inventory = append(c.Inventory, Slot{Item: data.Item, Bulky: data.Bulky})
			if c.SlotsUsed() >= 10 {
				c.CurrentHP = 0
			}
		})

	case "character.item_dropped":
		var data ItemDropped
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return fmt.Errorf("unmarshal %s: %w", evt.Type, err)
		}
		return p.updateCharacter(ctx, evt.Stream, func(c *Sheet) {
			for i, slot := range c.Inventory {
				if slot.Item == data.Item && !slot.Fatigue {
					c.Inventory = append(c.Inventory[:i], c.Inventory[i+1:]...)
					break
				}
			}
		})

	case "character.fatigue_added":
		return p.updateCharacter(ctx, evt.Stream, func(c *Sheet) {
			c.Inventory = append(c.Inventory, Slot{Item: "Fatigue", Fatigue: true})
		})

	case "character.fatigue_cleared":
		return p.updateCharacter(ctx, evt.Stream, func(c *Sheet) {
			filtered := c.Inventory[:0]
			for _, slot := range c.Inventory {
				if !slot.Fatigue {
					filtered = append(filtered, slot)
				}
			}
			c.Inventory = filtered
		})

	case "character.died":
		return p.updateCharacter(ctx, evt.Stream, func(c *Sheet) {
			c.Dead = true
		})
	}

	return nil
}

// characterIDFromStream extracts the character ID from a stream name like "character:id".
func characterIDFromStream(stream string) string {
	for i, ch := range stream {
		if ch == ':' {
			return stream[i+1:]
		}
	}
	return stream
}

func (p *CharacterProjector) updateCharacter(ctx context.Context, stream string, fn func(*Sheet)) error {
	id := characterIDFromStream(stream)
	c, err := p.rw.GetCharacter(ctx, id)
	if err != nil {
		return fmt.Errorf("load character %s: %w", id, err)
	}
	fn(c)
	return p.rw.SaveCharacter(ctx, c)
}
