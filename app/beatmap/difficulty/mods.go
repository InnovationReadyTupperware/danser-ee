package difficulty

import (
	"log"

	"github.com/wieku/rplpa"
)

type Modifier int64

const (
	None   = Modifier(iota)
	NoFail = Modifier(1 << (iota - uint(1)))
	Easy
	TouchDevice
	Hidden
	HardRock
	SuddenDeath
	DoubleTime
	Relax
	HalfTime
	Nightcore // Only set along with DoubleTime. i.e: NC only gives 576
	Flashlight
	Autoplay
	SpunOut
	Autopilot // Autopilot
	Perfect   // Only set along with SuddenDeath. i.e: PF only gives 16416
	Key4
	Key5
	Key6
	Key7
	Key8
	FadeIn
	Random
	Cinema
	Target
	Key9
	KeyCoop
	Key1
	Key3
	Key2
	ScoreV2
	LastMod
	Daycore
	// Keep the former Lazer bit occupied. Modifier values are persisted in
	// replay and command data, so shifting later bits would reinterpret
	// Classic, DifficultyAdjust, Mirror, and Traceable in old data.
	reservedLazerModifier
	Classic
	DifficultyAdjust
	Mirror
	Traceable

	// DifficultyAdjustMask is outdated, use GetDiffMaskedMods instead
	DifficultyAdjustMask    = HardRock | Easy | DoubleTime | Nightcore | HalfTime | Daycore | Flashlight | Relax
	difficultyAdjustMaskNew = HardRock | Easy | DoubleTime | HalfTime | Flashlight | Relax | Autopilot | TouchDevice
)

// GetDiffMaskedMods returns mods that can affect star-rating attributes.
// Hidden is included because the 20260706 Reading skill changes SR under HD;
// older calculators may calculate an equivalent NM/HD result separately.
func GetDiffMaskedMods(mods Modifier) Modifier {
	// Normalize standalone NC/DC bits to their underlying speed mods for stable difficulty keys
	if mods.Active(Nightcore) {
		mods = (mods & (^Nightcore)) | DoubleTime
	}

	if mods.Active(Daycore) {
		mods = (mods & (^Daycore)) | HalfTime
	}

	return (difficultyAdjustMaskNew | Hidden) & mods
}

var modsString = [...]string{
	"NF",
	"EZ",
	"TD",
	"HD",
	"HR",
	"SD",
	"DT",
	"RX",
	"HT",
	"NC",
	"FL",
	"AT", // Auto.
	"SO",
	"AP", // Autopilot.
	"PF",
	"K4",
	"K5",
	"K6",
	"K7",
	"K8",
	"FI",
	"RN", // Random
	"CN",
	"TG",
	"K9",
	"K0",
	"K1",
	"K3",
	"K2",
	"V2",
	"LM",
	"DC",
	"",
	"CL",
	"DA",
	"MR",
	"TC",
}

var modsStringFull = [...]string{
	"NoFail",
	"Easy",
	"TouchDevice",
	"Hidden",
	"HardRock",
	"SuddenDeath",
	"DoubleTime",
	"Relax",
	"HalfTime",
	"Nightcore",
	"Flashlight",
	"Autoplay",
	"SpunOut",
	"Autopilot",
	"Perfect",
	"Key4",
	"Key5",
	"Key6",
	"Key7",
	"Key8",
	"FadeIn",
	"Random",
	"Cinema",
	"Target",
	"Key9",
	"KeyCoop",
	"Key1",
	"Key3",
	"Key2",
	"ScoreV2",
	"LastMod",
	"Daycore",
	"",
	"Classic",
	"DifficultyAdjust",
	"Mirror",
	"Traceable",
}

var knownUnsupportedLazerPerformanceMods = map[string]string{
	"BL": "Blinds",
	"DF": "Deflate",
	"MG": "Magnetised",
}

