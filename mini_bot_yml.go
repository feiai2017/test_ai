package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

/***************
 * CLI
 ***************/
var (
	flagSim    bool
	flagN      int
	flagSeed   int64
	flagConfig string
	flagOut    string // 输出 JSON：{init, events}
)

func init() {
	flag.BoolVar(&flagSim, "simsvc", false, "run simulation batch instead of single demo")
	flag.IntVar(&flagN, "n", 1000, "number of battles to simulate")
	flag.Int64Var(&flagSeed, "seed", 0, "random seed (0 = now)")
	flag.StringVar(&flagConfig, "config", "", "YAML config file for planner mode")
	flag.StringVar(&flagOut, "outjson", "", "write battle result JSON to file (viewer input); empty prints to stdout")
}

/***************
 * 基本类型/工具
 ***************/
type Pos struct{ R, C int }

func manhattan(a, b Pos) int {
	dr := a.R - b.R
	if dr < 0 {
		dr = -dr
	}
	dc := a.C - b.C
	if dc < 0 {
		dc = -dc
	}
	return dr + dc
}
func inBounds(p Pos, h, w int) bool { return p.R >= 0 && p.R < h && p.C >= 0 && p.C < w }
func divCeil(a, b int) int          { return (a + b - 1) / b }

/***************
 * YAML 载入（策划模式）
 ***************/
type YAMLBuff struct {
	Name          string  `yaml:"name"`
	ATKMul        float64 `yaml:"atk_mul"`
	DMGTakenMul   float64 `yaml:"dmg_taken_mul"`
	SPDMul        float64 `yaml:"spd_mul"`
	DurationTicks int     `yaml:"duration_ticks"`
}

type YAMLSkill struct {
	Name           string    `yaml:"name"`
	Kind           string    `yaml:"kind"` // damage | heal_self
	Damage         int       `yaml:"damage"`
	Heal           int       `yaml:"heal"`
	Range          int       `yaml:"range"`
	CDTicks        int       `yaml:"cd_ticks"`
	CDActions      int       `yaml:"cd_actions"`
	SplashPct      float64   `yaml:"splash_pct"`
	ApplyBuffOnHit *YAMLBuff `yaml:"apply_buff_on_hit"`
}

type YAMLPersonality struct {
	Aggression float64 `yaml:"aggression"`
	Cautious   float64 `yaml:"cautious"`
}

type YAMLUnit struct {
	Name        string          `yaml:"name"`
	Team        string          `yaml:"team"`
	MaxHP       int             `yaml:"max_hp"`
	HP          int             `yaml:"hp"`
	BaseATK     int             `yaml:"base_atk"`
	Range       int             `yaml:"range"`
	Pos         [2]int          `yaml:"pos"`
	SPD         int             `yaml:"spd"`
	CritChance  float64         `yaml:"crit_chance"`
	CritMult    float64         `yaml:"crit_mult"`
	Personality YAMLPersonality `yaml:"personality"`
	Skill       *YAMLSkill      `yaml:"skill"`
}

type YAMLGrid struct {
	H int `yaml:"h"`
	W int `yaml:"w"`
}

type YAMLRules struct {
	LowHPPercent  int `yaml:"low_hp_percent"`
	TickPerAction int `yaml:"tick_per_action"`
}

type YAMLConfig struct {
	Grid  YAMLGrid   `yaml:"grid"`
	Rules YAMLRules  `yaml:"rules"`
	Units []YAMLUnit `yaml:"units"`
}

func loadYAML(path string) (*YAMLConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg YAMLConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if cfg.Grid.H == 0 {
		cfg.Grid.H = 3
	}
	if cfg.Grid.W == 0 {
		cfg.Grid.W = 3
	}
	if cfg.Rules.LowHPPercent == 0 {
		cfg.Rules.LowHPPercent = 30
	}
	if cfg.Rules.TickPerAction == 0 {
		cfg.Rules.TickPerAction = 30
	}
	for i := range cfg.Units {
		if cfg.Units[i].HP == 0 {
			cfg.Units[i].HP = cfg.Units[i].MaxHP
		}
		if cfg.Units[i].CritMult == 0 {
			cfg.Units[i].CritMult = 1.5
		}
	}
	return &cfg, nil
}

/***************
 * 战斗模型
 ***************/
type Buff struct {
	Name       string
	ATKMul     float64
	DMGTaken   float64
	SPDMul     float64
	Duration   int
	SourceUnit string
}

func (b *Buff) tick(dt int) { b.Duration -= dt }

type SkillType int

const (
	SkillDamage SkillType = iota
	SkillHealSelf
)

