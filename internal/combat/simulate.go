package combat

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"test_ai/internal/config"
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
	Meta          SimMeta            `json:"meta,omitempty"`
}

type SimMeta struct {
	Boss   SimBossMeta   `json:"boss"`
	Heroes []SimHeroMeta `json:"heroes"`
	Notes  []string      `json:"notes,omitempty"`
}

type SimBossMeta struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MaxHP    int    `json:"max_hp"`
	GuardMax int    `json:"guard_max"`
	Note     string `json:"note"`
}

type SimHeroMeta struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Element string  `json:"element"`
	MaxHP   int     `json:"max_hp"`
	Speed   float64 `json:"speed"`
	Note    string  `json:"note"`
}

type Env struct {
	Time  float64
	Delta float64
	Rng   *rand.Rand
}

func RunSingle(env *Env, boss *Entity, party *PartyController, rr *ReactionResolver, ph *BossPhaser, bossCfg *config.BossConfig, heroesCfg *config.HeroesConfig, record bool) SimResult {
	var events []Event
	emit := func(ev Event) {
		if record {
			events = append(events, ev)
		}
	}
	rr.Emit = emit

	// ---- Helpers ----
	logLine := func(ts float64, source, id, format string, args ...any) {
		if !record {
			return
		}
		text := fmt.Sprintf(format, args...)
		payload := map[string]any{"text": text}
		if source != "" {
			payload["source"] = source
		}
		if id != "" {
			payload["id"] = id
		}
		emit(Event{T: ts, Type: "LogLine", Payload: payload})
	}

	damageBySkill := map[string]float64{}
	damageByHero := map[string]float64{}
	if bossCfg == nil {
		bossCfg = &config.BossConfig{}
	}

	// ---- Boss meta/init ----
	meta := SimMeta{
		Boss: SimBossMeta{
			ID:       bossCfg.ID,
			Name:     bossCfg.ID,
			MaxHP:    bossCfg.MaxHP,
			GuardMax: bossCfg.GuardMax,
			Note:     bossCfg.Note,
		},
	}
	if meta.Boss.Name == "" {
		meta.Boss.Name = boss.ID
	}
	// Apply MaxHP/GuardMax from config to the runtime entity (authoritative)
	if meta.Boss.MaxHP > 0 {
		boss.MaxHP = meta.Boss.MaxHP
	}
	if boss.HP <= 0 || boss.HP > boss.MaxHP {
		boss.HP = boss.MaxHP
	}
	if meta.Boss.GuardMax > 0 {
		boss.GuardMax = meta.Boss.GuardMax
	}
	if boss.Guard <= 0 || boss.Guard > boss.GuardMax {
		boss.Guard = boss.GuardMax
	}
	bossDisplayName := meta.Boss.Name

	if bossCfg.Weaken.Note != "" {
		meta.Notes = append(meta.Notes, bossCfg.Weaken.Note)
	}
	for _, phSpec := range bossCfg.Phases {
		if phSpec.Note != "" {
			meta.Notes = append(meta.Notes, fmt.Sprintf("Phase @%.0f%%: %s", phSpec.Threshold*100, phSpec.Note))
		}
	}

	reacHist := map[string]int{}
	reactByHero := map[string]float64{}
	totalDamage := 0.0

	// ---- Heroes init from config ----
	defaultSpawn := func(idx int) Vec2 { return Vec2{X: 1, Y: 3 + float64(idx)*2} }

	heroSpecs := map[string]config.HeroDef{}
	if heroesCfg != nil {
		for _, hs := range heroesCfg.Heroes {
			heroSpecs[hs.ID] = hs
		}
	}
	heroNames := map[string]string{}

	for i := range party.Heroes {
		h := &party.Heroes[i]
		if len(h.Skills) == 0 {
			h.Skills = NewSkillBook(nil).Instantiate(h.ID, h.Elem)
		}
		spec, ok := heroSpecs[h.ID]
		displayName := h.ID
		if ok && spec.Name != "" {
			displayName = spec.Name
		}
		heroNames[h.ID] = displayName

		maxHP := 5000
		if ok && spec.MaxHP > 0 {
			maxHP = spec.MaxHP
		}
		speed := 3.0 + float64(i)*0.5
		if ok && spec.Speed > 0 {
			speed = spec.Speed
		}
		pos := defaultSpawn(i)
		if ok && (spec.Spawn.X != 0 || spec.Spawn.Y != 0) {
			pos = Vec2{X: spec.Spawn.X, Y: spec.Spawn.Y}
		}
		resist := map[string]float64{}
		if ok && len(spec.Resist) > 0 {
			for k, v := range spec.Resist {
				resist[k] = v
			}
		}

		h.Entity = &Entity{
			ID:              h.ID,
			HP:              maxHP,
			MaxHP:           maxHP,
			Pos:             pos,
			Speed:           speed,
			Range:           h.MaxRange(),
			AtkCD:           h.MinCooldown(),
			Resist:          resist,
			Weakness:        map[string]bool{},
			Statuses:        map[string]Status{},
			Tags:            map[string]bool{},
			BuffDMGTakenMul: 1.0,
			Guard:           0,
			GuardMax:        0,
		}

		// Meta & Spawn event with hp/max_hp
		meta.Heroes = append(meta.Heroes, SimHeroMeta{
			ID:      h.ID,
			Name:    displayName,
			Element: h.Elem,
			MaxHP:   maxHP,
			Speed:   speed,
			Note:    spec.Note,
		})

		emit(Event{T: env.Time, Type: "Spawn", Payload: map[string]any{
			"id": h.ID, "x": h.Entity.Pos.X, "y": h.Entity.Pos.Y,
			"hp": h.Entity.HP, "max_hp": h.Entity.MaxHP,
			"guard": h.Entity.Guard, "guard_max": h.Entity.GuardMax,
		}})
		logLine(env.Time, "hero", h.ID, "%s spawns: HP %d, Speed %.1f", displayName, maxHP, speed)
	}

	// Boss spawn (include hp/max_hp/guard)
	emit(Event{T: env.Time, Type: "Spawn", Payload: map[string]any{
		"id": boss.ID, "x": boss.Pos.X, "y": boss.Pos.Y, "boss": true,
		"hp": boss.HP, "max_hp": boss.MaxHP,
		"guard": boss.Guard, "guard_max": boss.GuardMax,
	}})
	logLine(env.Time, "boss", boss.ID, "%s spawns: HP %d, Guard %d", bossDisplayName, boss.MaxHP, boss.GuardMax)

	// ---- Controllers ----
	policy := &RoundRobinPolicy{Interval: 5.0, nextSwitch: 5.0}
	bossCtl := NewBossController(bossCfg, boss, ph, emit, env.Rng)
	bossCtl.SetHeroNames(heroNames)
	bossCtl.SetBossName(bossDisplayName)

	// ---- Slot assignment (triangle) ----
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

	// ---- Reaction callbacks ----
	rr.OnReactionDamage = func(reactionID string, amount float64, sourceHeroID string) {
		reactByHero[sourceHeroID] += amount
		name := heroNames[sourceHeroID]
		if name == "" {
			name = sourceHeroID
		}
		logLine(env.Time, "hero", sourceHeroID, "%s triggers reaction %s on %s for %.0f",
			name, reactionID, bossDisplayName, amount)
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
		name := heroNames[heroID]
		if name == "" {
			name = heroID
		}
		tagText := "none"
		if len(tags) > 0 {
			tagText = strings.Join(tags, "/")
		}
		logLine(env.Time, "hero", heroID, "%s cooldown adjust %.1fs, tags:%s", name, amount, tagText)
	}
	rr.OnReaction = func(id string, _ string, _ string) {
		if id != "" {
			reacHist[id]++
			logLine(env.Time, "system", "", "reaction triggered: %s", id)
		}
	}
	rr.OnGuardDamage = func(target *Entity, amount float64, source string) {
		if target == boss {
			bossCtl.ApplyGuardDamage(amount, env.Time, source)
			return
		}
		target.Guard -= int(amount)
		if target.Guard < 0 {
			target.Guard = 0
		}
		emit(Event{T: env.Time, Type: "GuardChanged", Payload: map[string]any{
			"id": target.ID, "guard": target.Guard,
		}})
		name := heroNames[target.ID]
		if name == "" {
			name = target.ID
		}
		logLine(env.Time, "system", target.ID, "%s guard reduced by %s: -%.0f -> %d",
			name, source, amount, target.Guard)
	}

	// ---- Simulation loop ----
	env.Delta = 0.1
	for env.Time = 0; env.Time < 120 && boss.HP > 0; env.Time += env.Delta {
		// party switch
		if idx, ok := policy.Next(env, party); ok && party.TrySwitchTo(idx, env.Time) {
			emit(Event{T: env.Time, Type: "Switch", Payload: map[string]any{
				"to": party.Active().ID, "index": idx,
			}})
			activeID := party.Active().ID
			logLine(env.Time, "system", activeID, "%s switched to frontline", heroNames[activeID])
			reassignSlots(idx)
		}

		active := party.Active()
		leader := active.Entity
		toBoss := boss.Pos.Sub(leader.Pos)
		distLB := toBoss.Len()
		fwd := toBoss.Norm()

		// move leader towards boss if out of range
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

		// maintain triangle formation
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

		// hero auto skill
		ready := active.ReadySkill(env.Time)
		if ready != nil && env.Time >= leader.nextAtk && leader.Pos.Sub(boss.Pos).Len() <= ready.Template.Range {
			ready.Trigger(env.Time)
			skillLabel := ready.Template.Note
			if skillLabel == "" {
				skillLabel = ready.Template.ID
			}
			if ready.Template.GlobalCD > 0 {
				leader.nextAtk = env.Time + ready.Template.GlobalCD
			} else {
				leader.nextAtk = env.Time
			}

			emit(Event{T: env.Time, Type: "Cast", Payload: map[string]any{
				"caster": leader.ID, "skill": ready.Template.ID, "x": leader.Pos.X, "y": leader.Pos.Y,
			}})
			logLine(env.Time, "hero", leader.ID, "%s casts \"%s\"", heroNames[leader.ID], skillLabel)

			// apply statuses
			for _, ap := range ready.Template.Applies {
				if ap.ID == "" || ap.Duration <= 0 {
					continue
				}
				boss.Statuses[ap.ID] = Status{Name: ap.ID, ExpireAt: env.Time + ap.Duration}
				emit(Event{T: env.Time, Type: "ApplyStatus", Payload: map[string]any{
					"target": boss.ID, "status": ap.ID, "dur": ap.Duration,
				}})
				logLine(env.Time, "hero", leader.ID, "%s applies status %s to %s", heroNames[leader.ID], ap.ID, bossDisplayName)
			}

			// reactions
			rr.TryTrigger(ready.Template.Elem, boss, active.ID)

			// damage
			resist := boss.Resist[ready.Template.Elem]
			dmg := ready.Template.Damage * (1.0 - resist) * boss.BuffDMGTakenMul
			if dmg < 1 {
				dmg = 1
			}
			boss.HP -= int(dmg)
			if boss.HP < 0 {
				boss.HP = 0
			}

			// guard
			if ready.Template.GuardBrk > 0 {
				bossCtl.ApplyGuardDamage(ready.Template.GuardBrk, env.Time, ready.Template.ID)
			}

			totalDamage += dmg
			damageBySkill[ready.Template.ID] += dmg
			damageByHero[active.ID] += dmg

			emit(Event{T: env.Time, Type: "Hit", Payload: map[string]any{
				"elem": ready.Template.Elem, "dmg": int(dmg), "hp": boss.HP, "target": boss.ID, "caster": leader.ID,
			}})
			logLine(env.Time, "hero", leader.ID, "%s hits %s with \"%s\" for %.0f (HP %d)",
				heroNames[leader.ID], bossDisplayName, skillLabel, dmg, boss.HP)
		}

		// boss update
		bossCtl.Update(env, party)

		// expire statuses
		for name, st := range boss.Statuses {
			if env.Time >= st.ExpireAt {
				delete(boss.Statuses, name)
			}
		}

		_ = ph.Tick(env.Time)

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
		Meta:          meta,
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
