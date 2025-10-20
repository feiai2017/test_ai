package combat

import (
	"math"

	"test_ai/internal/config"
)

type SkillTemplate struct {
	ID       string
	Elem     string
	Range    float64
	Cooldown float64
	GlobalCD float64
	Damage   float64
	Priority int
	Applies  []AppliedState
	Tags     []string
	GuardBrk float64
	Note     string
}

type AppliedState struct {
	ID       string
	Duration float64
}

type HeroSkill struct {
	Template  SkillTemplate
	NextReady float64
}

func (hs *HeroSkill) Ready(now float64) bool {
	return now >= hs.NextReady
}

func (hs *HeroSkill) Trigger(now float64) {
	hs.NextReady = now + hs.Template.Cooldown
}

func (hs *HeroSkill) HasTag(tag string) bool {
	for _, t := range hs.Template.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

func (hs *HeroSkill) HasAnyTag(tags []string) bool {
	if len(tags) == 0 {
		return true
	}
	for _, tag := range tags {
		if hs.HasTag(tag) {
			return true
		}
	}
	return false
}

type SkillBook struct {
	byHero map[string][]SkillTemplate
	byElem map[string][]SkillTemplate
}

func NewSkillBook(cfg *config.SkillsConfig) *SkillBook {
	sb := &SkillBook{
		byHero: map[string][]SkillTemplate{},
		byElem: map[string][]SkillTemplate{},
	}
	if cfg == nil {
		return sb
	}
	for _, s := range cfg.Skills {
		tpl := SkillTemplate{
			ID:       s.ID,
			Elem:     s.Elem,
			Range:    s.Range,
			Cooldown: s.CD,
			GlobalCD: s.GCD,
			Damage:   s.Damage,
			Priority: s.Priority,
			Tags:     append([]string(nil), s.Tags...),
			GuardBrk: s.GuardBrk,
			Note:     s.Note,
		}
		if len(s.Applies) > 0 {
			tpl.Applies = make([]AppliedState, len(s.Applies))
			for i, ap := range s.Applies {
				tpl.Applies[i] = AppliedState{ID: ap.ID, Duration: ap.Duration}
			}
		}
		if s.Hero != "" {
			sb.byHero[s.Hero] = append(sb.byHero[s.Hero], tpl)
			continue
		}
		if s.Elem != "" {
			sb.byElem[s.Elem] = append(sb.byElem[s.Elem], tpl)
			continue
		}
		// fallback bucket (no hero / elem) acts as generic default
		sb.byElem["*"] = append(sb.byElem["*"], tpl)
	}
	return sb
}

func (sb *SkillBook) templatesFor(heroID, elem string) []SkillTemplate {
	if sb == nil {
		return nil
	}
	if heroID != "" {
		if v := sb.byHero[heroID]; len(v) > 0 {
			return v
		}
	}
	if elem != "" {
		if v := sb.byElem[elem]; len(v) > 0 {
			return v
		}
	}
	return sb.byElem["*"]
}

func (sb *SkillBook) Instantiate(heroID, elem string) []*HeroSkill {
	tpls := sb.templatesFor(heroID, elem)
	if len(tpls) == 0 {
		tpls = []SkillTemplate{
			{
				ID:       "basic." + elem,
				Elem:     elem,
				Range:    3.0,
				Cooldown: 1.0,
				Damage:   90,
				Priority: 10,
				Applies:  nil,
			},
		}
	}
	out := make([]*HeroSkill, len(tpls))
	for i := range tpls {
		out[i] = &HeroSkill{Template: tpls[i]}
	}
	return out
}

func maxRangeOf(skills []*HeroSkill) float64 {
	maxRange := 0.0
	for _, sk := range skills {
		if sk.Template.Range > maxRange {
			maxRange = sk.Template.Range
		}
	}
	return maxRange
}

func minCooldownOf(skills []*HeroSkill) float64 {
	minCD := math.MaxFloat64
	for _, sk := range skills {
		if sk.Template.Cooldown > 0 && sk.Template.Cooldown < minCD {
			minCD = sk.Template.Cooldown
		}
	}
	if minCD == math.MaxFloat64 {
		return 1.0
	}
	return minCD
}
