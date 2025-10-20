package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"test_ai/internal/combat"
	"test_ai/internal/config"
	"test_ai/internal/util"
)

func main() {
	var cfgDir, out, bossID string
	var seed int64
	var n int
	var saveLog bool
	flag.StringVar(&cfgDir, "config", "assets", "config dir")
	flag.StringVar(&out, "out", "out.json", "output file (single) or summary file (batch)")
	flag.StringVar(&bossID, "boss", "boss001", "boss id")
	flag.Int64Var(&seed, "seed", 12345, "seed")
	flag.IntVar(&n, "n", 1, "number of simulations")
	flag.BoolVar(&saveLog, "log", true, "save full event log when n==1")
	flag.Parse()

	elems, reacts, skillsCfg, bossCfg, err := config.LoadAll(cfgDir)
	if err != nil {
		panic(err)
	}
	skillBook := combat.NewSkillBook(skillsCfg)

	if n <= 1 {
		env := &combat.Env{Rng: util.New(seed)}
		boss := combat.NewBoss(bossID, bossCfg.MaxHP, bossCfg.GuardMax)
		party := combat.NewParty([]combat.Hero{
			{ID: "pyro_knight", Elem: "fire", Tags: map[string]bool{}},
			{ID: "aqua_mage", Elem: "water", Tags: map[string]bool{}},
			{ID: "storm_rogue", Elem: "storm", Tags: map[string]bool{}},
		}, skillBook)
		events := make([]combat.Event, 0, 256)
		emit := func(ev combat.Event) { events = append(events, ev) }
		rr := combat.NewReactionResolver(reacts, elems, func() float64 { return env.Time }, emit)
		ph := combat.NewBossPhaser(bossCfg, boss, emit)

		res := combat.RunSingle(env, boss, party, rr, ph, saveLog)
		if saveLog && res.Events == nil {
			res.Events = events
		}

		if err := os.WriteFile(out, combat.MarshalPretty(res), 0644); err != nil {
			panic(err)
		}
		fmt.Printf("Single simsvc finished. Win=%v, T=%.2fs, DPS=%.1f -> %s\n", res.Win, res.Duration, res.DPS, out)
		return
	}

	type stat struct {
		Win     int
		SumT    float64
		SumDPS  float64
		BySkill map[string]float64
		ByHero  map[string]float64
	}
	var st = stat{
		BySkill: map[string]float64{},
		ByHero:  map[string]float64{},
	}
	var mu sync.Mutex
	wg := sync.WaitGroup{}
	workers := 8
	jobs := make(chan int, n)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := range jobs {
				env := &combat.Env{Rng: util.New(seed + int64(workerID)*7919 + int64(i))}
				boss := combat.NewBoss(bossID, bossCfg.MaxHP, bossCfg.GuardMax)
				party := combat.NewParty([]combat.Hero{
					{ID: "pyro_knight", Elem: "fire", Tags: map[string]bool{}},
					{ID: "aqua_mage", Elem: "water", Tags: map[string]bool{}},
					{ID: "storm_rogue", Elem: "storm", Tags: map[string]bool{}},
				}, skillBook)
				noop := func(ev combat.Event) {}
				rr := combat.NewReactionResolver(reacts, elems, func() float64 { return env.Time }, noop)
				ph := combat.NewBossPhaser(bossCfg, boss, noop)
				res := combat.RunSingle(env, boss, party, rr, ph, false)

				mu.Lock()
				if res.Win {
					st.Win++
				}
				st.SumT += res.Duration
				st.SumDPS += res.DPS
				for k, v := range res.DamageBySkill {
					st.BySkill[k] += v
				}
				for k, v := range res.DamageByHero {
					st.ByHero[k] += v
				}
				mu.Unlock()
			}
		}(w)
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	totalDmg := 0.0
	for _, v := range st.BySkill {
		totalDmg += v
	}

	percent := func(m map[string]float64) map[string]any {
		out := map[string]any{}
		for k, v := range m {
			share := 0.0
			if totalDmg > 0 {
				share = v / totalDmg
			}
			out[k] = map[string]any{"total": v, "ratio": share}
		}
		return out
	}

	summary := map[string]any{
		"runs":         n,
		"win_rate":     float64(st.Win) / float64(n),
		"avg_time":     st.SumT / float64(n),
		"avg_dps":      st.SumDPS / float64(n),
		"total_damage": totalDmg,
		"by_skill":     percent(st.BySkill),
		"by_hero":      percent(st.ByHero),
	}
	if err := os.WriteFile(out, combat.MarshalPretty(summary), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Batch %d done -> %s\n", n, filepath.Base(out))
}
