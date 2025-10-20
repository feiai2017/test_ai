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
//	CTThreshold = 100 // 满槽阈值
//	MaxActions  = 500 // 安全上限，避免死循环
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
//func divCeil(a, b int) int {
//	// 假设 a,b > 0
//	return (a + b - 1) / b
//}
//
//type Unit struct {
//	Name  string
//	Team  string
//	HP    int
//	ATK   int
//	Range int
//	Pos   Pos
//	Alive bool
//	SPD   int // 新增：速度
//	CT    int // 新增：行动条
//}
//
//func (u *Unit) String() string {
//	return fmt.Sprintf("%s(%s) hp=%d pos=(%d,%d) spd=%d ct=%d",
//		u.Name, u.Team, u.HP, u.Pos.R, u.Pos.C, u.SPD, u.CT)
//}
//
//type Battle struct {
//	Units   []*Unit
//	Actions int // 已发生的出手数（代替回合数）
//	// 注意：一个“动作”是某个单位的一次攻击或移动
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
//
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
//func (b *Battle) enemiesInRange(u *Unit) []*Unit {
//	var es []*Unit
//	for _, e := range b.enemiesOf(u) {
//		if manhattan(u.Pos, e.Pos) <= u.Range {
//			es = append(es, e)
//		}
//	}
//	sort.Slice(es, func(i, j int) bool {
//		// 打最脆 → 更近 → 名字
//		if es[i].HP != es[j].HP {
//			return es[i].HP < es[j].HP
//		}
//		di := manhattan(u.Pos, es[i].Pos)
//		dj := manhattan(u.Pos, es[j].Pos)
//		if di != dj {
//			return di < dj
//		}
//		return es[i].Name < es[j].Name
//	})
//	return es
//}
//
//// 让单位朝 target 迈出 1 步；必须更靠近且不撞人
//func (b *Battle) tryMoveTowards(u *Unit, target Pos) bool {
//	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}} // 上、左、右、下（稳定）
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
//func (b *Battle) stepUnit(u *Unit) {
//	if !u.Alive {
//		return
//	}
//	// 能打就打
//	if targets := b.enemiesInRange(u); len(targets) > 0 {
//		t := targets[0]
//		fmt.Printf(" - %s attacks %s for %d\n", u.Name, t.Name, u.ATK)
//		t.HP -= u.ATK
//		if t.HP <= 0 && t.Alive {
//			t.Alive = false
//			fmt.Printf("   > %s is defeated\n", t.Name)
//		}
//		return
//	}
//	// 否则前进
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
//// 推进时间：找到“最先能行动”的最小时间步 t（单位：tick），全体 CT += t*SPD
//// 然后在 CT>=阈值 的人中选择一个出手（SPD 高者优先，其次名字）
//func (b *Battle) nextActorAdvance() *Unit {
//	type cand struct {
//		U *Unit
//		// 需要的时间步；若已满槽则为 0
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
//		t := divCeil(need, u.SPD) // 最少需要的时间步
//		cs = append(cs, cand{U: u, T: t})
//	}
//	if len(cs) == 0 {
//		return nil
//	}
//
//	// 找到全体的最小 T（最早行动时间）
//	minT := math.MaxInt32
//	for _, c := range cs {
//		if c.T < minT {
//			minT = c.T
//		}
//	}
//	// 推进所有人的 CT
//	if minT > 0 {
//		for _, u := range b.Units {
//			if u.Alive && u.SPD > 0 {
//				u.CT += minT * u.SPD
//			}
//		}
//	}
//
//	// 现在挑选 CT>=阈值 的人中：SPD 高者优先，然后按名字稳定
//	var ready []*Unit
//	for _, u := range b.Units {
//		if u.Alive && u.CT >= CTThreshold {
//			ready = append(ready, u)
//		}
//	}
//	if len(ready) == 0 {
//		// 理论上不会发生，因为推进到了有人满槽
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
//func (b *Battle) Run() {
//	fmt.Println("======== BATTLE START ========")
//	for _, u := range b.Units {
//		fmt.Printf(" * %s\n", u)
//	}
//
//	for {
//		over, winner := b.isOver()
//		if over {
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
//		// 出手
//		fmt.Printf("---- ACTION #%d: %s (spd=%d ct=%d) ----\n",
//			b.Actions+1, actor.Name, actor.SPD, actor.CT)
//		b.stepUnit(actor)
//		actor.CT -= CTThreshold
//		if actor.CT < 0 {
//			actor.CT = 0
//		} // 保险
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
//	// 初始布阵 + SPD
//	a1 := &Unit{Name: "A1", Team: TeamA, HP: 10, ATK: 4, Range: 1, Pos: Pos{0, 0}, Alive: true, SPD: 8, CT: 0}
//	a2 := &Unit{Name: "A2", Team: TeamA, HP: 8, ATK: 3, Range: 1, Pos: Pos{2, 0}, Alive: true, SPD: 7, CT: 0}
//	b1 := &Unit{Name: "B1", Team: TeamB, HP: 10, ATK: 4, Range: 1, Pos: Pos{0, 2}, Alive: true, SPD: 9, CT: 0}
//	b2 := &Unit{Name: "B2", Team: TeamB, HP: 8, ATK: 3, Range: 1, Pos: Pos{2, 2}, Alive: true, SPD: 6, CT: 0}
//
//	b := &Battle{Units: []*Unit{a1, a2, b1, b2}}
//
//	fmt.Println("Start Time:", time.Now().Format(time.RFC3339))
//	b.Run()
//}
