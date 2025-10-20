package config

type SkillsConfig struct {
	Skills []Skill `yaml:"skills"`
}

type Skill struct {
	ID       string         `yaml:"id"`
	Hero     string         `yaml:"hero"`
	Elem     string         `yaml:"elem"`
	Range    float64        `yaml:"range"`
	CD       float64        `yaml:"cd"`
	Damage   float64        `yaml:"damage"`
	Priority int            `yaml:"priority"`
	GCD      float64        `yaml:"gcd"`
	Applies  []AppliedState `yaml:"applies"`
	Tags     []string       `yaml:"tags"`
	GuardBrk float64        `yaml:"guard_break"`
	Note     string         `yaml:"note"`
}

type AppliedState struct {
	ID       string  `yaml:"id"`
	Duration float64 `yaml:"duration"`
}