type Skill struct {
	Name           string
	Kind           SkillType
	Damage         int
	Heal           int
	Range          int
	CD             int
	CDLeft         int
	SplashPct      float64
	ApplyBuffOnHit *Buff
}

type Personality struct {
	Aggression float64
	Cautious   float64
}

type Unit struct {
	Name    string
	Team    string
	MaxHP   int
	HP      int
	BaseATK int
	Range   int
	Pos     Pos
	Alive   bool

	SPD int
	CT  int

	Skill       *Skill
	CritChance  float64
	CritMult    float64
	Personality Personality
	Buffs       []Buff
}

func (u *Unit) currentATK() int {
	m := 1.0
	for _, b := range u.Buffs {
		m *= (1 + b.ATKMul)
	}
	atk := int(math.Round(float64(u.BaseATK) * m))
	if atk < 1 {
		atk = 1
	}
	return atk
}
func (u *Unit) currentSPD() int {
	m := 1.0
	for _, b := range u.Buffs {
		m *= (1 + b.SPDMul)
	}
	spd := int(math.Round(float64(u.SPD) * m))
	if spd < 1 {
		spd = 1
	}
	return spd
}
func (u *Unit) dmgTakenMultiplier() float64 {
	m := 1.0
	for _, b := range u.Buffs {
		m *= (1 + b.DMGTaken)
	}
	return m
}

/***************
 * 事件日志（导出 JSON 用）
 ***************/
type Event struct {
	TimeTick    int    `json:"timeTick"`
	Actor       string `json:"actor"`
	Action      string `json:"action"`
	Target      string `json:"target,omitempty"`
	Damage      int    `json:"damage,omitempty"`
	Crit        bool   `json:"crit,omitempty"`
	Heal        int    `json:"heal,omitempty"`
	FromSkill   string `json:"fromSkill,omitempty"`
	Note        string `json:"note,omitempty"`
	BuffApplied string `json:"buffApplied,omitempty"`
	BuffExpire  string `json:"buffExpire,omitempty"`
	NewPos      *Pos   `json:"newPos,omitempty"`
}
type InitUnit struct {
	Name  string `json:"name"`
	Team  string `json:"team"`
	MaxHP int    `json:"maxHP"`
	HP    int    `json:"hp"`
	Row   int    `json:"row"`
	Col   int    `json:"col"`
}
type InitState struct {
	GridH int        `json:"gridH"`
	GridW int        `json:"gridW"`
	Units []InitUnit `json:"units"`
}

/***************
 * 战场
 ***************/
type Battle struct {
	H, W       int
	Units      []*Unit
	LowHPPct   int
	TickPerAct int

	TimeTick int
	Actions  int
	LogOn    bool
	Events   []Event
	Init     InitState
}

func (b *Battle) pushEvent(e Event) { b.Events = append(b.Events, e) }
func (b *Battle) logf(f string, a ...any) {
	if b.LogOn {
		fmt.Printf(f+"\n", a...)
	}
}

func (b *Battle) enemiesOf(u *Unit) []*Unit {
	var out []*Unit
	for _, x := range b.Units {
		if x.Alive && x.Team != u.Team {
			out = append(out, x)
		}
	}
	return out
}
func (b *Battle) occupied() map[Pos]bool {
	m := make(map[Pos]bool)
	for _, x := range b.Units {
		if x.Alive {
			m[x.Pos] = true
		}
	}
	return m
}
func (b *Battle) nearestEnemy(u *Unit) *Unit {
	es := b.enemiesOf(u)
	if len(es) == 0 {
		return nil
	}
	sort.Slice(es, func(i, j int) bool {
		di := manhattan(u.Pos, es[i].Pos)
		dj := manhattan(u.Pos, es[j].Pos)
		if di != dj {
			return di < dj
		}
		if es[i].HP != es[j].HP {
			return es[i].HP < es[j].HP
		}
		return es[i].Name < es[j].Name
	})
	return es[0]
}
func (b *Battle) enemiesInRange(u *Unit, rng int) []*Unit {
	var es []*Unit
	for _, e := range b.enemiesOf(u) {
		if manhattan(u.Pos, e.Pos) <= rng {
			es = append(es, e)
		}
	}
	return es
}

/***************
 * 移动
 ***************/