func (mods Modifier) GetScoreMultiplier() float64 {
	multiplier := 1.0

	if mods&NoFail > 0 && mods&ScoreV2 == 0 {
		multiplier *= 0.5
	}

	if mods&Easy > 0 {
		multiplier *= 0.5
	}

	if mods&HalfTime > 0 {
		multiplier *= 0.3
	}

	if mods&Hidden > 0 {
		multiplier *= 1.06
	}

	if mods&HardRock > 0 {
		if mods&ScoreV2 > 0 {
			multiplier *= 1.10
		} else {
			multiplier *= 1.06
		}
	}

	if mods&DoubleTime > 0 {
		if mods&ScoreV2 > 0 {
			multiplier *= 1.20
		} else {
			multiplier *= 1.12
		}
	}

	if mods&Flashlight > 0 {
		multiplier *= 1.12
	}

	if (mods&Relax | mods&Autopilot) > 0 {
		// Relax and Autopilot have different multipliers in the two
		// gameplay implementations. Difficulty.GetScoreMultiplier owns
		// that decision because Modifier does not carry gameplay provenance.
		multiplier = 0
	}

	if mods&SpunOut > 0 {
		multiplier *= 0.9
	}

	if mods&Classic > 0 {
		multiplier *= 0.96
	}

	if mods&DifficultyAdjust > 0 {
		multiplier *= 0.5
	}

	return multiplier
}

func (mods Modifier) String() (s string) {
	mods &^= reservedLazerModifier

	if mods.Active(Nightcore) {
		mods &= ^DoubleTime
	}

	if mods.Active(Daycore) {
		mods &= ^HalfTime
	}

	if mods.Active(Perfect) {
		mods &= ^SuddenDeath
	}

	for i := range len(modsString) {
		activated := mods&1 == 1
		if activated {
			s += modsString[i]
		}

		mods >>= 1
	}

	return
}

func (mods Modifier) StringFull() (s []string) {
	mods &^= reservedLazerModifier

	if mods.Active(Nightcore) {
		mods &= ^DoubleTime
	}

	if mods.Active(Daycore) {
		mods &= ^HalfTime
	}

	if mods.Active(Perfect) {
		mods &= ^SuddenDeath
	}

	return mods.StringFull2()
}

func (mods Modifier) StringFull2() (s []string) {
	mods &^= reservedLazerModifier

	for i := range len(modsString) {
		activated := mods&1 == 1
		if activated && modsStringFull[i] != "" {
			s = append(s, modsStringFull[i])
		}

		mods >>= 1
	}

	return
}

func ParseFromAcronym(mod string) (m Modifier) {
	for index, availableMod := range modsString {
		if availableMod != "" && availableMod == mod {
			m = 1 << uint(index)
			break
		}
	}

	if m == None && mod != "" {
		if name, ok := knownUnsupportedLazerPerformanceMods[mod]; ok {
			log.Printf("Ignoring unsupported osu!lazer %s mod %q; star rating/performance may differ", name, mod)
		} else {
			log.Printf("Ignoring unknown mod acronym %q", mod)
		}
	}

	return
}

func (mods Modifier) ConvertToModInfoList() (mi []rplpa.ModInfo) {
	mods &^= reservedLazerModifier

	if mods.Active(Nightcore) {
		mods &= ^DoubleTime
	}

	if mods.Active(Daycore) {
		mods &= ^HalfTime
	}

	if mods.Active(Perfect) {
		mods &= ^SuddenDeath
	}

	for i := range len(modsString) {
		if mods&1 == 1 && modsString[i] != "" {
			mi = append(mi, rplpa.ModInfo{
				Acronym:  modsString[i],
				Settings: make(map[string]any),
			})
		}

		mods >>= 1
	}

	return
}

func ParseMods(mods string) (m Modifier) {
	modsSl := make([]string, len(mods)/2)
	for n, modPart := range mods {
		modsSl[n/2] += string(modPart)
	}

	for _, mod := range modsSl {
		m |= ParseFromAcronym(mod)
	}

	if m.Active(Nightcore) {
		m |= DoubleTime
	}

	if m.Active(Perfect) {
		m |= SuddenDeath
	}

	if m.Active(Daycore) {
		m |= HalfTime
	}

	return
}

