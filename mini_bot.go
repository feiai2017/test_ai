package main

//
//import (
//	"fmt"
//	"math"
//	"math/rand"
//	"sort"
//	"time"
//)
//
//const (
//	GridH    = 3
//	GridW    = 3
//	MaxTurns = 50
//	TeamA    = "A"
//	TeamB    = "B"
//)
//
//type Pos struct{ R, C int }
//
//func inBounds(p Pos) bool {
//	return 0 <= p.R && p.R < GridH && 0 <= p.C && p.C < GridW
//}
//func manhattan(a, b Pos) int { return abs(a.R-b.R) + abs(a.C-b.C) }
//func abs(x int) int {
//	if x < 0 {
//		return -x
//	}
//	return x
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
//}
//
//func (u *Unit) String() string {
//	return fmt.Sprintf("%s(%s) hp=%d atk=%d rng=%d pos=(%d,%d)", u.Name, u.Team, u.HP, u.ATK, u.Range, u.Pos.R, u.Pos.C)
//}
//
//type Battle struct {
//	Units []*Unit
//	Turn  int
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
//
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
//	out := make(map[Pos]bool)
//	for _, u := range b.Units {
//		if u.Alive {
//			out[u.Pos] = true
//		}
//	}
//	return out
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
//		if es[i].Pos.R != es[j].Pos.R {
//			return es[i].Pos.R < es[j].Pos.R
//		}
//		return es[i].Pos.C < es[j].Pos.C
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
//func (b *Battle) tryMoveTowards(u *Unit, target Pos) bool {
//	dirs := []Pos{{-1, 0}, {0, -1}, {0, 1}, {1, 0}}
//	occ := b.occupied()
//	delete(occ, u.Pos) // can move from current position
//
//	bestDist := math.MaxInt32
//	var candidates []Pos
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
//				candidates = []Pos{np}
//			} else if distNext == bestDist {
//				candidates = append(candidates, np)
//			}
//		}
//	}
//	if len(candidates) == 0 {
//		return false
//	}
//	sort.Slice(candidates, func(i, j int) bool {
//		if candidates[i].R != candidates[j].R {
//			return candidates[i].R < candidates[j].R
//		}
//		return candidates[i].C < candidates[j].C
//	})
//	u.Pos = candidates[0]
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
//	if b.Turn >= MaxTurns {
//		return true, "Draw"
//	}
//	return false, ""
//}
//
//func (b *Battle) StepUnit(u *Unit) {
//	if !u.Alive {
//		return
//	}
//	if targets := b.enemiesInRange(u); len(targets) > 0 {
//		t := targets[0]
//		fmt.Printf("- %s attacks %s for %d damage\n", u.Name, t.Name, u.ATK)
//		t.HP -= u.ATK
//		if t.HP <= 0 && t.Alive {
//			t.Alive = false
//			fmt.Printf("  > %s is defeated!\n", t.Name)
//		}
//		return
//	}
//
//	enemy := b.nearestEnemy(u)
//	if enemy == nil {
//		return
//	}
//	moved := b.tryMoveTowards(u, enemy.Pos)
//	if moved {
//		fmt.Printf("- %s moves to (%d,%d)\n", u.Name, u.Pos.R, u.Pos.C)
//	} else {
//		fmt.Printf("- %s stays (blocked)\n", u.Name)
//	}
//}
//
//func (b *Battle) Run() {
//	for {
//		over, winner := b.isOver()
//		if over {
//			fmt.Println("============= BATTLE OVER =============")
//			fmt.Printf("Winner: %s\n", winner)
//			return
//		}
//		fmt.Printf("============ TURN %d =============\n", b.Turn)
//
//		for _, u := range b.Units {
//			if u.Alive {
//				b.StepUnit(u)
//			}
//		}
//
//		fmt.Println("State: ")
//		for _, u := range b.Units {
//			if u.Alive {
//				fmt.Printf(" * %s\n", u)
//			} else {
//				fmt.Printf(" * %s (dead)\n", u.Name)
//			}
//		}
//		b.Turn++
//	}
//}
//
//func main() {
//	rand.Seed(42)
//
//	a1 := &Unit{Name: "A1", Team: TeamA, HP: 10, ATK: 4, Range: 1, Pos: Pos{0, 0}, Alive: true}
//	a2 := &Unit{Name: "A2", Team: TeamA, HP: 8, ATK: 5, Range: 2, Pos: Pos{0, 2}, Alive: true}
//	b1 := &Unit{Name: "B1", Team: TeamB, HP: 12, ATK: 3, Range: 1, Pos: Pos{2, 0}, Alive: true}
//	b2 := &Unit{Name: "B2", Team: TeamB, HP: 9, ATK: 4, Range: 2, Pos: Pos{2, 2}, Alive: true}
//
//	b := &Battle{
//		Units: []*Unit{a1, a2, b1, b2},
//		Turn:  1,
//	}
//
//	fmt.Println("============= BATTLE START =============")
//	fmt.Printf("Start Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
//	for _, u := range b.Units {
//		fmt.Printf(" * %s\n", u)
//	}
//	b.Run()
//}