func (b *Battle) tryMoveTowards(u *Unit, target Pos) bool {
	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
	occ := b.occupied()
	delete(occ, u.Pos)
	now := manhattan(u.Pos, target)
	best := []Pos{}
	bestDist := math.MaxInt32
	for _, d := range dirs {
		np := Pos{u.Pos.R + d.R, u.Pos.C + d.C}
		if !inBounds(np, b.H, b.W) || occ[np] {
			continue
		}
		nd := manhattan(np, target)
		if nd < now {
			if nd < bestDist {
				bestDist = nd
				best = []Pos{np}
			} else if nd == bestDist {
				best = append(best, np)
			}
		}
	}
	if len(best) == 0 {
		return false
	}
	sort.Slice(best, func(i, j int) bool {
		if best[i].R != best[j].R {
			return best[i].R < best[j].R
		}
		return best[i].C < best[j].C
	})
	u.Pos = best[0]
	b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "move", NewPos: &u.Pos})
	return true
}
func (b *Battle) tryMoveAway(u *Unit) bool {
	enemy := b.nearestEnemy(u)
	if enemy == nil {
		return false
	}
	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
	occ := b.occupied()
	delete(occ, u.Pos)
	now := manhattan(u.Pos, enemy.Pos)
	best := []Pos{}
	bestDist := -1
	for _, d := range dirs {
		np := Pos{u.Pos.R + d.R, u.Pos.C + d.C}
		if !inBounds(np, b.H, b.W) || occ[np] {
			continue
		}
		nd := manhattan(np, enemy.Pos)
		if nd > now {
			if nd > bestDist {
				bestDist = nd
				best = []Pos{np}
			} else if nd == bestDist {
				best = append(best, np)
			}
		}
	}
	if len(best) == 0 {
		return false
	}
	sort.Slice(best, func(i, j int) bool {
		if best[i].R != best[j].R {
			return best[i].R < best[j].R
		}
		return best[i].C < best[j].C
	})
	u.Pos = best[0]
	b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "retreat", NewPos: &u.Pos})
	return true
}

/***************
 * 评分（Utility）
 ***************/
type ActionKind int

const (
	ActNone ActionKind = iota
	ActCastSkill
	ActAttack
	ActMoveCloser
)

type Decision struct {
	Kind   ActionKind
	Target *Unit
	StepTo *Pos
	Score  float64
}

func better(a, c Decision) Decision {
	if c.Score > a.Score {
		return c
	}
	return a
}
func isLowHP(u *Unit, pct int) int {
	if u.HP*100 <= u.MaxHP*pct {
		return 1
	}

	return 0
}

func scoreCastSkillDamage(u, t *Unit) float64 {
	dmg := float64(u.Skill.Damage)
	if t != nil {
		dmg *= t.dmgTakenMultiplier()
	}
	kill := 0.0
	if t != nil && float64(t.HP) <= dmg {
		kill = 1
	}
	dist := 0.0
	if t != nil {
		dist = float64(manhattan(u.Pos, t.Pos))
	}
	prox := 1.0 / (1.0 + dist)
	agg, cau := u.Personality.Aggression, u.Personality.Cautious
	return (100*kill+10*dmg+2*prox)*(1+0.15*agg) - 10*cau*float64(isLowHP(u, 30))
}
func scoreAttack(u, t *Unit) float64 {
	dmg := float64(u.currentATK())
	if t != nil {
		dmg *= t.dmgTakenMultiplier()
	}
	kill := 0.0
	if t != nil && float64(t.HP) <= dmg {
		kill = 1
	}
	dist := 0.0
	if t != nil {
		dist = float64(manhattan(u.Pos, t.Pos))
	}
	prox := 1.0 / (1.0 + dist)
	agg, cau := u.Personality.Aggression, u.Personality.Cautious
	return (80*kill+5*dmg+1.5*prox)*(1+0.10*agg) - 8*cau*float64(isLowHP(u, 30))
}
func scoreMoveCloser(u *Unit, np Pos, enemy *Unit) float64 {
	distAfter := float64(manhattan(np, enemy.Pos))
	prox := 1.0 / (1.0 + distAfter)
	bonus := 0.0
	if manhattan(np, enemy.Pos) <= u.Range {
		bonus += 5
	}
	if u.Skill != nil && u.Skill.CDLeft <= 0 && u.Skill.Kind == SkillDamage && manhattan(np, enemy.Pos) <= u.Skill.Range {
		bonus += 8
	}
	agg, cau := u.Personality.Aggression, u.Personality.Cautious
	return (2*prox+bonus)*(1+0.05*agg) - 5*cau*float64(isLowHP(u, 30))
}

