package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func loadYAML(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, out)
}

func LoadAll(dir string) (*ElementsConfig, *ReactionsConfig, *SkillsConfig, *HeroesConfig, *BossConfig, error) {
	var ec ElementsConfig
	var rc ReactionsConfig
	var sc SkillsConfig
	var hc HeroesConfig
	var bc BossConfig
	if err := loadYAML(filepath.Join(dir, "elements.yaml"), &ec); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if err := loadYAML(filepath.Join(dir, "skills.yaml"), &sc); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if err := loadYAML(filepath.Join(dir, "reactions.yaml"), &rc); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if err := loadYAML(filepath.Join(dir, "heroes.yaml"), &hc); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if err := loadYAML(filepath.Join(dir, "boss_phases", "boss001.yaml"), &bc); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return &ec, &rc, &sc, &hc, &bc, nil
}
