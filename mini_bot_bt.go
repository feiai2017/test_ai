package main

//import (
//	"encoding/json"
//	"flag"
//	"fmt"
//	"math"
//	"math/rand"
//	"os"
//	"sort"
//	"time"
//)
//
///***************
// * 常量 & CLI
// ***************/
//const (
//	GridH       = 3
//	GridW       = 3
//	CTThreshold = 100
//	MaxActions  = 600 // 安全上限
//	TeamA       = "A"
//	TeamB       = "B"
//)
//
//var (
//	// 可调 CLI
//	flagSim           bool
//	flagN             int
//	flagSeed          int64
//	flagSaveJSON      string
//	flagLowHPPercent  int
//	flagTickPerAction int
//)
//
///***************
// * 工具函数
// ***************/
//type Pos struct{ R, C int }
//
//func inBounds(p Pos) bool { return p.R >= 0 && p.R < GridH && p.C >= 0 && p.C < GridW }
//func manhattan(a, b Pos) int {
//	dr := a.R - b.R
//	if dr < 0 {
//		dr = -dr
//	}
//	dc := a.C - b.C
//	if dc < 0 {
//		dc = -dc
//	}
//	return dr + dc
//}
//func divCeil(a, b int) int { return (a + b - 1) / b }
//
///***************
// * Buff / Debuff
// ***************/
//type Buff struct {
//	Name       string  `json:"name"`
//	ATKMul     float64 `json:"atkMul"` // +0.20 = +20% ATK
//	DMGTaken   float64 `json:"dmgTakenMul"`
//	SPDMul     float64 `json:"spdMul"`
//	Duration   int     `json:"durationTick"`
//	SourceUnit string  `json:"source,omitempty"`
//}
//
//func (b *Buff) tick(dt int) { b.Duration -= dt }
//
///***************
// * 技能系统（CD单位：tick）
// ***************/
//type SkillType int
//
//const (
//	SkillDamage SkillType = iota
//	SkillHealSelf
//)
//
//type Skill struct {
//	Name   string
//	Kind   SkillType
//	Damage int // 伤害技能
//	Heal   int // 自疗技能
//	Range  int // 施放距离
//	CD     int // CD（tick）
//	CDLeft int // 剩余CD
//
//	// v2.0 扩展：附带 Buff / Debuff 与范围溅射
//	ApplyBuffOnHit *Buff   // 命中时给对方施加的 Debuff（拷贝一份，带来源与时长）
//	SplashPct      float64 // 对目标相邻一圈敌人的溅射比例（0.5 = 50%）
//}
//
///***************
// * 人格/性格影响评分
// ***************/
//type Personality struct {
//	Aggression float64 // 越高越偏好高伤亡/击杀
//	Cautious   float64 // 越高越厌恶风险（低血时更保守）
//}
//
///***************
// * 单位 & 战局
// ***************/
//type Unit struct {
//	Name    string
//	Team    string
//	MaxHP   int
//	HP      int
//	BaseATK int
//	Range   int
//	Pos     Pos
//	Alive   bool
//
//	// 行动条
//	SPD int
//	CT  int
//
//	// 战斗属性
//	Skill       *Skill
//	CritChance  float64 // 0..1
//	CritMult    float64 // e.g. 1.5
//	Personality Personality
//	Buffs       []Buff
//}
//
//func (u *Unit) String() string {
//	return fmt.Sprintf("%s(%s) hp=%d/%d pos=(%d,%d) spd=%d ct=%d",
//		u.Name, u.Team, u.HP, u.MaxHP, u.Pos.R, u.Pos.C, u.SPD, u.CT)
//}
//
//func (u *Unit) currentATK() int {
//	mul := 1.0
//	for _, b := range u.Buffs {
//		mul *= (1.0 + b.ATKMul)
//	}
//	atk := int(math.Round(float64(u.BaseATK) * mul))
//	if atk < 1 {
//		atk = 1
//	}
//	return atk
//}
//
//func (u *Unit) currentSPD() int {
//	mul := 1.0
//	for _, b := range u.Buffs {
//		mul *= (1.0 + b.SPDMul)
//	}
//	spd := int(math.Round(float64(u.SPD) * mul))
//	if spd < 1 {
//		spd = 1
//	}
//	return spd
//}
//
//func (u *Unit) dmgTakenMultiplier() float64 {
//	m := 1.0
//	for _, b := range u.Buffs {
//		m *= (1.0 + b.DMGTaken)
//	}
//	return m
//}
//
///***************
// * 事件日志（可 JSON 导出）
// ***************/
//type Event struct {
//	TimeTick    int    `json:"timeTick"`
//	Actor       string `json:"actor"`
//	Action      string `json:"action"`
//	Target      string `json:"target,omitempty"`
//	Damage      int    `json:"damage,omitempty"`
//	Crit        bool   `json:"crit,omitempty"`
//	Heal        int    `json:"heal,omitempty"`
//	FromSkill   string `json:"fromSkill,omitempty"`
//	Note        string `json:"note,omitempty"`
//	BuffApplied string `json:"buffApplied,omitempty"`
//	BuffExpire  string `json:"buffExpire,omitempty"`
//	NewPos      *Pos   `json:"newPos,omitempty"`
//}
//
//type Battle struct {
//	Units      []*Unit
//	Actions    int
//	TimeTick   int
//	LogOn      bool
//	Events     []Event
//	TickPerAct int // tick 等价动作数（用于从“CD几次动作”换算）
//	LowHPPct   int // BT 硬约束阈值
//}
//
//func (b *Battle) logf(format string, a ...any) {
//	if b.LogOn {
//		fmt.Printf(format+"\n", a...)
//	}
//}
//
//func (b *Battle) pushEvent(e Event) {
//	b.Events = append(b.Events, e)
//}
//
///***************
// * 查询与辅助
// ***************/
//func (b *Battle) enemiesOf(u *Unit) []*Unit {
//	var out []*Unit
//	for _, x := range b.Units {
//		if x.Alive && x.Team != u.Team {
//			out = append(out, x)
//		}
//	}
//	return out
//}
//func (b *Battle) alliesOf(u *Unit) []*Unit {
//	var out []*Unit
//	for _, x := range b.Units {
//		if x.Alive && x.Team == u.Team && x != u {
//			out = append(out, x)
//		}
//	}
//	return out
//}
//func (b *Battle) occupied() map[Pos]bool {
//	m := make(map[Pos]bool)
//	for _, x := range b.Units {
//		if x.Alive {
//			m[x.Pos] = true
//		}
//	}
//	return m
//}
//func chooseTargetBy(have []*Unit, origin Pos) *Unit {
//	if len(have) == 0 {
//		return nil
//	}
//	sort.Slice(have, func(i, j int) bool {
//		if have[i].HP != have[j].HP {
//			return have[i].HP < have[j].HP
//		}
//		di := manhattan(origin, have[i].Pos)
//		dj := manhattan(origin, have[j].Pos)
//		if di != dj {
//			return di < dj
//		}
//		return have[i].Name < have[j].Name
//	})
//	return have[0]
//}
//func (b *Battle) nearestEnemy(u *Unit) *Unit {
//	es := b.enemiesOf(u)
//	if len(es) == 0 {
//		return nil
//	}
//	sort.Slice(es, func(i, j int) bool {
//		di := manhattan(u.Pos, es[i].Pos)
//		dj := manhattan(u.Pos, es[j].Pos)
//		if di != dj {
//			return di < dj
//		}
//		if es[i].HP != es[j].HP {
//			return es[i].HP < es[j].HP
//		}
//		return es[i].Name < es[j].Name
//	})
//	return es[0]
//}
//func (b *Battle) enemiesInRange(u *Unit, rng int) []*Unit {
//	var es []*Unit
//	for _, e := range b.enemiesOf(u) {
//		if manhattan(u.Pos, e.Pos) <= rng {
//			es = append(es, e)
//		}
//	}
//	return es
//}
//
///***************
// * 移动：靠近/远离
// ***************/
//func (b *Battle) tryMoveTowards(u *Unit, target Pos) bool {
//	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
//	occ := b.occupied()
//	delete(occ, u.Pos)
//	bestDist := math.MaxInt32
//	var cands []Pos
//	now := manhattan(u.Pos, target)
//	for _, d := range dirs {
//		np := Pos{u.Pos.R + d.R, u.Pos.C + d.C}
//		if !inBounds(np) || occ[np] {
//			continue
//		}
//		nd := manhattan(np, target)
//		if nd < now {
//			if nd < bestDist {
//				bestDist = nd
//				cands = []Pos{np}
//			} else if nd == bestDist {
//				cands = append(cands, np)
//			}
//		}
//	}
//	if len(cands) == 0 {
//		return false
//	}
//	sort.Slice(cands, func(i, j int) bool {
//		if cands[i].R != cands[j].R {
//			return cands[i].R < cands[j].R
//		}
//		return cands[i].C < cands[j].C
//	})
//	u.Pos = cands[0]
//	b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "move", NewPos: &u.Pos})
//	return true
//}
//func (b *Battle) tryMoveAway(u *Unit) bool {
//	enemy := b.nearestEnemy(u)
//	if enemy == nil {
//		return false
//	}
//	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
//	occ := b.occupied()
//	delete(occ, u.Pos)
//	bestDist := -1
//	var cands []Pos
//	now := manhattan(u.Pos, enemy.Pos)
//	for _, d := range dirs {
//		np := Pos{u.Pos.R + d.R, u.Pos.C + d.C}
//		if !inBounds(np) || occ[np] {
//			continue
//		}
//		nd := manhattan(np, enemy.Pos)
//		if nd > now {
//			if nd > bestDist {
//				bestDist = nd
//				cands = []Pos{np}
//			} else if nd == bestDist {
//				cands = append(cands, np)
//			}
//		}
//	}
//	if len(cands) == 0 {
//		return false
//	}
//	sort.Slice(cands, func(i, j int) bool {
//		if cands[i].R != cands[j].R {
//			return cands[i].R < cands[j].R
//		}
//		return cands[i].C < cands[j].C
//	})
//	u.Pos = cands[0]
//	b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "retreat", NewPos: &u.Pos})
//	return true
//}
//
///***************
// * 评分（Utility AI）+ 人格
// ***************/
//type ActionKind int
//
//const (
//	ActNone ActionKind = iota
//	ActCastSkill
//	ActAttack
//	ActMoveCloser
//)
//
//type Decision struct {
//	Kind   ActionKind
//	Target *Unit
//	StepTo *Pos
//	Score  float64
//}
//
//func better(a, b Decision) Decision {
//	if b.Score > a.Score {
//		return b
//	}
//	return a
//}
//
//func scoreCastSkillDamage(u, t *Unit, b *Battle) float64 {
//	// 基础项
//	dmg := float64(u.Skill.Damage)
//	kill := 0.0
//	if t.HP <= int(dmg*t.dmgTakenMultiplier()) {
//		kill = 1.0
//	}
//	dist := float64(manhattan(u.Pos, t.Pos))
//	prox := 1.0 / (1.0 + dist)
//
//	// 人格修正
//	agg := u.Personality.Aggression
//	cau := u.Personality.Cautious
//
//	return (100*kill+10*dmg+2*prox)*(1.0+0.15*agg) - 10.0*cau*float64(lowHPPenalty(u))
//}
//func scoreAttack(u, t *Unit, b *Battle) float64 {
//	dmg := float64(u.currentATK())
//	kill := 0.0
//	if t.HP <= int(dmg*t.dmgTakenMultiplier()) {
//		kill = 1.0
//	}
//	dist := float64(manhattan(u.Pos, t.Pos))
//	prox := 1.0 / (1.0 + dist)
//
//	agg := u.Personality.Aggression
//	cau := u.Personality.Cautious
//	return (80*kill+5*dmg+1.5*prox)*(1.0+0.10*agg) - 8.0*cau*float64(lowHPPenalty(u))
//}
//func scoreMoveCloser(u *Unit, np Pos, enemy *Unit, b *Battle) float64 {
//	distAfter := float64(manhattan(np, enemy.Pos))
//	prox := 1.0 / (1.0 + distAfter)
//	bonus := 0.0
//	if manhattan(np, enemy.Pos) <= u.Range {
//		bonus += 5.0
//	}
//	if u.Skill != nil && u.Skill.CDLeft <= 0 && u.Skill.Kind == SkillDamage && manhattan(np, enemy.Pos) <= u.Skill.Range {
//		bonus += 8.0
//	}
//	agg := u.Personality.Aggression
//	cau := u.Personality.Cautious
//	return (2.0*prox+bonus)*(1.0+0.05*agg) - 5.0*cau*float64(lowHPPenalty(u))
//}
//func lowHPPenalty(u *Unit) int {
//	if u.HP*100 <= u.MaxHP*30 {
//		return 1
//	}
//	return 0
//}
//
//func (b *Battle) decide(u *Unit) Decision {
//	var best Decision
//	// 技能
//	if u.Skill != nil && u.Skill.CDLeft <= 0 && u.Skill.Kind == SkillDamage {
//		es := b.enemiesInRange(u, u.Skill.Range)
//		// 遍历所有敌人评分更稳（而不是 chooseTargetBy）
//		for _, t := range es {
//			s := scoreCastSkillDamage(u, t, b)
//			best = better(best, Decision{Kind: ActCastSkill, Target: t, Score: s})
//		}
//	}
//	// 普攻
//	ts := b.enemiesInRange(u, u.Range)
//	for _, t := range ts {
//		s := scoreAttack(u, t, b)
//		best = better(best, Decision{Kind: ActAttack, Target: t, Score: s})
//	}
//	// 靠近
//	if e := b.nearestEnemy(u); e != nil {
//		if np, ok := bestStepCloser(u, e.Pos, b); ok {
//			s := scoreMoveCloser(u, np, e, b)
//			best = better(best, Decision{Kind: ActMoveCloser, StepTo: &np, Score: s})
//		}
//	}
//	if best.Kind == ActNone {
//		return Decision{Kind: ActNone, Score: -1}
//	}
//	return best
//}
//func bestStepCloser(u *Unit, target Pos, b *Battle) (Pos, bool) {
//	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
//	occ := b.occupied()
//	delete(occ, u.Pos)
//	best := Pos{}
//	ok := false
//	bestDist := 1 << 30
//	now := manhattan(u.Pos, target)
//	for _, d := range dirs {
//		np := Pos{u.Pos.R + d.R, u.Pos.C + d.C}
//		if !inBounds(np) || occ[np] {
//			continue
//		}
//		nd := manhattan(np, target)
//		if nd < now {
//			if !ok || nd < bestDist || (nd == bestDist && (np.R < best.R || (np.R == best.R && np.C < best.C))) {
//				ok = true
//				bestDist = nd
//				best = np
//			}
//		}
//	}
//	return best, ok
//}
//
///***************
// * 行为树（BT）硬约束：低血先治疗/撤退
// ***************/
//type BTStatus int
//
//const (
//	BTSuccess BTStatus = iota
//	BTFailure
//	BTRunning
//)
//
//type BTNode interface{ Tick(*Blackboard) BTStatus }
//type Blackboard struct {
//	B   *Battle
//	U   *Unit
//	Log func(string, ...any)
//}
//
//type Selector struct{ Children []BTNode }
//
//func (s *Selector) Tick(bb *Blackboard) BTStatus {
//	for _, ch := range s.Children {
//		if ch.Tick(bb) == BTSuccess {
//			return BTSuccess
//		}
//	}
//	return BTFailure
//}
//
//type Sequence struct{ Children []BTNode }
//
//func (s *Sequence) Tick(bb *Blackboard) BTStatus {
//	for _, ch := range s.Children {
//		if ch.Tick(bb) != BTSuccess {
//			return BTFailure
//		}
//	}
//	return BTSuccess
//}
//
//type Condition func(*Blackboard) bool
//type CondNode struct{ Fn Condition }
//
//func (c *CondNode) Tick(bb *Blackboard) BTStatus {
//	if c.Fn(bb) {
//		return BTSuccess
//	}
//	return BTFailure
//}
//
//type Action func(*Blackboard) bool
//type ActionNode struct{ Fn Action }
//
//func (a *ActionNode) Tick(bb *Blackboard) BTStatus {
//	if a.Fn(bb) {
//		return BTSuccess
//	}
//	return BTFailure
//}
//
//func isLowHP(bb *Blackboard) bool {
//	u := bb.U
//	return u.HP*100 <= u.MaxHP*bb.B.LowHPPct
//}
//func hasHealReady(bb *Blackboard) bool {
//	u := bb.U
//	return u.Skill != nil && u.Skill.Kind == SkillHealSelf && u.Skill.CDLeft <= 0
//}
//func doHeal(bb *Blackboard) bool {
//	u := bb.U
//	if u.Skill == nil || u.Skill.Kind != SkillHealSelf || u.Skill.CDLeft > 0 {
//		return false
//	}
//	old := u.HP
//	heal := u.Skill.Heal
//	u.HP += heal
//	if u.HP > u.MaxHP {
//		u.HP = u.MaxHP
//	}
//	u.Skill.CDLeft = u.Skill.CD
//	bb.B.pushEvent(Event{TimeTick: bb.B.TimeTick, Actor: u.Name, Action: "heal", Heal: heal, FromSkill: u.Skill.Name})
//	bb.Log(" - %s casts %s, heal %d (%d->%d)", u.Name, u.Skill.Name, heal, old, u.HP)
//	return true
//}
//func canRetreat(bb *Blackboard) bool {
//	u := bb.U
//	enemy := bb.B.nearestEnemy(u)
//	if enemy == nil {
//		return false
//	}
//	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
//	occ := bb.B.occupied()
//	delete(occ, u.Pos)
//	now := manhattan(u.Pos, enemy.Pos)
//	for _, d := range dirs {
//		np := Pos{u.Pos.R + d.R, u.Pos.C + d.C}
//		if inBounds(np) && !occ[np] && manhattan(np, enemy.Pos) > now {
//			return true
//		}
//	}
//	return false
//}
//func doRetreat(bb *Blackboard) bool {
//	u := bb.U
//	if bb.B.tryMoveAway(u) {
//		bb.Log(" - %s retreats to (%d,%d)", u.Name, u.Pos.R, u.Pos.C)
//		return true
//	}
//	bb.Log(" - %s failed to retreat", u.Name)
//	return false
//}
//
//// 评分动作（含暴击、溅射、施加 Debuff）
//func (b *Battle) applyDamage(src, tgt *Unit, baseDamage int, fromSkill string) (final int, crit bool) {
//	// 暴击
//	dmg := float64(baseDamage)
//	if rand.Float64() < src.CritChance {
//		dmg = dmg * src.CritMult
//		crit = true
//	}
//	// 目标易伤
//	dmg = dmg * tgt.dmgTakenMultiplier()
//	final = int(math.Round(dmg))
//	if final < 1 {
//		final = 1
//	}
//	old := tgt.HP
//	tgt.HP -= final
//	b.pushEvent(Event{TimeTick: b.TimeTick, Actor: src.Name, Action: "hit", Target: tgt.Name, Damage: final, Crit: crit, FromSkill: fromSkill})
//	if tgt.HP <= 0 && tgt.Alive {
//		tgt.Alive = false
//		b.logf("   > %s is defeated", tgt.Name)
//		b.pushEvent(Event{TimeTick: b.TimeTick, Actor: src.Name, Action: "kill", Target: tgt.Name, FromSkill: fromSkill})
//	} else {
//		b.logf("   > %s hits %s for %d (%d->%d)%s", src.Name, tgt.Name, final, old, tgt.HP, map[bool]string{true: " CRIT", false: ""}[crit])
//	}
//	return
//}
//func (b *Battle) doCastDamage(u *Unit, t *Unit) {
//	if u.Skill == nil || u.Skill.CDLeft > 0 || u.Skill.Kind != SkillDamage {
//		return
//	}
//	b.logf(" - %s casts %s on %s", u.Name, u.Skill.Name, t.Name)
//	_, _ = b.applyDamage(u, t, u.Skill.Damage, u.Skill.Name)
//
//	// 溅射
//	if u.Skill.SplashPct > 0 {
//		for _, e := range b.enemiesOf(u) {
//			if e == t || !e.Alive {
//				continue
//			}
//			if manhattan(e.Pos, t.Pos) == 1 { // 贴邻
//				splash := int(math.Round(float64(u.Skill.Damage) * u.Skill.SplashPct))
//				_, _ = b.applyDamage(u, e, splash, u.Skill.Name+"(splash)")
//			}
//		}
//	}
//	// 附加 Debuff（给主目标）
//	if u.Skill.ApplyBuffOnHit != nil && t.Alive {
//		cp := *u.Skill.ApplyBuffOnHit
//		cp.SourceUnit = u.Name
//		t.Buffs = append(t.Buffs, cp)
//		b.logf("   > %s applied debuff [%s] to %s", u.Name, cp.Name, t.Name)
//		b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "applyBuff", Target: t.Name, BuffApplied: cp.Name, FromSkill: u.Skill.Name})
//	}
//
//	u.Skill.CDLeft = u.Skill.CD
//}
//func (b *Battle) doAttack(u *Unit, t *Unit) {
//	atk := u.currentATK()
//	b.logf(" - %s attacks %s", u.Name, t.Name)
//	_, _ = b.applyDamage(u, t, atk, "Attack")
//}
//func (b *Battle) doMove(u *Unit, np Pos) {
//	u.Pos = np
//	b.logf(" - %s moves to (%d,%d)", u.Name, u.Pos.R, u.Pos.C)
//	b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "move", NewPos: &u.Pos})
//}
//
//func (b *Battle) doUtilityDecision(u *Unit) {
//	dec := b.decide(u)
//	switch dec.Kind {
//	case ActCastSkill:
//		if dec.Target != nil && dec.Target.Alive {
//			b.doCastDamage(u, dec.Target)
//		}
//	case ActAttack:
//		if dec.Target != nil && dec.Target.Alive {
//			b.doAttack(u, dec.Target)
//		}
//	case ActMoveCloser:
//		if dec.StepTo != nil {
//			b.doMove(u, *dec.StepTo)
//		} else {
//			b.logf(" - %s waits", u.Name)
//		}
//	default:
//		b.logf(" - %s waits", u.Name)
//	}
//}
//
//func (b *Battle) stepUnitBT(u *Unit) {
//	if !u.Alive {
//		return
//	}
//	bb := &Blackboard{B: b, U: u, Log: func(f string, a ...any) { b.logf(f, a...) }}
//	root := &Selector{Children: []BTNode{
//		&Sequence{Children: []BTNode{
//			&CondNode{Fn: isLowHP},
//			&Selector{Children: []BTNode{
//				&Sequence{Children: []BTNode{&CondNode{Fn: hasHealReady}, &ActionNode{Fn: doHeal}}},
//				&Sequence{Children: []BTNode{&CondNode{Fn: canRetreat}, &ActionNode{Fn: doRetreat}}},
//			}},
//		}},
//		&ActionNode{Fn: func(bb *Blackboard) bool { bb.B.doUtilityDecision(bb.U); return true }},
//	}}
//	_ = root.Tick(bb)
//}
//
///***************
// * 时间推进（含：CD & Buff 按时间衰减）
// ***************/
//func (b *Battle) advanceTime(minT int) {
//	if minT <= 0 {
//		return
//	}
//	b.TimeTick += minT
//	for _, u := range b.Units {
//		if !u.Alive {
//			continue
//		}
//		// 按当前 SPD 推进 CT
//		spd := u.currentSPD()
//		u.CT += minT * spd
//
//		// 冷却
//		if u.Skill != nil && u.Skill.CDLeft > 0 {
//			u.Skill.CDLeft -= minT
//			if u.Skill.CDLeft < 0 {
//				u.Skill.CDLeft = 0
//			}
//		}
//		// Buff 持续时间
//		dst := u.Buffs[:0]
//		for _, bf := range u.Buffs {
//			bf.tick(minT)
//			if bf.Duration > 0 {
//				dst = append(dst, bf)
//			} else {
//				// 记录过期
//				b.pushEvent(Event{TimeTick: b.TimeTick, Actor: u.Name, Action: "buffExpire", BuffExpire: bf.Name})
//			}
//		}
//		u.Buffs = dst
//	}
//}
//
//func (b *Battle) nextActorAdvance() *Unit {
//	type cand struct {
//		U *Unit
//		T int
//	}
//	var cs []cand
//	for _, u := range b.Units {
//		if !u.Alive || u.currentSPD() <= 0 {
//			continue
//		}
//		if u.CT >= CTThreshold {
//			cs = append(cs, cand{U: u, T: 0})
//		} else {
//			need := CTThreshold - u.CT
//			spd := u.currentSPD()
//			cs = append(cs, cand{U: u, T: divCeil(need, spd)})
//		}
//	}
//	if len(cs) == 0 {
//		return nil
//	}
//	minT := math.MaxInt32
//	for _, c := range cs {
//		if c.T < minT {
//			minT = c.T
//		}
//	}
//	// 推进时间（CT、CD、Buff 都跟随时间推进）
//	b.advanceTime(minT)
//
//	// 选可行动者
//	var ready []*Unit
//	for _, u := range b.Units {
//		if u.Alive && u.CT >= CTThreshold {
//			ready = append(ready, u)
//		}
//	}
//	if len(ready) == 0 {
//		return nil
//	}
//	sort.Slice(ready, func(i, j int) bool {
//		si, sj := ready[i].currentSPD(), ready[j].currentSPD()
//		if si != sj {
//			return si > sj
//		}
//		return ready[i].Name < ready[j].Name
//	})
//	return ready[0]
//}
//
///***************
// * 终局判断
// ***************/
//func (b *Battle) isOver() (bool, string) {
//	aliveA, aliveB := 0, 0
//	for _, u := range b.Units {
//		if u.Alive {
//			if u.Team == TeamA {
//				aliveA++
//			} else {
//				aliveB++
//			}
//		}
//	}
//	if aliveA == 0 && aliveB == 0 {
//		return true, "Draw"
//	}
//	if aliveA == 0 {
//		return true, TeamB
//	}
//	if aliveB == 0 {
//		return true, TeamA
//	}
//	if b.Actions >= MaxActions {
//		return true, "Draw"
//	}
//	return false, ""
//}
//
///***************
// * 主循环
// ***************/
//func (b *Battle) Run() string {
//	b.logf("======== BATTLE START ========")
//	for _, u := range b.Units {
//		b.logf(" * %s", u)
//	}
//	for {
//		if over, winner := b.isOver(); over {
//			b.logf("======== BATTLE END ========")
//			b.logf("Winner: %s", winner)
//			return winner
//		}
//		actor := b.nextActorAdvance()
//		if actor == nil {
//			b.logf("No actor available; Draw")
//			return "Draw"
//		}
//		b.logf("---- ACTION #%d: %s (spd=%d ct=%d, time=%d) ----", b.Actions+1, actor.Name, actor.currentSPD(), actor.CT, b.TimeTick)
//		b.stepUnitBT(actor)
//		actor.CT -= CTThreshold
//		if actor.CT < 0 {
//			actor.CT = 0
//		}
//		for _, u := range b.Units {
//			if u.Alive {
//				b.logf(" * %s", u)
//			} else {
//				b.logf(" * %s (dead)", u.Name)
//			}
//		}
//		b.Actions++
//	}
//}
//
///***************
// * 初始化 & 仿真
// ***************/
//type Config struct {
//	A1HP, A2HP int
//	B1HP, B2HP int
//}
//
//func makeRandomPositions() (Pos, Pos, Pos, Pos) {
//	rows := []int{0, 1, 2}
//	rand.Shuffle(3, func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
//	a1 := Pos{rows[0], 0}
//	a2 := Pos{rows[1], 0}
//	rand.Shuffle(3, func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
//	b1 := Pos{rows[0], 2}
//	b2 := Pos{rows[1], 2}
//	return a1, a2, b1, b2
//}
//
//func makeBattleForSim(cfg Config) *Battle {
//	a1pos, a2pos, b1pos, b2pos := makeRandomPositions()
//
//	// Debuff 模板
//	slow60 := &Buff{Name: "Slow", SPDMul: -0.2, Duration: 60}
//	vuln60 := &Buff{Name: "Vulnerable", DMGTaken: +0.15, Duration: 60}
//
//	a1 := &Unit{Name: "A1", Team: TeamA, MaxHP: cfg.A1HP, HP: cfg.A1HP, BaseATK: 4, Range: 1, Pos: a1pos, Alive: true, SPD: 8,
//		Skill:      &Skill{Name: "PowerStrike", Kind: SkillDamage, Damage: 6, Range: 1, CD: 3 * flagTickPerAction, ApplyBuffOnHit: slow60},
//		CritChance: 0.20, CritMult: 1.5, Personality: Personality{Aggression: 0.6, Cautious: 0.3}}
//	a2 := &Unit{Name: "A2", Team: TeamA, MaxHP: cfg.A2HP, HP: cfg.A2HP, BaseATK: 3, Range: 1, Pos: a2pos, Alive: true, SPD: 7,
//		Skill:      &Skill{Name: "SecondWind", Kind: SkillHealSelf, Heal: 5, Range: 0, CD: 3 * flagTickPerAction},
//		CritChance: 0.10, CritMult: 1.5, Personality: Personality{Aggression: 0.3, Cautious: 0.5}}
//
//	b1 := &Unit{Name: "B1", Team: TeamB, MaxHP: cfg.B1HP, HP: cfg.B1HP, BaseATK: 4, Range: 1, Pos: b1pos, Alive: true, SPD: 9,
//		Skill:      &Skill{Name: "Firebolt", Kind: SkillDamage, Damage: 5, Range: 2, CD: 2 * flagTickPerAction, SplashPct: 0.5, ApplyBuffOnHit: vuln60},
//		CritChance: 0.15, CritMult: 1.5, Personality: Personality{Aggression: 0.5, Cautious: 0.2}}
//	b2 := &Unit{Name: "B2", Team: TeamB, MaxHP: cfg.B2HP, HP: cfg.B2HP, BaseATK: 3, Range: 1, Pos: b2pos, Alive: true, SPD: 6,
//		CritChance: 0.10, CritMult: 1.5, Personality: Personality{Aggression: 0.4, Cautious: 0.4}}
//
//	return &Battle{Units: []*Unit{a1, a2, b1, b2}, LogOn: false, TickPerAct: flagTickPerAction, LowHPPct: flagLowHPPercent}
//}
//
//func makeBattleDemo() *Battle {
//	// 固定站位 + 打印日志 + 保存事件
//	// Debuff 模板
//	slow60 := &Buff{Name: "Slow", SPDMul: -0.2, Duration: 60}
//	vuln60 := &Buff{Name: "Vulnerable", DMGTaken: +0.15, Duration: 60}
//
//	a1 := &Unit{Name: "A1", Team: TeamA, MaxHP: 12, HP: 12, BaseATK: 4, Range: 1, Pos: Pos{0, 0}, Alive: true, SPD: 8,
//		Skill:      &Skill{Name: "PowerStrike", Kind: SkillDamage, Damage: 6, Range: 1, CD: 3 * flagTickPerAction, ApplyBuffOnHit: slow60},
//		CritChance: 0.20, CritMult: 1.5, Personality: Personality{Aggression: 0.6, Cautious: 0.3}}
//	a2 := &Unit{Name: "A2", Team: TeamA, MaxHP: 10, HP: 10, BaseATK: 3, Range: 1, Pos: Pos{2, 0}, Alive: true, SPD: 7,
//		Skill:      &Skill{Name: "SecondWind", Kind: SkillHealSelf, Heal: 5, Range: 0, CD: 3 * flagTickPerAction},
//		CritChance: 0.10, CritMult: 1.5, Personality: Personality{Aggression: 0.3, Cautious: 0.5}}
//
//	b1 := &Unit{Name: "B1", Team: TeamB, MaxHP: 12, HP: 12, BaseATK: 4, Range: 1, Pos: Pos{0, 2}, Alive: true, SPD: 9,
//		Skill:      &Skill{Name: "Firebolt", Kind: SkillDamage, Damage: 5, Range: 2, CD: 2 * flagTickPerAction, SplashPct: 0.5, ApplyBuffOnHit: vuln60},
//		CritChance: 0.15, CritMult: 1.5, Personality: Personality{Aggression: 0.5, Cautious: 0.2}}
//	b2 := &Unit{Name: "B2", Team: TeamB, MaxHP: 10, HP: 10, BaseATK: 3, Range: 1, Pos: Pos{2, 2}, Alive: true, SPD: 6,
//		CritChance: 0.10, CritMult: 1.5, Personality: Personality{Aggression: 0.4, Cautious: 0.4}}
//
//	return &Battle{Units: []*Unit{a1, a2, b1, b2}, LogOn: true, TickPerAct: flagTickPerAction, LowHPPct: flagLowHPPercent}
//}
//
///***************
// * main
// ***************/
//func main() {
//	flag.BoolVar(&flagSim, "simsvc", false, "run simulation batch instead of single demo")
//	flag.IntVar(&flagN, "n", 1000, "number of battles to simulate")
//	flag.Int64Var(&flagSeed, "seed", 0, "random seed (0 = now)")
//	flag.StringVar(&flagSaveJSON, "savejson", "", "save single-battle event log to JSON file")
//	flag.IntVar(&flagLowHPPercent, "lowhp", 30, "low HP threshold percent for BT hard constraint")
//	flag.IntVar(&flagTickPerAction, "tickperaction", 30, "ticks equivalent to one action (cooldown scale)")
//	flag.Parse()
//
//	if flagSeed == 0 {
//		flagSeed = time.Now().UnixNano()
//	}
//	rand.Seed(flagSeed)
//
//	if !flagSim {
//		fmt.Println("Start Time:", time.Now().Format(time.RFC3339))
//		b := makeBattleDemo()
//		winner := b.Run()
//		fmt.Println("Winner:", winner)
//		if flagSaveJSON != "" {
//			data, _ := json.MarshalIndent(b.Events, "", "  ")
//			_ = os.WriteFile(flagSaveJSON, data, 0644)
//			fmt.Println("Saved event log to:", flagSaveJSON)
//		}
//		return
//	}
//
//	// 批量仿真
//	base := Config{A1HP: 12, A2HP: 10, B1HP: 12, B2HP: 10}
//	var winA, winB, draw int
//	for i := 0; i < flagN; i++ {
//		cfg := base
//		// 轻微扰动（±1 HP）
//		cfg.A1HP += rand.Intn(3) - 1
//		cfg.A2HP += rand.Intn(3) - 1
//		cfg.B1HP += rand.Intn(3) - 1
//		cfg.B2HP += rand.Intn(3) - 1
//		if cfg.A1HP < 8 {
//			cfg.A1HP = 8
//		}
//		if cfg.A2HP < 7 {
//			cfg.A2HP = 7
//		}
//		if cfg.B1HP < 8 {
//			cfg.B1HP = 8
//		}
//		if cfg.B2HP < 7 {
//			cfg.B2HP = 7
//		}
//
//		b := makeBattleForSim(cfg)
//		w := b.Run()
//		switch w {
//		case TeamA:
//			winA++
//		case TeamB:
//			winB++
//		default:
//			draw++
//		}
//	}
//
//	fmt.Println("====== Simulation Result ======")
//	fmt.Printf("Seed: %d, Battles: %d\n", flagSeed, flagN)
//	fmt.Printf("Team A Wins: %d (%.2f%%)\n", winA, 100.0*float64(winA)/float64(flagN))
//	fmt.Printf("Team B Wins: %d (%.2f%%)\n", winB, 100.0*float64(winB)/float64(flagN))
//	fmt.Printf("Draws:       %d (%.2f%%)\n", draw, 100.0*float64(draw)/float64(flagN))
//}