func (b *Battle) decide(u *Unit) Decision {
	var best Decision
	if u.Skill != nil && u.Skill.CDLeft <= 0 && u.Skill.Kind == SkillDamage {
		es := b.enemiesInRange(u, u.Skill.Range)
		for _, t := range es {
			best = better(best, Decision{Kind: ActCastSkill, Target: t, Score: scoreCastSkillDamage(u, t)})
		}
	}
	ts := b.enemiesInRange(u, u.Range)
	for _, t := range ts {
		best = better(best, Decision{Kind: ActAttack, Target: t, Score: scoreAttack(u, t)})
	}
	if e := b.nearestEnemy(u); e != nil {
		if np, ok := b.bestStepCloser(u, e.Pos); ok {
			best = better(best, Decision{Kind: ActMoveCloser, StepTo: &np, Score: scoreMoveCloser(u, np, e)})
		}
	}
	if best.Kind == ActNone {
		return Decision{Kind: ActNone, Score: -1}
	}
	return best
}
func (b *Battle) bestStepCloser(u *Unit, target Pos) (Pos, bool) {
	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
	occ := b.occupied()
	delete(occ, u.Pos)
	now := manhattan(u.Pos, target)
	has := false
	bestDist := 1 << 30
	best := Pos{}
	for _, d := range dirs {
		np := Pos{u.Pos.R + d.R, u.Pos.C + d.C}
		if !inBounds(np, b.H, b.W) || occ[np] {
			continue
		}
		nd := manhattan(np, target)
		if nd < now {
			if !has || nd < bestDist || (nd == bestDist && (np.R < best.R || (np.R == best.R && np.C < best.C))) {
				has, bestDist, best = true, nd, np
			}
		}
	}
	return best, has
}

/***************
 * 行为树硬约束
 ***************/
type BTStatus int

const (
	BTSuccess BTStatus = iota
	BTFailure
	BTRunning
)

type BTNode interface{ Tick(*Blackboard) BTStatus }
type Blackboard struct {
	B   *Battle
	U   *Unit
	Log func(string, ...any)
}
type Selector struct{ Children []BTNode }

func (s *Selector) Tick(bb *Blackboard) BTStatus {
	for _, ch := range s.Children {
		if ch.Tick(bb) == BTSuccess {
			return BTSuccess
		}
	}
	return BTFailure
}

type Sequence struct{ Children []BTNode }

func (s *Sequence) Tick(bb *Blackboard) BTStatus {
	for _, ch := range s.Children {
		if ch.Tick(bb) != BTSuccess {
			return BTFailure
		}
	}
	return BTSuccess
}

type Condition func(*Blackboard) bool
type CondNode struct{ Fn Condition }

func (c *CondNode) Tick(bb *Blackboard) BTStatus {
	if c.Fn(bb) {
		return BTSuccess
	}
	return BTFailure
}

type Action func(*Blackboard) bool
type ActionNode struct{ Fn Action }

func (a *ActionNode) Tick(bb *Blackboard) BTStatus {
	if a.Fn(bb) {
		return BTSuccess
	}
	return BTFailure
}

func (b *Battle) condLowHP(bb *Blackboard) bool { return isLowHP(bb.U, b.LowHPPct) == 1 }
func hasHealReady(bb *Blackboard) bool {
	u := bb.U
	return u.Skill != nil && u.Skill.Kind == SkillHealSelf && u.Skill.CDLeft <= 0
}
func doHeal(bb *Blackboard) bool {
	u := bb.U
	if u.Skill == nil || u.Skill.Kind != SkillHealSelf || u.Skill.CDLeft > 0 {
		return false
	}
	old := u.HP
	heal := u.Skill.Heal
	u.HP += heal
	if u.HP > u.MaxHP {
		u.HP = u.MaxHP
	}
	u.Skill.CDLeft = u.Skill.CD
	bb.B.pushEvent(Event{TimeTick: bb.B.TimeTick, Actor: u.Name, Action: "heal", Heal: heal, FromSkill: u.Skill.Name})
	bb.Log(" - %s heals %d (%d->%d)", u.Name, heal, old, u.HP)
	return true
}
func canRetreat(bb *Blackboard) bool {
	u := bb.U
	enemy := bb.B.nearestEnemy(u)
	if enemy == nil {
		return false
	}
	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
	occ := bb.B.occupied()
	delete(occ, u.Pos)
	now := manhattan(u.Pos, enemy.Pos)
	for _, d := range dirs {
		np := Pos{u.Pos.R + d.R, u.Pos.C + d.C}
		if inBounds(np, bb.B.H, bb.B.W) && !occ[np] && manhattan(np, enemy.Pos) > now {
			return true
		}
	}
	return false
}
func doRetreat(bb *Blackboard) bool {
	u := bb.U
	if bb.B.tryMoveAway(u) {
		bb.Log(" - %s retreats to (%d,%d)", u.Name, u.Pos.R, u.Pos.C)
		return true
	}
	bb.Log(" - %s blocked", u.Name)
	return false
}

