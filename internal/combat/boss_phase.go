package combat

import (
	"test_ai/internal/config"
)

type BossPhaser struct {
	Cfg   *config.BossConfig
	Boss  *Entity
	Emit  func(Event)
	phase int
}

func NewBossPhaser(cfg *config.BossConfig, boss *Entity, emit func(Event)) *BossPhaser {
	// 初始阶段效果（按需）
	if len(cfg.Phases) > 0 {
		on := cfg.Phases[0].OnEnter
		if on.ResistDelta != nil {
			for k, v := range on.ResistDelta {
				boss.Resist[k] += v
			}
		}
		for _, w := range on.Weakness {
			boss.Weakness[w] = true
		}
	}
	return &BossPhaser{Cfg: cfg, Boss: boss, Emit: emit}
}

func (bp *BossPhaser) Tick(now float64) {
	hpPct := float64(bp.Boss.HP) / float64(bp.Boss.MaxHP)
	next := bp.phase + 1
	if next < len(bp.Cfg.Phases) && hpPct <= bp.Cfg.Phases[next].Threshold {
		bp.phase = next
		on := bp.Cfg.Phases[next].OnEnter
		if on.Announce != "" {
			bp.Emit(Event{T: now, Type: "Announce", Payload: map[string]any{"text": on.Announce}})
		}
		if on.SetGuard > 0 {
			bp.Boss.Guard = on.SetGuard
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
		bp.Emit(Event{T: now, Type: "PhaseEnter", Payload: map[string]any{"phase": bp.phase}})
	}
}
