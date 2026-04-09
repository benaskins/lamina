package cairn_test

import (
	"testing"

	"github.com/benaskins/cairn"
)

func TestSave_NaturalOneAlwaysPasses(t *testing.T) {
	engine := cairn.NewEngine()
	// With ability score 0, only a natural 1 can pass.
	char := &cairn.Sheet{ID: "test", CharName: "Test", STR: 0, DEX: 0, WIL: 0, CurrentHP: 6, MaxHP: 6}
	passes := 0
	for range 1000 {
		result := engine.Save(char, "STR", "test")
		if result.Passed {
			passes++
			if result.Roll != 1 {
				t.Errorf("passed with non-1 roll %d on score 0", result.Roll)
			}
		}
	}
	if passes == 0 {
		t.Error("expected some natural 1s in 1000 rolls")
	}
}

func TestSave_NaturalTwentyAlwaysFails(t *testing.T) {
	engine := cairn.NewEngine()
	char := &cairn.Sheet{ID: "test", CharName: "Test", STR: 20, DEX: 20, WIL: 20, CurrentHP: 6, MaxHP: 6}
	failures := 0
	for range 1000 {
		result := engine.Save(char, "STR", "test")
		if !result.Passed {
			failures++
			if result.Roll != 20 {
				t.Errorf("failed with non-20 roll %d on score 20", result.Roll)
			}
		}
	}
	if failures == 0 {
		t.Error("expected some natural 20s in 1000 rolls")
	}
}

func TestAttack_EnhancedRollsD12(t *testing.T) {
	engine := cairn.NewEngine()
	for range 200 {
		result := engine.Attack(cairn.AttackOpts{WeaponDie: 6, Armor: 0, Modifier: cairn.ModifierEnhanced})
		if result.Die != 12 {
			t.Errorf("enhanced attack should use d12, got d%d", result.Die)
		}
	}
}

func TestAttack_ImpairedRollsD4(t *testing.T) {
	engine := cairn.NewEngine()
	for range 200 {
		result := engine.Attack(cairn.AttackOpts{WeaponDie: 8, Armor: 0, Modifier: cairn.ModifierImpaired})
		if result.Die != 4 {
			t.Errorf("impaired attack should use d4, got d%d", result.Die)
		}
	}
}

func TestAttack_DamageMinimumZero(t *testing.T) {
	engine := cairn.NewEngine()
	for range 500 {
		result := engine.Attack(cairn.AttackOpts{WeaponDie: 4, Armor: 3})
		if result.Damage < 0 {
			t.Errorf("damage should never be negative, got %d", result.Damage)
		}
	}
}

func TestApplyDamage_OverflowToSTR(t *testing.T) {
	engine := cairn.NewEngine()
	char := &cairn.Sheet{ID: "test", CharName: "Test", STR: 10, CurrentHP: 3, MaxHP: 6}
	result := engine.ApplyDamage(char, 5)
	if result.NewHP != 0 {
		t.Errorf("HP should be 0 after overflow, got %d", result.NewHP)
	}
	if result.Overflow != 2 {
		t.Errorf("overflow should be 2, got %d", result.Overflow)
	}
	if !result.Critical {
		t.Error("overflow should set Critical=true")
	}
}

func TestHeal_BlockedWhenDeprived(t *testing.T) {
	engine := cairn.NewEngine()
	char := &cairn.Sheet{ID: "test", CharName: "Test", CurrentHP: 2, MaxHP: 6}
	for range 10 {
		char.Inventory = append(char.Inventory, cairn.Slot{Item: "torch"})
	}
	result := engine.Heal(char, 4)
	if !result.Blocked {
		t.Error("heal should be blocked when deprived")
	}
	if result.NewHP != 2 {
		t.Errorf("HP should not change when blocked, got %d", result.NewHP)
	}
}

func TestInventory_FullSetsDeprived(t *testing.T) {
	char := &cairn.Sheet{ID: "test"}
	for range 10 {
		char.Inventory = append(char.Inventory, cairn.Slot{Item: "torch"})
	}
	if !char.IsDeprived() {
		t.Error("character with 10 slots should be deprived")
	}
}

func TestInventory_BulkyTakesTwoSlots(t *testing.T) {
	char := &cairn.Sheet{ID: "test"}
	char.Inventory = append(char.Inventory, cairn.Slot{Item: "zweihander", Bulky: true})
	if char.SlotsUsed() != 2 {
		t.Errorf("bulky item should take 2 slots, got %d", char.SlotsUsed())
	}
}
