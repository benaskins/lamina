package game

// Character is a game entity with an identity, hit points, and conditions.
type Character interface {
	GetID() string
	GetName() string
	GetHP() int
	GetMaxHP() int
	GetArmor() int
	IsAlive() bool
	IsDeprived() bool
	Ability(name string) int
}

// SaveResult holds the outcome of a save attempt.
type SaveResult struct {
	Ability  string
	Score    int
	Roll     int
	Passed   bool
	Critical bool // natural 1 (always pass) or 20 (always fail)
	Detail   string
}

// AttackModifier adjusts the attack die.
type AttackModifier int

const (
	ModifierNormal   AttackModifier = 0
	ModifierEnhanced AttackModifier = 1  // roll d12
	ModifierImpaired AttackModifier = -1 // roll d4
)

// AttackOpts configures an attack roll.
type AttackOpts struct {
	WeaponDie int
	Armor     int
	Modifier  AttackModifier
}

// AttackResult is the outcome of an attack.
type AttackResult struct {
	Die    int
	Roll   int
	Armor  int
	Damage int
	Detail string
}

// DamageResult is the outcome of applying damage to a character.
type DamageResult struct {
	OldHP    int
	NewHP    int
	Overflow int  // damage beyond 0 HP that hits primary ability
	Critical bool // overflow was triggered
	Detail   string
}

// HealResult is the outcome of healing.
type HealResult struct {
	OldHP   int
	NewHP   int
	Blocked bool // healing was blocked (e.g., Deprived)
	Detail  string
}

// ReactionResult holds the outcome of an NPC reaction roll.
type ReactionResult struct {
	Roll        int
	Disposition string // "hostile", "wary", "curious", "kind", "helpful"
}

// RulesEngine resolves game mechanics for a rule system.
type RulesEngine interface {
	Save(char Character, ability string, reason string) SaveResult
	Attack(opts AttackOpts) AttackResult
	ApplyDamage(char Character, amount int) DamageResult
	Heal(char Character, amount int) HealResult
	Name() string
	RulesText() string
}
