package combat

import (
	"encoding/json"
	"math/rand"
)

// slotLocal defines three anchor offsets for the triangle formation.
var slotLocal = struct {
	Apex, Left, Right Vec2
}{
	Apex:  Vec2{X: 0.0, Y: 0.0},
	Left:  Vec2{X: -1.2, Y: +0.8},
	Right: Vec2{X: -1.2, Y: -0.8},
}

// rotateByFwd rotates a local offset so that X points at the boss and Y is the right vector.
func rotateByFwd(local Vec2, fwd Vec2) Vec2 {
	right := Vec2{X: fwd.Y, Y: -fwd.X}
	return Vec2{X: fwd.X*local.X + right.X*local.Y, Y: fwd.Y*local.X + right.Y*local.Y}
}

type SimInput struct {
	BossID string `json:"boss_id"`
	Seed   int64  `json:"seed"`
	Party  []Hero `json:"party"`
	Ops    []struct {
		T  float64 `json:"t"`
		Op string  `json:"op"`
		To int     `json:"to"`
	} `json:"ops"`
}

type SimResult struct {
	Win           bool               `json:"win"`
	Duration      float64            `json:"duration"`
	Events        []Event            `json:"events,omitempty"`
	DPS           float64            `json:"dps"`
	Reactions     map[string]int     `json:"reactions"`
	DamageBySkill map[string]float64 `json:"damage_by_skill,omitempty"`
	DamageByHero  map[string]float64 `json:"damage_by_hero,omitempty"`
}

type Env struct {
	Time  float64
	Delta float64
	Rng   *rand.Rand
}

