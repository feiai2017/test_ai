package combat

import (
	"fmt"
	"math"
	"math/rand"

	"test_ai/internal/config"
)

type bossStage int

const (
	stageIdle bossStage = iota
	stageTelegraph
	stageCasting
	stageRecover
	stageWeaken
)

type intentState struct {
	Intent   *config.BossIntent
	Stage    bossStage
	StageEnd float64
	Started  float64
}

type climaxScheduler struct {
	cfg  config.ClimaxConfig
	next float64
	on   bool
}

func (c *climaxScheduler) Configure(cfg config.ClimaxConfig, now float64, rng *rand.Rand) {
	c.cfg = cfg
	if cfg.Intent == "" || cfg.IntervalMax <= 0 {
		c.on = false
		return
	}
	c.on = true
	c.next = now + rollInterval(cfg, rng)
}

func (c *climaxScheduler) Due(now float64) bool {
	return c.on && now >= c.next
}

func (c *climaxScheduler) Consume(now float64, rng *rand.Rand) {
	if !c.on {
		return
	}
	c.next = now + rollInterval(c.cfg, rng)
}

func rollInterval(cfg config.ClimaxConfig, rng *rand.Rand) float64 {
	min := cfg.IntervalMin
	max := cfg.IntervalMax
	if max <= 0 {
		return min
	}
	if max < min {
		max = min
	}
	if min == max {
		return min
	}
	return min + rng.Float64()*(max-min)
}

type weightedIntent struct {
	ref    *config.BossIntent
	weight float64
}

type BossController struct {
	cfg       *config.BossConfig
	boss      *Entity
	phaser    *BossPhaser
	emit      func(Event)
	rng       *rand.Rand
	state     intentState
	hasIntent bool

	intents map[string]*config.BossIntent

	weights []weightedIntent

	mini  climaxScheduler
	major climaxScheduler

	weakened       bool
	weakenEnd      float64
	damageAmpStack float64

	lastPhase int
	bossName  string
	heroNames map[string]string
}

func NewBossController(cfg *config.BossConfig, boss *Entity, ph *BossPhaser, emit func(Event), rng *rand.Rand) *BossController {
	if cfg == nil {
		cfg = &config.BossConfig{}
	}
	intents := map[string]*config.BossIntent{}
	for i := range cfg.Intents {
		it := cfg.Intents[i]
		copy := it
		intents[it.ID] = &copy
	}
	bc := &BossController{
		cfg:       cfg,
		boss:      boss,
		phaser:    ph,
		emit:      emit,
		rng:       rng,
		intents:   intents,
		lastPhase: ph.CurrentPhase(),
		bossName:  boss.ID,
		heroNames: map[string]string{},
	}
	bc.resetPhaseState(0, 0)
	return bc
}

func (bc *BossController) SetHeroNames(names map[string]string) {
	if names == nil {
		bc.heroNames = map[string]string{}
		return
	}
	bc.heroNames = names
}

func (bc *BossController) SetBossName(name string) {
	if name != "" {
		bc.bossName = name
	}
}

func (bc *BossController) heroName(id string) string {
	if bc.heroNames != nil {
		if n, ok := bc.heroNames[id]; ok && n != "" {
			return n
		}
	}
	return id
}

func (bc *BossController) intentName(it *config.BossIntent) string {
	if it == nil {
		return "?"
	}
	if it.Name != "" {
		return it.Name
	}
	return it.ID
}

func (bc *BossController) emitLog(t float64, source, id, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	payload := map[string]any{"text": text}
	if source != "" {
		payload["source"] = source
	}
	if id != "" {
		payload["id"] = id
	}
	bc.emit(Event{T: t, Type: "LogLine", Payload: payload})
}

func (bc *BossController) resetPhaseState(phase int, now float64) {
	spec := bc.phaser.PhaseSpec()
	bc.weights = bc.weights[:0]
	if spec != nil {
		for _, pi := range spec.Intents {
			if intent := bc.intents[pi.Intent]; intent != nil {
				bc.weights = append(bc.weights, weightedIntent{ref: intent, weight: math.Max(pi.Weight, 0)})
			}
		}
		bc.mini.Configure(spec.MiniClimax, now, bc.rng)
		bc.major.Configure(spec.MajorClimax, now, bc.rng)
	}
	bc.state = intentState{}
	bc.hasIntent = false
}