func (b *Battle) doCastDamage(u *Unit, t *Unit) {
	if u.Skill == nil || u.Skill.CDLeft > 0 || u.Skill.Kind != SkillDamage {
		return
	}
	b.logf(" - %s casts %s on %s", u.Name, u.Skill.Name, t.Name)
	b.applyDamage(u, t, u.Skill.Damage, u.Skill.Name)
	// 溅射
	if u.Skill.SplashPct > 0 {
		for _, e := range b.enemiesOf(u) {
			if e == t || !e.Alive {
				continue
			}
			if manhattan(e.Pos, t.Pos) == 1 {
				b.applyDamage(u, e, int(math.Round(float64(u.Skill.Damage)*u.Skill.SplashPct)), u.Skill.Name+"(splash)")
			}
		}
	}
	// Debuff
	if u.Skill.ApplyBuffOnHit != nil && t.Alive {
		cp := *u.Skill.ApplyBuffOnHit
		cp.SourceUnit = u.Name
		t.Buffs = append(t.Buffs, cp)
		b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "applyBuff", Target: t.Name, BuffApplied: cp.Name, FromSkill: u.Skill.Name})
		b.logf("   > %s applied [%s] to %s", u.Name, cp.Name, t.Name)
	}
	u.Skill.CDLeft = u.Skill.CD
}
func (b *Battle) doAttack(u *Unit, t *Unit) {
	b.logf(" - %s attacks %s", u.Name, t.Name)
	b.applyDamage(u, t, u.currentATK(), "Attack")
}
func (b *Battle) applyDamage(src, tgt *Unit, base int, fromSkill string) {
	// 暴击
	dmg := float64(base)
	crit := false
	if rand.Float64() < src.CritChance {
		dmg *= src.CritMult
		crit = true
	}
	dmg *= tgt.dmgTakenMultiplier()
	final := int(math.Round(dmg))
	if final < 1 {
		final = 1
	}
	old := tgt.HP
	tgt.HP -= final
	b.pushEvent(Event{TimeTick: b.TimeTick, Actor: src.Name, Action: "hit", Target: tgt.Name, Damage: final, Crit: crit, FromSkill: fromSkill})
	if tgt.HP <= 0 && tgt.Alive {
		tgt.Alive = false
		b.logf("   > %s is defeated", tgt.Name)
		b.pushEvent(Event{TimeTick: b.TimeTick, Actor: src.Name, Action: "kill", Target: tgt.Name, FromSkill: fromSkill})
	} else {
		b.logf("   > %s hits %s for %d (%d->%d)%s", src.Name, tgt.Name, final, old, tgt.HP, map[bool]string{true: " CRIT", false: ""}[crit])
	}
}

func (b *Battle) doUtilityDecision(u *Unit) {
	dec := b.decide(u)
	switch dec.Kind {
	case ActCastSkill:
		if dec.Target != nil && dec.Target.Alive {
			b.doCastDamage(u, dec.Target)
		}
	case ActAttack:
		if dec.Target != nil && dec.Target.Alive {
			b.doAttack(u, dec.Target)
		}
	case ActMoveCloser:
		if dec.StepTo != nil {
			u.Pos = *dec.StepTo
			b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "move", NewPos: &u.Pos})
			b.logf(" - %s moves to (%d,%d)", u.Name, u.Pos.R, u.Pos.C)
		}
	default:
		b.logf(" - %s waits", u.Name)
	}
}

func (b *Battle) stepUnitBT(u *Unit) {
	if !u.Alive {
		return
	}
	bb := &Blackboard{B: b, U: u, Log: func(f string, a ...any) { b.logf(f, a...) }}
	root := &Selector{Children: []BTNode{
		&Sequence{Children: []BTNode{
			&CondNode{Fn: b.condLowHP},
			&Selector{Children: []BTNode{
				&Sequence{Children: []BTNode{&CondNode{Fn: hasHealReady}, &ActionNode{Fn: doHeal}}},
				&Sequence{Children: []BTNode{&CondNode{Fn: canRetreat}, &ActionNode{Fn: doRetreat}}},
			}},
		}},
		&ActionNode{Fn: func(bb *Blackboard) bool { bb.B.doUtilityDecision(bb.U); return true }},
	}}
	_ = root.Tick(bb)
}

/***************
 * 时间推进（CD+Buff按时间）
 ***************/
