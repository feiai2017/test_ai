package combat

import (
	"fmt"
	"strings"
	"test_ai/internal/config"
)

type ReactionResolver struct {
	Table        []config.Reaction
	icd          map[string]float64 // targetID|reactionID -> nextAllowedTime
	statusToElem map[string]string
	Emit         func(ev Event)
	TimeNow      func() float64

	OnReaction       func(reactionID string, targetID string, sourceHeroID string)
	OnReactionDamage func(reactionID string, amount float64, sourceHeroID string)
	OnCooldownAdjust func(heroID string, amount float64, tags []string)
}

func NewReactionResolver(rc *config.ReactionsConfig, ec *config.ElementsConfig, now func() float64, emit func(Event)) *ReactionResolver {
	return &ReactionResolver{
		Table:        rc.Reactions,
		icd:          map[string]float64{},
		statusToElem: buildStatusElementIndex(ec),
		TimeNow:      now,
		Emit:         emit,
	}
}

func (rr *ReactionResolver) TryTrigger(atkElem string, target *Entity, sourceHeroID string) {
	now := rr.TimeNow()
	targetElems := rr.inferTargetElems(target)
	for _, r := range rr.Table {
		if !pairMatch(r.When, atkElem, targetElems) {
			continue
		}
		key := fmt.Sprintf("%s|%s", target.ID, r.ID)
		if next, ok := rr.icd[key]; ok && now < next {
			continue
		}
		rr.icd[key] = now + r.ICD

		rr.Emit(Event{T: now, Type: "Reaction", Payload: map[string]any{
			"id": r.ID, "atkElem": atkElem, "target": target.ID, "source": sourceHeroID,
		}})
		if rr.OnReaction != nil {
			rr.OnReaction(r.ID, target.ID, sourceHeroID)
		}

		for _, ef := range r.Effects {
			switch ef.Type {
			case "damage_amp_target":
				target.BuffDMGTakenMul *= (1.0 + ef.Value)
			case "apply_status":
				target.Statuses[ef.Status] = Status{Name: ef.Status, ExpireAt: now + ef.Duration}
			case "purge_status":
				for _, s := range ef.Statuses {
					delete(target.Statuses, s)
				}
			case "break_guard":
				if target.Guard > 0 {
					target.Guard -= int(ef.Value)
					if target.Guard < 0 {
						target.Guard = 0
					}
					rr.Emit(Event{T: now, Type: "GuardChanged", Payload: map[string]any{"guard": target.Guard}})
				}
			case "shred_resist":
				for k, v := range ef.Table {
					target.Resist[k] -= v
				}
			case "react_damage":
				if ef.Amount > 0 {
					amount := ef.Amount
					target.HP -= int(amount)
					if target.HP < 0 {
						target.HP = 0
					}
					rr.Emit(Event{T: now, Type: "ReactionDamage", Payload: map[string]any{
						"reaction": r.ID, "amount": int(amount), "target": target.ID, "source": sourceHeroID, "hp": target.HP,
					}})
					if rr.OnReactionDamage != nil {
						rr.OnReactionDamage(r.ID, amount, sourceHeroID)
					}
				}
			case "cdr_source":
				if rr.OnCooldownAdjust != nil && ef.Value != 0 {
					rr.OnCooldownAdjust(sourceHeroID, ef.Value, ef.Tags)
				}
			}
		}
	}
}

func (rr *ReactionResolver) inferTargetElems(t *Entity) []string {
	if rr.statusToElem == nil {
		return nil
	}
	var elems []string
	seen := map[string]bool{}
	for name := range t.Statuses {
		if elem, ok := rr.statusToElem[name]; ok && !seen[elem] {
			elems = append(elems, elem)
			seen[elem] = true
		}
	}
	return elems
}

func pairMatch(when []string, atk string, targetElems []string) bool {
	if len(when) != 2 {
		return false
	}
	a, b := when[0], when[1]
	has := func(x string) bool {
		for _, e := range targetElems {
			if e == x {
				return true
			}
		}
		return false
	}
	return (strings.EqualFold(atk, a) && has(b)) || (strings.EqualFold(atk, b) && has(a))
}

func buildStatusElementIndex(ec *config.ElementsConfig) map[string]string {
	index := map[string]string{
		"wet":       "water",
		"burning":   "fire",
		"frostbite": "frost",
		"frozen":    "frost",
	}
	if ec == nil {
		return index
	}
	for _, el := range ec.Elements {
		elemID := strings.ToLower(el.ID)
		if elemID == "" {
			continue
		}
		for _, tag := range el.TagsAdd {
			if strings.HasPrefix(tag, "apply.") {
				status := strings.TrimPrefix(tag, "apply.")
				if status != "" {
					index[status] = elemID
				}
			}
		}
		if el.Mods.Dot.ID != "" {
			index[el.Mods.Dot.ID] = elemID
		}
		if el.Mods.Debuff.ID != "" {
			index[el.Mods.Debuff.ID] = elemID
		}
	}
	return index
}