func (bc *BossController) Update(env *Env, party *PartyController) {
	now := env.Time
	if bc.phaser.Tick(now) {
		bc.lastPhase = bc.phaser.CurrentPhase()
		bc.resetPhaseState(bc.lastPhase, now)
	}
	if bc.weakened {
		if now >= bc.weakenEnd {
			bc.exitWeaken(now)
		}
		return
	}
	if bc.state.Stage == stageTelegraph || bc.state.Stage == stageCasting {
		if bc.boss.Guard <= 0 {
			bc.enterWeaken(now)
			return
		}
	}
	if !bc.hasIntent {
		bc.pickNextIntent(now)
	}
	if !bc.hasIntent {
		return
	}
	if now < bc.state.StageEnd {
		return
	}
	switch bc.state.Stage {
	case stageTelegraph:
		bc.enterStage(stageCasting, now, bc.state.Intent.Cast)
	case stageCasting:
		bc.executeIntent(now, party)
		bc.enterStage(stageRecover, now, bc.state.Intent.Recovery)
	case stageRecover:
		bc.hasIntent = false
		bc.state = intentState{}
	default:
		bc.hasIntent = false
	}
}

func (bc *BossController) pickNextIntent(now float64) {
	var intent *config.BossIntent
	if bc.major.Due(now) {
		intent = bc.intents[bc.major.cfg.Intent]
		bc.major.Consume(now, bc.rng)
	}
	if intent == nil && bc.mini.Due(now) {
		intent = bc.intents[bc.mini.cfg.Intent]
		bc.mini.Consume(now, bc.rng)
	}
	if intent == nil {
		total := 0.0
		for _, w := range bc.weights {
			total += w.weight
		}
		if total > 0 {
			pick := bc.rng.Float64() * total
			acc := 0.0
			for _, w := range bc.weights {
				acc += w.weight
				if pick <= acc {
					intent = w.ref
					break
				}
			}
		}
	}
	if intent == nil {
		return
	}
	if intent.Telegraph <= 0 {
		intent.Telegraph = 0.1
	}
	bc.state = intentState{
		Intent:   intent,
		Stage:    stageTelegraph,
		StageEnd: now + intent.Telegraph,
		Started:  now,
	}
	bc.hasIntent = true
	bc.emit(Event{T: now, Type: "BossIntentTelegraph", Payload: map[string]any{
		"intent": intent.ID,
		"name":   intent.Name,
		"phase":  bc.lastPhase,
		"note":   intent.Note,
	}})
}

func (bc *BossController) enterStage(stage bossStage, now float64, duration float64) {
	if duration <= 0 {
		duration = 0.05
	}
	bc.state.Stage = stage
	bc.state.StageEnd = now + duration
	switch stage {
	case stageCasting:
		bc.emit(Event{T: now, Type: "BossIntentCast", Payload: map[string]any{
			"intent": bc.state.Intent.ID,
			"name":   bc.intentName(bc.state.Intent),
		}})
	case stageRecover:
		bc.emit(Event{T: now, Type: "BossIntentRecover", Payload: map[string]any{
			"intent": bc.state.Intent.ID,
			"name":   bc.intentName(bc.state.Intent),
		}})
	}
}

func (bc *BossController) executeIntent(now float64, party *PartyController) {
	intent := bc.state.Intent
	hitAll := false
	for _, tag := range intent.Tags {
		if tag == "aoe" {
			hitAll = true
			break
		}
	}
	if hitAll {
		for i := range party.Heroes {
			h := &party.Heroes[i]
			if h.Entity == nil || h.Entity.HP <= 0 {
				continue
			}
			bc.applyDamageToHero(intent, now, h.Entity)
		}
	} else {
		target := party.Active().Entity
		if target != nil && target.HP > 0 {
			bc.moveIntoRange(intent.Range, target, now)
			bc.applyDamageToHero(intent, now, target)
		}
	}
	if intent.GuardDamage > 0 && bc.boss.Guard < bc.boss.GuardMax {
		prev := bc.boss.Guard
		bc.boss.Guard += intent.GuardDamage
		if bc.boss.Guard > bc.boss.GuardMax {
			bc.boss.Guard = bc.boss.GuardMax
		}
		bc.emit(Event{T: now, Type: "GuardChanged", Payload: map[string]any{
			"guard": bc.boss.Guard,
			"src":   intent.ID,
		}})
		bc.emitLog(now, "system", bc.boss.ID, "%s 的护甲回复 %d -> %d", bc.bossName, prev, bc.boss.Guard)
	}
}