func RunSingle(env *Env, boss *Entity, party *PartyController, rr *ReactionResolver, ph *BossPhaser, record bool) SimResult {
	var events []Event
	emit := func(ev Event) {
		if record {
			events = append(events, ev)
		}
	}
	rr.Emit = emit

	damageBySkill := map[string]float64{}
	damageByHero := map[string]float64{}
	reacHist := map[string]int{}
	reactByHero := map[string]float64{}
	totalDamage := 0.0

	spawns := []Vec2{{X: 1, Y: 3}, {X: 1, Y: 5}, {X: 1, Y: 7}}
	for i := range party.Heroes {
		h := &party.Heroes[i]
		if len(h.Skills) == 0 {
			h.Skills = NewSkillBook(nil).Instantiate(h.ID, h.Elem)
		}
		h.Entity = &Entity{
			ID:              h.ID,
			HP:              5000,
			MaxHP:           5000,
			Pos:             spawns[i%len(spawns)],
			Speed:           3.0 + float64(i)*0.5,
			Range:           h.MaxRange(),
			AtkCD:           h.MinCooldown(),
			Resist:          map[string]float64{},
			Weakness:        map[string]bool{},
			Statuses:        map[string]Status{},
			Tags:            map[string]bool{},
			BuffDMGTakenMul: 1.0,
		}
		emit(Event{T: env.Time, Type: "Spawn", Payload: map[string]any{
			"id": h.ID, "x": h.Entity.Pos.X, "y": h.Entity.Pos.Y,
		}})
	}
	emit(Event{T: env.Time, Type: "Spawn", Payload: map[string]any{
		"id": boss.ID, "x": boss.Pos.X, "y": boss.Pos.Y, "boss": true,
	}})

	bAI := NewBossAI()
	policy := &RoundRobinPolicy{Interval: 5.0, nextSwitch: 5.0}

	slotIdx := [3]int{0, 1, 2}
	reassignSlots := func(activeIndex int) {
		switch activeIndex {
		case 0:
			slotIdx = [3]int{0, 1, 2}
		case 1:
			slotIdx = [3]int{1, 0, 2}
		case 2:
			slotIdx = [3]int{2, 0, 1}
		}
	}
	reassignSlots(party.ActiveIndex)

	rr.OnReactionDamage = func(_ string, amount float64, sourceHeroID string) {
		reactByHero[sourceHeroID] += amount
	}
	rr.OnCooldownAdjust = func(heroID string, amount float64, tags []string) {
		if heroID == "" || amount == 0 {
			return
		}
		for i := range party.Heroes {
			if party.Heroes[i].ID == heroID {
				party.Heroes[i].AdjustCooldown(env.Time, amount, tags)
				break
			}
		}
	}
	rr.OnReaction = func(id string, _ string, _ string) {
		if id != "" {
			reacHist[id]++
		}
	}

	env.Delta = 0.1
	for env.Time = 0; env.Time < 120 && boss.HP > 0; env.Time += env.Delta {
		if idx, ok := policy.Next(env, party); ok && party.TrySwitchTo(idx, env.Time) {
			emit(Event{T: env.Time, Type: "Switch", Payload: map[string]any{
				"to": party.Active().ID, "index": idx,
			}})
			reassignSlots(idx)
		}

		active := party.Active()
		leader := active.Entity
		toBoss := boss.Pos.Sub(leader.Pos)
		distLB := toBoss.Len()
		fwd := toBoss.Norm()

		if distLB > leader.Range {
			step := leader.Speed * env.Delta
			if step > distLB {
				step = distLB
			}
			if step > 0 {
				old := leader.Pos
				leader.Pos = leader.Pos.Add(fwd.Scale(step))
				emit(Event{T: env.Time, Type: "Move", Payload: map[string]any{
					"id": leader.ID, "from": []float64{old.X, old.Y}, "to": []float64{leader.Pos.X, leader.Pos.Y},
				}})
			}
		}

		for slot := 0; slot < 3; slot++ {
			idxHero := slotIdx[slot]
			h := &party.Heroes[idxHero]
			if h.Entity == nil || h.Entity == leader {
				continue
			}
			var local Vec2
			switch slot {
			case 0:
				local = slotLocal.Apex
			case 1:
				local = slotLocal.Left
			case 2:
				local = slotLocal.Right
			}
			targetPos := leader.Pos.Add(rotateByFwd(local, fwd))
			me := h.Entity
			diff := targetPos.Sub(me.Pos)
			d := diff.Len()
			if d > 0.01 {
				step := me.Speed * env.Delta
				if step > d {
					step = d
				}
				old := me.Pos
				me.Pos = me.Pos.Add(diff.Norm().Scale(step))
				emit(Event{T: env.Time, Type: "Move", Payload: map[string]any{
					"id": me.ID, "from": []float64{old.X, old.Y}, "to": []float64{me.Pos.X, me.Pos.Y},
				}})
			}
		}

		ready := active.ReadySkill(env.Time)
		if ready != nil && env.Time >= leader.nextAtk && leader.Pos.Sub(boss.Pos).Len() <= ready.Template.Range {
			ready.Trigger(env.Time)
			if ready.Template.GlobalCD > 0 {
				leader.nextAtk = env.Time + ready.Template.GlobalCD
			} else {
				leader.nextAtk = env.Time
			}

			emit(Event{T: env.Time, Type: "Cast", Payload: map[string]any{
				"caster": leader.ID, "skill": ready.Template.ID, "x": leader.Pos.X, "y": leader.Pos.Y,
			}})

			for _, ap := range ready.Template.Applies {
				if ap.ID == "" || ap.Duration <= 0 {
					continue
				}
				boss.Statuses[ap.ID] = Status{Name: ap.ID, ExpireAt: env.Time + ap.Duration}
				emit(Event{T: env.Time, Type: "ApplyStatus", Payload: map[string]any{
					"target": boss.ID, "status": ap.ID, "dur": ap.Duration,
				}})
			}

			rr.TryTrigger(ready.Template.Elem, boss, active.ID)

			resist := boss.Resist[ready.Template.Elem]
			dmg := ready.Template.Damage * (1.0 - resist) * boss.BuffDMGTakenMul
			if dmg < 1 {
				dmg = 1
			}
			boss.HP -= int(dmg)
			if boss.HP < 0 {
				boss.HP = 0
			}

			totalDamage += dmg
			damageBySkill[ready.Template.ID] += dmg
			damageByHero[active.ID] += dmg

			emit(Event{T: env.Time, Type: "Hit", Payload: map[string]any{
				"elem": ready.Template.Elem, "dmg": int(dmg), "hp": boss.HP, "target": boss.ID, "caster": leader.ID,
			}})

			bAI.OnHit(env, boss, leader, emit)
		}

		bAI.Update(env, boss, party.Active().Entity, emit)

		for name, st := range boss.Statuses {
			if env.Time >= st.ExpireAt {
				delete(boss.Statuses, name)
			}
		}

		ph.Tick(env.Time)

		if boss.HP <= 0 || party.Active().Entity.HP <= 0 {
			break
		}
	}

	for hid, extra := range reactByHero {
		damageByHero[hid] += extra
	}

	res := SimResult{
		Win:           boss.HP <= 0,
		Duration:      env.Time,
		DPS:           totalDamage / (env.Time + 1e-6),
		Reactions:     reacHist,
		DamageBySkill: damageBySkill,
		DamageByHero:  damageByHero,
	}
	if record {
		res.Events = events
	}
	return res
}

func MarshalPretty(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}