func (b *Battle) advanceTime(dt int) {
	if dt <= 0 {
		return
	}
	b.TimeTick += dt
	for _, u := range b.Units {
		if !u.Alive {
			continue
		}
		u.CT += dt * u.currentSPD()
		if u.Skill != nil && u.Skill.CDLeft > 0 {
			u.Skill.CDLeft -= dt
			if u.Skill.CDLeft < 0 {
				u.Skill.CDLeft = 0
			}
		}
		dst := u.Buffs[:0]
		for _, bf := range u.Buffs {
			bf.tick(dt)
			if bf.Duration > 0 {
				dst = append(dst, bf)
			} else {
				b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "buffExpire", BuffExpire: bf.Name})
			}
		}
		u.Buffs = dst
	}
}
func (b *Battle) nextActorAdvance() *Unit {
	type cand struct {
		U *Unit
		T int
	}
	var cs []cand
	for _, u := range b.Units {
		if !u.Alive || u.currentSPD() <= 0 {
			continue
		}
		if u.CT >= 100 {
			cs = append(cs, cand{U: u, T: 0})
			continue
		}
		need := 100 - u.CT
		cs = append(cs, cand{U: u, T: divCeil(need, u.currentSPD())})
	}
	if len(cs) == 0 {
		return nil
	}
	minT := math.MaxInt32
	for _, c := range cs {
		if c.T < minT {
			minT = c.T
		}
	}
	b.advanceTime(minT)
	var ready []*Unit
	for _, u := range b.Units {
		if u.Alive && u.CT >= 100 {
			ready = append(ready, u)
		}
	}
	if len(ready) == 0 {
		return nil
	}
	sort.Slice(ready, func(i, j int) bool {
		si, sj := ready[i].currentSPD(), ready[j].currentSPD()
		if si != sj {
			return si > sj
		}
		return ready[i].Name < ready[j].Name
	})
	return ready[0]
}

func (b *Battle) isOver() (bool, string) {
	aliveA, aliveB := 0, 0
	for _, u := range b.Units {
		if u.Alive {
			if u.Team == "A" {
				aliveA++
			} else {
				aliveB++
			}
		}
	}
	if aliveA == 0 && aliveB == 0 {
		return true, "Draw"
	}
	if aliveA == 0 {
		return true, "B"
	}
	if aliveB == 0 {
		return true, "A"
	}
	if b.Actions >= 600 {
		return true, "Draw"
	}
	return false, ""
}

func (b *Battle) Run() string {
	// 初始化 InitState（给可视化）
	if len(b.Init.Units) == 0 {
		b.Init.GridH, b.Init.GridW = b.H, b.W
		for _, u := range b.Units {
			b.Init.Units = append(b.Init.Units, InitUnit{
				Name: u.Name, Team: u.Team, MaxHP: u.MaxHP, HP: u.HP, Row: u.Pos.R, Col: u.Pos.C,
			})
		}
	}
	b.logf("======== BATTLE START ========")
	for _, u := range b.Units {
		b.logf(" * %s(%s) HP=%d/%d pos=(%d,%d)", u.Name, u.Team, u.HP, u.MaxHP, u.Pos.R, u.Pos.C)
	}
	for {
		if over, winner := b.isOver(); over {
			b.logf("======== BATTLE END ========")
			b.logf("Winner: %s", winner)
			return winner
		}
		actor := b.nextActorAdvance()
		if actor == nil {
			b.logf("No actor; Draw")
			return "Draw"
		}
		b.logf("---- ACTION #%d: %s (spd=%d ct=%d, t=%d) ----", b.Actions+1, actor.Name, actor.currentSPD(), actor.CT, b.TimeTick)
		b.stepUnitBT(actor)
		actor.CT -= 100
		if actor.CT < 0 {
			actor.CT = 0
		}
		for _, u := range b.Units {
			if u.Alive {
				b.logf(" * %s(%s) HP=%d/%d", u.Name, u.Team, u.HP, u.MaxHP)
			} else {
				b.logf(" * %s (dead)", u.Name)
			}
		}
		b.Actions++
	}
}

/***************
 * 构建战斗：从 YAML 或默认
 ***************/
