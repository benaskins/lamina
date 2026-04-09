package cairn

// Slot is one entry in a character's 10-slot inventory.
type Slot struct {
	Item    string
	Bulky   bool // occupies 2 slots
	Fatigue bool // marks a fatigue token (occupies 1 slot)
}

// Scar is a permanent consequence of reaching exactly 0 HP.
type Scar struct {
	Description string
	Source      string
}

// Sheet is a Cairn character sheet.
type Sheet struct {
	ID         string
	CharName   string
	Background string
	STR, DEX, WIL          int
	MaxSTR, MaxDEX, MaxWIL int
	CurrentHP  int
	MaxHP      int
	ArmorValue int // 0=none, 1=leather/brigandine, 2=chain, 3=plate; +1 for shield
	Gold       int
	Inventory  []Slot
	Scars      []Scar
	Dead       bool
}

func (s *Sheet) GetID() string    { return s.ID }
func (s *Sheet) GetName() string  { return s.CharName }
func (s *Sheet) GetHP() int       { return s.CurrentHP }
func (s *Sheet) GetMaxHP() int    { return s.MaxHP }
func (s *Sheet) GetArmor() int    { return s.ArmorValue }
func (s *Sheet) IsAlive() bool    { return !s.Dead && s.STR > 0 }
func (s *Sheet) IsDeprived() bool { return s.SlotsUsed() >= 10 }

func (s *Sheet) Ability(name string) int {
	switch name {
	case "STR":
		return s.STR
	case "DEX":
		return s.DEX
	case "WIL":
		return s.WIL
	}
	return 0
}

// SlotsUsed returns total inventory slots consumed. Bulky items take 2 slots.
func (s *Sheet) SlotsUsed() int {
	total := 0
	for _, slot := range s.Inventory {
		if slot.Bulky {
			total += 2
		} else {
			total++
		}
	}
	return total
}
