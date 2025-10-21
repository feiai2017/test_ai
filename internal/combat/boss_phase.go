package combat

import "test_ai/internal/config"

type BossPhaser struct {
	Cfg   *config.BossConfig
	Boss  *Entity
	Emit  func(Event)
	phase int
}

func NewBossPhaser(cfg *config.BossConfig, boss *Entity, emit func(Event)) *BossPhaser {
	bp := &BossPhaser{Cfg: cfg, Boss: boss, Emit: emit}
	if cfg != nil && len(cfg.Phases) > 0 {
		bp.applyPhase(0, 0)
	}
	return bp
}

func (bp *BossPhaser) CurrentPhase() int { return bp.phase }

func (bp *BossPhaser) PhaseSpec() *config.Phase {
	if bp.Cfg == nil || bp.phase < 0 || bp.phase >= len(bp.Cfg.Phases) {
		return nil
	}
	return &bp.Cfg.Phases[bp.phase]
}

func (bp *BossPhaser) Tick(now float64) bool {
	if bp.Cfg == nil {
		return false
	}
	hpPct := float64(bp.Boss.HP) / float64(bp.Boss.MaxHP)
	next := bp.phase + 1
	if next < len(bp.Cfg.Phases) && hpPct <= bp.Cfg.Phases[next].Threshold {
		bp.applyPhase(next, now)
		return true
	}
	return false
}

func (bp *BossPhaser) applyPhase(to int, now float64) {
	if bp.Cfg == nil || to < 0 || to >= len(bp.Cfg.Phases) {
		return
	}
	bp.phase = to
	on := bp.Cfg.Phases[to].OnEnter
	if on.Announce != "" {
		bp.Emit(Event{T: now, Type: "Announce", Payload: map[string]any{
			"text":  on.Announce,
			"phase": to,
			"tags":  on.Tags,
		}})
	}
	if on.SetGuard > 0 {
		bp.Boss.Guard = on.SetGuard
		if bp.Boss.Guard > bp.Boss.GuardMax {
			bp.Boss.Guard = bp.Boss.GuardMax
		}
		bp.Emit(Event{T: now, Type: "GuardChanged", Payload: map[string]any{
			"guard": bp.Boss.Guard,
		}})
	}
	if on.ResistDelta != nil {
		for k, v := range on.ResistDelta {
			bp.Boss.Resist[k] += v
		}
	}
	bp.Boss.Weakness = map[string]bool{}
	for _, w := range on.Weakness {
		bp.Boss.Weakness[w] = true
	}
	bp.Emit(Event{T: now, Type: "PhaseEnter", Payload: map[string]any{
		"phase": to,
		"note":  on.Note,
	}})
}