func buildBattleFromYAML(cfg *YAMLConfig, logOn bool) *Battle {
	b := &Battle{
		H: cfg.Grid.H, W: cfg.Grid.W,
		LowHPPct:   cfg.Rules.LowHPPercent,
		TickPerAct: cfg.Rules.TickPerAction,
		LogOn:      logOn,
	}
	for _, yu := range cfg.Units {
		u := &Unit{
			Name: yu.Name, Team: yu.Team,
			MaxHP: yu.MaxHP, HP: yu.HP,
			BaseATK: yu.BaseATK, Range: yu.Range,
			Pos:   Pos{yu.Pos[0], yu.Pos[1]},
			Alive: true, SPD: yu.SPD,
			CritChance: yu.CritChance, CritMult: yu.CritMult,
			Personality: Personality{Aggression: yu.Personality.Aggression, Cautious: yu.Personality.Cautious},
		}
		if yu.Skill != nil {
			sk := &Skill{Name: yu.Skill.Name, Range: yu.Skill.Range, SplashPct: yu.Skill.SplashPct}
			switch yu.Skill.Kind {
			case "damage":
				sk.Kind = SkillDamage
				sk.Damage = yu.Skill.Damage
			case "heal_self":
				sk.Kind = SkillHealSelf
				sk.Heal = yu.Skill.Heal
			default:
				sk.Kind = SkillDamage
			}
			if yu.Skill.CDTicks > 0 {
				sk.CD = yu.Skill.CDTicks
			} else if yu.Skill.CDActions > 0 {
				sk.CD = yu.Skill.CDActions * cfg.Rules.TickPerAction
			}
			if yu.Skill.ApplyBuffOnHit != nil {
				sk.ApplyBuffOnHit = &Buff{
					Name:     yu.Skill.ApplyBuffOnHit.Name,
					ATKMul:   yu.Skill.ApplyBuffOnHit.ATKMul,
					DMGTaken: yu.Skill.ApplyBuffOnHit.DMGTakenMul,
					SPDMul:   yu.Skill.ApplyBuffOnHit.SPDMul,
					Duration: yu.Skill.ApplyBuffOnHit.DurationTicks,
				}
			}
			u.Skill = sk
		}
		b.Units = append(b.Units, u)
	}
	return b
}

func buildDefaultBattle(logOn bool) *Battle {
	cfg := &YAMLConfig{
		Grid:  YAMLGrid{H: 3, W: 3},
		Rules: YAMLRules{LowHPPercent: 30, TickPerAction: 30},
		Units: []YAMLUnit{
			{Name: "A1", Team: "A", MaxHP: 12, HP: 12, BaseATK: 4, Range: 1, Pos: [2]int{0, 0}, SPD: 8, CritChance: 0.2, CritMult: 1.5, Personality: YAMLPersonality{0.6, 0.3},
				Skill: &YAMLSkill{Name: "PowerStrike", Kind: "damage", Damage: 6, Range: 1, CDActions: 3, ApplyBuffOnHit: &YAMLBuff{Name: "Slow", SPDMul: -0.2, DurationTicks: 60}}},
			{Name: "A2", Team: "A", MaxHP: 10, HP: 10, BaseATK: 3, Range: 1, Pos: [2]int{2, 0}, SPD: 7, CritChance: 0.1, CritMult: 1.5, Personality: YAMLPersonality{0.3, 0.5},
				Skill: &YAMLSkill{Name: "SecondWind", Kind: "heal_self", Heal: 5, CDActions: 3}},
			{Name: "B1", Team: "B", MaxHP: 12, HP: 12, BaseATK: 4, Range: 1, Pos: [2]int{0, 2}, SPD: 9, CritChance: 0.15, CritMult: 1.5, Personality: YAMLPersonality{0.5, 0.2},
				Skill: &YAMLSkill{Name: "Firebolt", Kind: "damage", Damage: 5, Range: 2, CDActions: 2, SplashPct: 0.5, ApplyBuffOnHit: &YAMLBuff{Name: "Vulnerable", DMGTakenMul: 0.15, DurationTicks: 60}}},
			{Name: "B2", Team: "B", MaxHP: 10, HP: 10, BaseATK: 3, Range: 1, Pos: [2]int{2, 2}, SPD: 6, CritChance: 0.1, CritMult: 1.5, Personality: YAMLPersonality{0.4, 0.4}},
		},
	}
	return buildBattleFromYAML(cfg, logOn)
}

/***************
 * main：单场/仿真/导出JSON
 ***************/
