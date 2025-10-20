package combat

type Hero struct {
	ID     string
	Elem   string
	Tags   map[string]bool
	Entity *Entity
	Skills []*HeroSkill
}

type PartyController struct {
	Heroes       []Hero
	ActiveIndex  int
	SwitchCD     float64
	LastSwitchAt float64
}

func NewParty(heroes []Hero, book *SkillBook) *PartyController {
	pc := &PartyController{Heroes: heroes, SwitchCD: 8.0, LastSwitchAt: -9999}
	sb := book
	if sb == nil {
		sb = NewSkillBook(nil)
	}
	for i := range pc.Heroes {
		if pc.Heroes[i].Tags == nil {
			pc.Heroes[i].Tags = map[string]bool{}
		}
		pc.Heroes[i].Skills = sb.Instantiate(pc.Heroes[i].ID, pc.Heroes[i].Elem)
	}
	return pc
}
func (p *PartyController) Active() *Hero              { return &p.Heroes[p.ActiveIndex] }
func (p *PartyController) CanSwitch(now float64) bool { return now-p.LastSwitchAt >= p.SwitchCD }
func (p *PartyController) TrySwitchTo(i int, now float64) bool {
	if i < 0 || i >= len(p.Heroes) || i == p.ActiveIndex || !p.CanSwitch(now) {
		return false
	}
	p.ActiveIndex = i
	p.LastSwitchAt = now
	return true
}
func (p *PartyController) NextBenchIndex() int {
	if len(p.Heroes) == 0 {
		return 0
	}
	return (p.ActiveIndex + 1) % len(p.Heroes)
}

func (h *Hero) ReadySkill(now float64) *HeroSkill {
	var best *HeroSkill
	for _, sk := range h.Skills {
		if !sk.Ready(now) {
			continue
		}
		if best == nil || sk.Template.Priority > best.Template.Priority {
			best = sk
			continue
		}
		if best != nil && sk.Template.Priority == best.Template.Priority && sk.Template.ID < best.Template.ID {
			best = sk
		}
	}
	return best
}

func (h *Hero) MaxRange() float64 {
	if len(h.Skills) == 0 {
		return 3.0
	}
	return maxRangeOf(h.Skills)
}

func (h *Hero) MinCooldown() float64 {
	if len(h.Skills) == 0 {
		return 1.0
	}
	return minCooldownOf(h.Skills)
}

func (h *Hero) AdjustCooldown(now float64, amount float64, tags []string) {
	if amount == 0 {
		return
	}
	for _, sk := range h.Skills {
		if !sk.HasAnyTag(tags) {
			continue
		}
		sk.NextReady -= amount
		if sk.NextReady < now {
			sk.NextReady = now
		}
	}
}

// —— 自动轮换策略 —— //
type SwitchPolicy interface {
	Next(env *Env, p *PartyController) (int, bool)
}
type RoundRobinPolicy struct {
	Interval   float64
	nextSwitch float64
}

func (rr *RoundRobinPolicy) Next(env *Env, p *PartyController) (int, bool) {
	if env.Time >= rr.nextSwitch {
		rr.nextSwitch = env.Time + rr.Interval
		return p.NextBenchIndex(), true
	}
	return -1, false
}
