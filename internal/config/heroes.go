package config

type HeroesConfig struct {
	Heroes []HeroDef `yaml:"heroes"`
}

type HeroDef struct {
	ID      string             `yaml:"id"`
	Name    string             `yaml:"name"`
	Element string             `yaml:"element"`
	Tags    []string           `yaml:"tags"`
	MaxHP   int                `yaml:"max_hp"`
	Speed   float64            `yaml:"speed"`
	Spawn   Vec2Def            `yaml:"spawn"`
	Resist  map[string]float64 `yaml:"resist"`
	Note    string             `yaml:"note"`
}

type Vec2Def struct {
	X float64 `yaml:"x"`
	Y float64 `yaml:"y"`
}
