package config

type ElementsConfig struct {
	Elements []Element `yaml:"elements"`
}
type Element struct {
	ID      string   `yaml:"id"`
	TagsAdd []string `yaml:"tags_add"`
	Mods    struct {
		DamageScalar float64 `yaml:"damage_scalar"`
		Dot          struct {
			ID       string  `yaml:"id"`
			Tick     float64 `yaml:"tick"`
			Duration float64 `yaml:"duration"`
			Scale    float64 `yaml:"scale"`
		} `yaml:"dot"`
		Chain struct {
			Jumps   int     `yaml:"jumps"`
			Falloff float64 `yaml:"falloff"`
		} `yaml:"chain"`
		Debuff struct {
			ID       string             `yaml:"id"`
			Duration float64            `yaml:"duration"`
			Vuln     map[string]float64 `yaml:"vuln"`
		} `yaml:"debuff"`
	} `yaml:"mods"`
}