func (mods Modifier) Active(mod Modifier) bool {
	return mods&mod > 0
}

// GetIncompatibleMods returns the modifiers excluded by one mod in mode
// Classic changes Lazer settings, not the provenance used by this matrix
func (mod Modifier) GetIncompatibleMods(mode GameplayMode) Modifier {
	var incompatible Modifier
	switch mod {
	case Easy:
		incompatible = HardRock | DifficultyAdjust
	case HardRock:
		incompatible = Easy | DifficultyAdjust | Mirror
	case DifficultyAdjust:
		incompatible = Easy | HardRock
	case Mirror:
		incompatible = HardRock
	case NoFail:
		incompatible = SuddenDeath | Perfect | Cinema
	case SuddenDeath:
		incompatible = NoFail | Perfect | Cinema
	case Perfect:
		incompatible = NoFail | SuddenDeath | Cinema
	case DoubleTime:
		incompatible = Nightcore | HalfTime | Daycore
	case Nightcore:
		incompatible = DoubleTime | HalfTime | Daycore
	case HalfTime:
		incompatible = Daycore | DoubleTime | Nightcore
	case Daycore:
		incompatible = HalfTime | DoubleTime | Nightcore
	case Hidden:
		incompatible = Traceable
	case Traceable:
		incompatible = Hidden
	case Relax:
		incompatible = Autoplay | Autopilot | Cinema
	case Autopilot:
		incompatible = Relax | SpunOut | Autoplay | TouchDevice | Cinema
	case SpunOut:
		incompatible = Autopilot | Autoplay | Cinema
	case Autoplay:
		incompatible = Relax | Autopilot | SpunOut | TouchDevice | Cinema
	case TouchDevice:
		incompatible = Autoplay | Autopilot | Cinema
	case Cinema:
		incompatible = NoFail | SuddenDeath | Perfect | Relax | Autoplay | Autopilot | SpunOut | TouchDevice
	case Classic:
		if mode.IsLazer() {
			incompatible = ScoreV2
		}
	case ScoreV2:
		if mode.IsLazer() {
			incompatible = Classic
		}
	}

	if mode == GameplayStable {
		switch mod {
		case NoFail, SuddenDeath, Perfect:
			incompatible |= Relax | Autopilot
		case Relax, Autopilot:
			incompatible |= NoFail | SuddenDeath | Perfect
		}
	}
	return incompatible
}

// GetIncompatibleComboForMode returns the first conflicting set of modifiers
// Replay bitmasks encode NC, DC, and PF together with their underlying mods
func (mods Modifier) GetIncompatibleComboForMode(mode GameplayMode) Modifier {
	mods &^= reservedLazerModifier
	if mods.Active(Target) {
		return Target
	}
	if mods.Active(Nightcore) {
		mods &^= DoubleTime
	}
	if mods.Active(Daycore) {
		mods &^= HalfTime
	}
	if mods.Active(Perfect) {
		mods &^= SuddenDeath
	}
	for i := range len(modsString) {
		mod := Modifier(1 << i)
		if incompatible := mods & mod.GetIncompatibleMods(mode); mods.Active(mod) && incompatible != None {
			return mod | incompatible
		}
	}
	return None
}

// GetIncompatibleCombo checks this difficulty using its gameplay provenance
func (diff *Difficulty) GetIncompatibleCombo() Modifier {
	return diff.Mods.GetIncompatibleComboForMode(diff.GetGameplayMode())
}

// Compatible checks modifiers using the default Lazer gameplay mode
// Replay callers must instead validate a Difficulty with their source mode
func (mods Modifier) Compatible() bool {
	return mods.GetIncompatibleComboForMode(GameplayLazer) == None
}
