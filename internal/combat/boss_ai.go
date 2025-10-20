package combat

// BossAI：追击 + 普攻 + 受击反击
type BossAI struct {
	NextAtkAt     float64
	NextRetaliate float64
	Range         float64
	AtkCD         float64
	RetaliateCD   float64
	Damage        float64
	Elem          string
}

func NewBossAI() *BossAI {
	return &BossAI{
		Range: 3.0, AtkCD: 1.4, RetaliateCD: 3.0,
		Damage: 85, Elem: "physical",
	}
}

func (ai *BossAI) Update(env *Env, boss *Entity, target *Entity, emit func(Event)) {
	// 追击
	diff := boss.Pos.Sub(target.Pos)
	dist := diff.Len()
	if dist > ai.Range {
		step := boss.Speed * env.Delta
		if step > dist {
			step = dist
		}
		if step > 0 {
			old := boss.Pos
			boss.Pos = boss.Pos.Add(diff.Norm().Scale(-step))
			emit(Event{T: env.Time, Type: "Move", Payload: map[string]any{
				"id": boss.ID, "from": []float64{old.X, old.Y}, "to": []float64{boss.Pos.X, boss.Pos.Y},
			}})
		}
		return
	}
	// 攻击
	if env.Time >= ai.NextAtkAt {
		ai.NextAtkAt = env.Time + ai.AtkCD
		emit(Event{T: env.Time, Type: "Cast", Payload: map[string]any{
			"caster": boss.ID, "skill": "boss.claw", "x": boss.Pos.X, "y": boss.Pos.Y,
		}})
		applyDamageToHero(env, ai, boss, target, emit)
	}
}

func (ai *BossAI) OnHit(env *Env, boss *Entity, src *Entity, emit func(Event)) {
	if env.Time < ai.NextRetaliate {
		return
	}
	ai.NextRetaliate = env.Time + ai.RetaliateCD
	emit(Event{T: env.Time, Type: "Cast", Payload: map[string]any{
		"caster": boss.ID, "skill": "boss.retaliate", "x": boss.Pos.X, "y": boss.Pos.Y,
	}})
	dmg := 55.0 * src.BuffDMGTakenMul
	src.HP -= int(dmg)
	if src.HP < 0 {
		src.HP = 0
	}
	emit(Event{T: env.Time, Type: "Hit", Payload: map[string]any{
		"elem": "physical", "dmg": int(dmg), "target": src.ID, "hp": src.HP, "caster": boss.ID,
	}})
	emit(Event{T: env.Time, Type: "ApplyStatus", Payload: map[string]any{
		"target": src.ID, "status": "slow", "dur": 1.0,
	}})
}

func applyDamageToHero(env *Env, ai *BossAI, boss *Entity, hero *Entity, emit func(Event)) {
	res := hero.Resist[ai.Elem]
	dmg := ai.Damage * (1.0 - res)
	if dmg < 1 {
		dmg = 1
	}
	hero.HP -= int(dmg)
	if hero.HP < 0 {
		hero.HP = 0
	}
	emit(Event{T: env.Time, Type: "Hit", Payload: map[string]any{
		"elem": ai.Elem, "dmg": int(dmg), "target": hero.ID, "hp": hero.HP, "caster": boss.ID,
	}})
}