func (bc *BossController) applyDamageToHero(intent *config.BossIntent, now float64, hero *Entity) {
	diff := hero.Pos.Sub(bc.boss.Pos)
	dist := diff.Len()
	if intent.Range > 0 && dist > intent.Range {
		return
	}
	res := hero.Resist[intent.Elem]
	dmg := intent.Damage * (1.0 - res)
	if dmg < 1 {
		dmg = 1
	}
	hero.HP -= int(dmg)
	if hero.HP < 0 {
		hero.HP = 0
	}
	bc.emit(Event{T: now, Type: "BossHit", Payload: map[string]any{
		"intent": intent.ID,
		"elem":   intent.Elem,
		"dmg":    int(dmg),
		"target": hero.ID,
		"hp":     hero.HP,
	}})
	bc.emitLog(now, "boss", bc.boss.ID, "%s 的「%s」命中 %s，造成 %.0f 伤害 (剩余 %d)", bc.bossName, bc.intentName(intent), bc.heroName(hero.ID), dmg, hero.HP)
}

func (bc *BossController) moveIntoRange(rng float64, target *Entity, now float64) {
	if rng <= 0 {
		return
	}
	diff := bc.boss.Pos.Sub(target.Pos)
	dist := diff.Len()
	if dist <= rng {
		return
	}
	step := bc.boss.Speed * 0.1
	if step <= 0 {
		return
	}
	if step > dist-rng {
		step = dist - rng
	}
	if step <= 0 {
		return
	}
	old := bc.boss.Pos
	dir := diff.Norm()
	bc.boss.Pos = bc.boss.Pos.Add(dir.Scale(-step))
	bc.emit(Event{T: now, Type: "BossMove", Payload: map[string]any{
		"id":   bc.boss.ID,
		"from": []float64{old.X, old.Y},
		"to":   []float64{bc.boss.Pos.X, bc.boss.Pos.Y},
	}})
}

func (bc *BossController) ApplyGuardDamage(amount float64, now float64, source string) {
	if amount <= 0 {
		return
	}
	bc.boss.Guard -= int(math.Round(amount))
	if bc.boss.Guard < 0 {
		bc.boss.Guard = 0
	}
	bc.emit(Event{T: now, Type: "GuardChanged", Payload: map[string]any{
		"guard":  bc.boss.Guard,
		"source": source,
	}})
	bc.emitLog(now, "system", bc.boss.ID, "%s 的护甲被 %s 削减 -> %d", bc.bossName, source, bc.boss.Guard)
	if bc.boss.Guard <= 0 && !bc.weakened {
		bc.enterWeaken(now)
	}
}

func (bc *BossController) enterWeaken(now float64) {
	if bc.weakened {
		return
	}
	bc.weakened = true
	bc.hasIntent = false
	bc.state = intentState{}
	cfg := bc.cfg.Weaken
	bc.weakenEnd = now + cfg.Duration
	bc.damageAmpStack = cfg.DamageAmp
	bc.boss.BuffDMGTakenMul *= 1.0 + cfg.DamageAmp
	bc.emit(Event{T: now, Type: "BossWeakenStart", Payload: map[string]any{
		"duration": cfg.Duration,
		"guard":    bc.boss.Guard,
	}})
}

func (bc *BossController) exitWeaken(now float64) {
	cfg := bc.cfg.Weaken
	bc.weakened = false
	if bc.damageAmpStack != 0 {
		bc.boss.BuffDMGTakenMul /= 1.0 + bc.damageAmpStack
	}
	if cfg.GuardReturn > 0 {
		bc.boss.Guard = cfg.GuardReturn
		if bc.boss.Guard > bc.boss.GuardMax {
			bc.boss.Guard = bc.boss.GuardMax
		}
		bc.emit(Event{T: now, Type: "GuardChanged", Payload: map[string]any{
			"guard": bc.boss.Guard,
			"src":   "weaken_recover",
		}})
	}
	bc.emit(Event{T: now, Type: "BossWeakenEnd", Payload: map[string]any{}})
	bc.emitLog(now, "system", bc.boss.ID, "%s 结束虚弱", bc.bossName)
}
