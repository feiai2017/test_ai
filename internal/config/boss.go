package config

type BossConfig struct {
	ID       string       `yaml:"id"`
	Note     string       `yaml:"note"`
	MaxHP    int          `yaml:"max_hp"`
	GuardMax int          `yaml:"guard_max"`
	Weaken   WeakenConfig `yaml:"weaken"`
	Intents  []BossIntent `yaml:"intents"`
	Phases   []Phase      `yaml:"phases"`
}

type WeakenConfig struct {
	Duration    float64 `yaml:"duration"`
	DamageAmp   float64 `yaml:"damage_amp"`
	GuardReturn int     `yaml:"guard_return"`
	Note        string  `yaml:"note"`
}

type BossIntent struct {
	ID            string   `yaml:"id"`
	Name          string   `yaml:"name"`
	Elem          string   `yaml:"elem"`
	Damage        float64  `yaml:"damage"`
	Range         float64  `yaml:"range"`
	Telegraph     float64  `yaml:"telegraph"`
	Cast          float64  `yaml:"cast"`
	Recovery      float64  `yaml:"recovery"`
	InterruptNeed int      `yaml:"interrupt_guard"`
	GuardDamage   int      `yaml:"guard_damage"`
	Tags          []string `yaml:"tags"`
	Radius        float64  `yaml:"radius"`
	MoveDuring    bool     `yaml:"move_during"`
	UpgradeOf     string   `yaml:"upgrade_of"`
	Note          string   `yaml:"note"`
}

type Phase struct {
	Threshold   float64       `yaml:"threshold"`
	Note        string        `yaml:"note"`
	OnEnter     PhaseEnter    `yaml:"on_enter"`
	Intents     []PhaseIntent `yaml:"intents"`
	MiniClimax  ClimaxConfig  `yaml:"mini_climax"`
	MajorClimax ClimaxConfig  `yaml:"major_climax"`
}

type PhaseEnter struct {
	Announce    string             `yaml:"announce"`
	SetGuard    int                `yaml:"set_guard"`
	ResistDelta map[string]float64 `yaml:"resist_delta"`
	Weakness    []string           `yaml:"weakness"`
	Note        string             `yaml:"note"`
}

type PhaseIntent struct {
	Intent    string  `yaml:"intent"`
	Weight    float64 `yaml:"weight"`
	UpgradeOf string  `yaml:"upgrade_of"`
	Note      string  `yaml:"note"`
}

type ClimaxConfig struct {
	Intent      string  `yaml:"intent"`
	IntervalMin float64 `yaml:"interval_min"`
	IntervalMax float64 `yaml:"interval_max"`
	Note        string  `yaml:"note"`
}
