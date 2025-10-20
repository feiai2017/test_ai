package main

//import (
//	"fmt"
//	"math"
//	"sort"
//	"time"
//)
//
//const (
//	GridH       = 3
//	GridW       = 3
//	CTThreshold = 100 // 行动条满槽阈值
//	MaxActions  = 500 // 安全上限
//	TeamA       = "A"
//	TeamB       = "B"
//)
//
//type Pos struct{ R, C int }
//
//func inBounds(p Pos) bool { return p.R >= 0 && p.R < GridH && p.C >= 0 && p.C < GridW }
//func manhattan(a, b Pos) int {
//	d := a.R - b.R
//	if d < 0 {
//		d = -d
//	}
//	e := a.C - b.C
//	if e < 0 {
//		e = -e
//	}
//	return d + e
//}
//func divCeil(a, b int) int { return (a + b - 1) / b }
//
//type Skill struct {
//	Name     string
//	Damage   int // 技能固定伤害（演示用）
//	Range    int // 技能射程（曼哈顿）
//	Cooldown int // 冷却回合数（以“全局动作数”为刻度）
//	CDLeft   int // 距离可用还剩多少“动作”
//}
//
//type Unit struct {
//	Name  string
//	Team  string
//	HP    int
//	ATK   int // 普通攻击伤害
//	Range int // 普通攻击射程
//	Pos   Pos
//	Alive bool
//	SPD   int    // 速度
//	CT    int    // 行动条
//	Skill *Skill // 可为空
//}
//
//func (u *Unit) String() string {
//	if u.Skill != nil {
//		return fmt.Sprintf("%s(%s) hp=%d pos=(%d,%d) spd=%d ct=%d | skill[%s R=%d DMG=%d CD=%d]",
//			u.Name, u.Team, u.HP, u.Pos.R, u.Pos.C, u.SPD, u.CT,
//			u.Skill.Name, u.Skill.Range, u.Skill.Damage, u.Skill.CDLeft)
//	}
//	return fmt.Sprintf("%s(%s) hp=%d pos=(%d,%d) spd=%d ct=%d",
//		u.Name, u.Team, u.HP, u.Pos.R, u.Pos.C, u.SPD, u.CT)
//}
//
//type Battle struct {
//	Units   []*Unit
//	Actions int // 已执行的“动作”计数
//}
//
//func (b *Battle) enemiesOf(u *Unit) []*Unit {
//	var out []*Unit
//	for _, x := range b.Units {
//		if x.Alive && x.Team != u.Team {
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
//
//// 通用：按“最脆 → 最近 → 名字”选目标
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
//
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
//// 朝 target 迈出一格（更近且不撞人）；顺序上、左、右、下；若有多个等价，按行列稳定
//func (b *Battle) tryMoveTowards(u *Unit, target Pos) bool {
//	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
//	occ := b.occupied()
//	delete(occ, u.Pos)
//
//	bestDist := math.MaxInt32
//	var cands []Pos
//	distNow := manhattan(u.Pos, target)
//
//	for _, d := range dirs {
//		np := Pos{u.Pos.R + d.R, u.Pos.C + d.C}
//		if !inBounds(np) || occ[np] {
//			continue
//		}
//		distNext := manhattan(np, target)
//		if distNext < distNow {
//			if distNext < bestDist {
//				bestDist = distNext
//				cands = []Pos{np}
//			} else if distNext == bestDist {
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
//	return true
//}
//
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
//// 技能是否可用（CD好、射程内有敌）
//func (b *Battle) canUseSkill(u *Unit) (*Unit, bool) {
//	if u.Skill == nil || u.Skill.CDLeft > 0 {
//		return nil, false
//	}
//	es := b.enemiesInRange(u, u.Skill.Range)
//	if len(es) == 0 {
//		return nil, false
//	}
//	t := chooseTargetBy(es, u.Pos)
//	return t, true
//}
//
//func (b *Battle) stepUnit(u *Unit) {
//	if !u.Alive {
//		return
//	}
//
//	// 1) 有技能且可用 → 放技能
//	if t, ok := b.canUseSkill(u); ok {
//		fmt.Printf(" - %s casts %s on %s for %d\n", u.Name, u.Skill.Name, t.Name, u.Skill.Damage)
//		t.HP -= u.Skill.Damage
//		u.Skill.CDLeft = u.Skill.Cooldown
//		if t.HP <= 0 && t.Alive {
//			t.Alive = false
//			fmt.Printf("   > %s is defeated\n", t.Name)
//		}
//		return
//	}
//
//	// 2) 普通攻击（有敌在普攻射程内）
//	if targets := b.enemiesInRange(u, u.Range); len(targets) > 0 {
//		t := chooseTargetBy(targets, u.Pos)
//		fmt.Printf(" - %s attacks %s for %d\n", u.Name, t.Name, u.ATK)
//		t.HP -= u.ATK
//		if t.HP <= 0 && t.Alive {
//			t.Alive = false
//			fmt.Printf("   > %s is defeated\n", t.Name)
//		}
//		return
//	}
//
//	// 3) 否则向最近敌人移动
//	enemy := b.nearestEnemy(u)
//	if enemy == nil {
//		return
//	}
//	if b.tryMoveTowards(u, enemy.Pos) {
//		fmt.Printf(" - %s moves to (%d,%d)\n", u.Name, u.Pos.R, u.Pos.C)
//	} else {
//		fmt.Printf(" - %s stays (blocked)\n", u.Name)
//	}
//}
//
//// 推进到“下一个能行动”的时刻，并选出行动者（SPD 优先→名字）
//func (b *Battle) nextActorAdvance() *Unit {
//	type cand struct {
//		U *Unit
//		T int
//	}
//	var cs []cand
//	for _, u := range b.Units {
//		if !u.Alive || u.SPD <= 0 {
//			continue
//		}
//		if u.CT >= CTThreshold {
//			cs = append(cs, cand{U: u, T: 0})
//			continue
//		}
//		need := CTThreshold - u.CT
//		cs = append(cs, cand{U: u, T: divCeil(need, u.SPD)})
//	}
//	if len(cs) == 0 {
//		return nil
//	}
//
//	minT := math.MaxInt32
//	for _, c := range cs {
//		if c.T < minT {
//			minT = c.T
//		}
//	}
//	if minT > 0 {
//		for _, u := range b.Units {
//			if u.Alive && u.SPD > 0 {
//				u.CT += minT * u.SPD
//			}
//		}
//	}
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
//		if ready[i].SPD != ready[j].SPD {
//			return ready[i].SPD > ready[j].SPD
//		}
//		return ready[i].Name < ready[j].Name
//	})
//	return ready[0]
//}
//
//func (b *Battle) tickGlobalCooldowns() {
//	// 简化：每发生一次“动作”，全体单位的技能冷却 -1（不少于 0）
//	for _, u := range b.Units {
//		if u.Skill != nil && u.Skill.CDLeft > 0 {
//			u.Skill.CDLeft--
//			if u.Skill.CDLeft < 0 {
//				u.Skill.CDLeft = 0
//			}
//		}
//	}
//}
//
//func (b *Battle) Run() {
//	fmt.Println("======== BATTLE START ========")
//	for _, u := range b.Units {
//		fmt.Printf(" * %s\n", u)
//	}
//
//	for {
//		if over, winner := b.isOver(); over {
//			fmt.Println("======== BATTLE END ========")
//			fmt.Printf("Winner: %s\n", winner)
//			return
//		}
//
//		actor := b.nextActorAdvance()
//		if actor == nil {
//			fmt.Println("No actor available; draw.")
//			return
//		}
//		fmt.Printf("---- ACTION #%d: %s (spd=%d ct=%d) ----\n",
//			b.Actions+1, actor.Name, actor.SPD, actor.CT)
//
//		b.stepUnit(actor)
//		actor.CT -= CTThreshold
//		if actor.CT < 0 {
//			actor.CT = 0
//		}
//
//		// 每个动作结束，推进一次全局冷却
//		b.tickGlobalCooldowns()
//
//		// 打印状态
//		fmt.Println("State:")
//		for _, u := range b.Units {
//			if u.Alive {
//				fmt.Printf(" * %s\n", u)
//			} else {
//				fmt.Printf(" * %s (dead)\n", u.Name)
//			}
//		}
//		b.Actions++
//	}
//}
//
//func main() {
//	// A 队：近战 + 爆发技能（Power Strike）
//	a1 := &Unit{
//		Name: "A1", Team: TeamA, HP: 11, ATK: 4, Range: 1, Pos: Pos{0, 0}, Alive: true, SPD: 8,
//		Skill: &Skill{Name: "Power Strike", Damage: 6, Range: 1, Cooldown: 3, CDLeft: 0},
//	}
//	a2 := &Unit{
//		Name: "A2", Team: TeamA, HP: 8, ATK: 3, Range: 1, Pos: Pos{2, 0}, Alive: true, SPD: 7,
//		// 无技能
//	}
//
//	// B 队：一个远程技能（Firebolt），一个近战
//	b1 := &Unit{
//		Name: "B1", Team: TeamB, HP: 10, ATK: 4, Range: 1, Pos: Pos{0, 2}, Alive: true, SPD: 9,
//		Skill: &Skill{Name: "Firebolt", Damage: 5, Range: 2, Cooldown: 2, CDLeft: 0},
//	}
//	b2 := &Unit{
//		Name: "B2", Team: TeamB, HP: 8, ATK: 3, Range: 1, Pos: Pos{2, 2}, Alive: true, SPD: 6,
//	}
//
//	b := &Battle{Units: []*Unit{a1, a2, b1, b2}}
//
//	fmt.Println("Start Time:", time.Now().Format(time.RFC3339))
//	b.Run()
//}
