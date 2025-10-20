package config

type ReactionsConfig struct {
	Reactions []Reaction `yaml:"reactions"`
}

type Reaction struct {
	ID      string   `yaml:"id"`
	Note    string   `yaml:"note"`
	When    []string `yaml:"when"`
	ICD     float64  `yaml:"icd"`
	Effects []Effect `yaml:"effects"`
}

type Effect struct {
	Type        string             `yaml:"type"`
	Value       float64            `yaml:"value"`
	Duration    float64            `yaml:"duration"`
	Status      string             `yaml:"status"`
	DotScaleAdd float64            `yaml:"dot_scale_add"`
	Statuses    []string           `yaml:"statuses"`
	Table       map[string]float64 `yaml:"table"`
	Amount      float64            `yaml:"amount"`
	Tags        []string           `yaml:"tags"`
	Note        string             `yaml:"note"`
}