func main() {
	flag.Parse()
	if flagSeed == 0 {
		flagSeed = time.Now().UnixNano()
	}
	rand.Seed(flagSeed)

	var ycfg *YAMLConfig
	var err error
	if flagConfig != "" {
		ycfg, err = loadYAML(flagConfig)
		if err != nil {
			fmt.Println("load YAML error:", err)
			return
		}
	}

	if !flagSim {
		// 单场并导出 JSON
		var b *Battle
		if ycfg != nil {
			b = buildBattleFromYAML(ycfg, true)
		} else {
			b = buildDefaultBattle(true)
		}
		fmt.Println("Start Time:", time.Now().Format(time.RFC3339))
		w := b.Run()
		fmt.Println("Winner:", w)

		// 打包输出（固定结构，供 battle_viewer.html 使用）
		type dump struct {
			Init   InitState `json:"init"`
			Events []Event   `json:"events"`
		}
		out := dump{Init: b.Init, Events: b.Events}
		data, _ := json.MarshalIndent(out, "", "  ")
		if flagOut != "" {
			_ = os.WriteFile(flagOut, data, 0644)
			fmt.Println("Saved JSON:", flagOut)
		} else {
			os.Stdout.Write(data)
		}
		return
	}

	// 批量仿真（不导出 JSON）
	var base *Battle
	if ycfg != nil {
		base = buildBattleFromYAML(ycfg, false)
	} else {
		base = buildDefaultBattle(false)
	}
	_ = base // 只是确保不报未使用

	var winA, winB, draw int
	for i := 0; i < flagN; i++ {
		// 每场都从 YAML 构建新战斗（不导出 JSON）
		var cfg *YAMLConfig
		if ycfg != nil {
			// 深拷贝一份
			bs, _ := json.Marshal(ycfg)
			var cp YAMLConfig
			_ = json.Unmarshal(bs, &cp)
			cfg = &cp
		} else {
			cfg = defaultYAMLClone()
		}
		// 轻微扰动 HP（±1）
		for j := range cfg.Units {
			d := rand.Intn(3) - 1
			cfg.Units[j].MaxHP += d
			if cfg.Units[j].HP > 0 {
				cfg.Units[j].HP += d
			}
			if cfg.Units[j].MaxHP < 6 {
				cfg.Units[j].MaxHP = 6
			}
			if cfg.Units[j].HP < 1 {
				cfg.Units[j].HP = cfg.Units[j].MaxHP
			}
		}
		b := buildBattleFromYAML(cfg, false)
		w := b.Run()
		switch w {
		case "A":
			winA++
		case "B":
			winB++
		default:
			draw++
		}
	}

	fmt.Println("====== Simulation Result ======")
	fmt.Printf("Seed: %d, Battles: %d\n", flagSeed, flagN)
	fmt.Printf("Team A Wins: %d (%.2f%%)\n", winA, 100.0*float64(winA)/float64(flagN))
	fmt.Printf("Team B Wins: %d (%.2f%%)\n", winB, 100.0*float64(winB)/float64(flagN))
	fmt.Printf("Draws:       %d (%.2f%%)\n", draw, 100.0*float64(draw)/float64(flagN))
}

func defaultYAMLClone() *YAMLConfig {
	cfg := &YAMLConfig{
		Grid:  YAMLGrid{H: 3, W: 3},
		Rules: YAMLRules{LowHPPercent: 30, TickPerAction: 30},
		Units: []YAMLUnit{
			{Name: "A1", Team: "A", MaxHP: 12, HP: 12, BaseATK: 4, Range: 1, Pos: [2]int{0, 0}, SPD: 8, CritChance: 0.2, CritMult: 1.5, Personality: YAMLPersonality{0.6, 0.3},
				Skill: &YAMLSkill{Name: "PowerStrike", Kind: "damage", Damage: 6, Range: 1, CDActions: 3, ApplyBuffOnHit: &YAMLBuff{Name: "Slow", SPDMul: -0.2, DurationTicks: 60}}},
			{Name: "A2", Team: "A", MaxHP: 10, HP: 10, BaseATK: 3, Range: 1, Pos: [2]int{2, 0}, SPD: 7, CritChance: 0.1, CritMult: 1.5, Personality: YAMLPersonality{0.3, 0.5},
				Skill: &YAMLSkill{Name: "SecondWind", Kind: "heal_self", Heal: 5, CDActions: 3}},
			{Name: "B1", Team: "B", MaxHP: 12, HP: 12, BaseATK: 4, Range: 1, Pos: [2]int{0, 2}, SPD: 9, CritChance: 0.15, CritMult: 1.5, Personality: YAMLPersonality{0.5, 0.2},
				Skill: &YAMLSkill{Name: "Firebolt", Kind: "damage", Damage: 5, Range: 2, CDActions: 2, SplashPct: 0.5, ApplyBuffOnHit: &YAMLBuff{Name: "Vulnerable", DMGTakenMul: 0.15, DurationTicks: 60}}},
			{Name: "B2", Team: "B", MaxHP: 10, HP: 10, BaseATK: 3, Range: 1, Pos: [2]int{2, 2}, SPD: 6, CritChance: 0.1, CritMult: 1.5, Personality: YAMLPersonality{0.4, 0.4}},
		},
	}
	return cfg
}
