package core

import (
	"fmt"
	"time"

	"github.com/wowsims/wotlk/sim/core/stats"
)

type ApplySpellResults func(sim *Simulation, target *Unit, spell *Spell)
type DynamicThreatBonusFunc func(result *SpellResult, spell *Spell) float64
type DynamicThreatMultiplierFunc func(result *SpellResult, spell *Spell) float64

type SpellConfig struct {
	// See definition of Spell (below) for comments on these.
	ActionID
	SpellSchool  SpellSchool
	ProcMask     ProcMask
	Flags        SpellFlag
	MissileSpeed float64
	ResourceType stats.Stat
	BaseCost     float64

	Cast CastConfig

	ApplyEffects ApplySpellResults

	BonusHitRating       float64
	BonusCritRating      float64
	BonusSpellPower      float64
	BonusExpertiseRating float64
	BonusArmorPenRating  float64

	DamageMultiplier         float64
	DamageMultiplierAdditive float64
	CritMultiplier           float64

	ThreatMultiplier        float64
	DynamicThreatMultiplier DynamicThreatMultiplierFunc

	FlatThreatBonus    float64
	DynamicThreatBonus DynamicThreatBonusFunc
}

// Metric totals for a spell against a specific target, for the current iteration.
type SpellMetrics struct {
	Casts              int32
	Misses             int32
	Hits               int32
	Crits              int32
	Crushes            int32
	Dodges             int32
	Glances            int32
	Parries            int32
	Blocks             int32
	PartialResists_1_4 int32   // 1/4 of the spell was resisted
	PartialResists_2_4 int32   // 2/4 of the spell was resisted
	PartialResists_3_4 int32   // 3/4 of the spell was resisted
	TotalDamage        float64 // Damage done by all casts of this spell.
	TotalThreat        float64 // Threat generated by all casts of this spell.
	TotalHealing       float64 // Healing done by all casts of this spell.
	TotalShielding     float64 // Shielding done by all casts of this spell.
	TotalCastTime      time.Duration
}

type Spell struct {
	// ID for this spell.
	ActionID

	// The unit who will perform this spell.
	Unit *Unit

	// Fire, Frost, Shadow, etc.
	SpellSchool SpellSchool

	// Controls which effects can proc from this spell.
	ProcMask ProcMask

	// Flags
	Flags SpellFlag

	// Speed in yards/second. Spell missile speeds can be found in the game data.
	// Example: https://wow.tools/dbc/?dbc=spellmisc&build=3.4.0.44996
	MissileSpeed float64

	// Should be stats.Mana, stats.Energy, stats.Rage, or unset.
	ResourceType      stats.Stat
	ResourceMetrics   *ResourceMetrics
	comboPointMetrics *ResourceMetrics
	runicPowerMetrics *ResourceMetrics
	bloodRuneMetrics  *ResourceMetrics
	frostRuneMetrics  *ResourceMetrics
	unholyRuneMetrics *ResourceMetrics
	deathRuneMetrics  *ResourceMetrics
	healthMetrics     []*ResourceMetrics

	// Base cost. Many effects in the game which 'reduce mana cost by X%'
	// are calculated using the base cost.
	BaseCost float64

	// Default cast parameters with all static effects applied.
	DefaultCast Cast

	CD       Cooldown
	SharedCD Cooldown

	// Performs a cast of this spell.
	castFn CastSuccessFunc

	SpellMetrics []SpellMetrics

	ApplyEffects ApplySpellResults

	// The current or most recent cast data.
	CurCast Cast

	BonusHitRating           float64
	BonusCritRating          float64
	BonusSpellPower          float64
	BonusExpertiseRating     float64
	BonusArmorPenRating      float64
	CastTimeMultiplier       float64
	CostMultiplier           float64
	DamageMultiplier         float64
	DamageMultiplierAdditive float64
	CritMultiplier           float64

	// Multiplier for all threat generated by this effect.
	ThreatMultiplier float64

	// Dynamic multiplier for all threat generated by this effect.
	DynamicThreatMultiplier DynamicThreatMultiplierFunc

	// Adds a fixed amount of threat to this spell, before multipliers.
	FlatThreatBonus float64

	// Adds a dynamic amount of threat to this spell, before multipliers.
	DynamicThreatBonus DynamicThreatBonusFunc

	initialBonusHitRating           float64
	initialBonusCritRating          float64
	initialBonusSpellPower          float64
	initialDamageMultiplier         float64
	initialDamageMultiplierAdditive float64
	initialCritMultiplier           float64
	initialThreatMultiplier         float64
	// Note that bonus expertise and armor pen are static, so we don't bother resetting them.

	resultCache SpellResult
}

func (unit *Unit) OnSpellRegistered(handler SpellRegisteredHandler) {
	for _, spell := range unit.Spellbook {
		handler(spell)
	}
	unit.spellRegistrationHandlers = append(unit.spellRegistrationHandlers, handler)
}

// Registers a new spell to the unit. Returns the newly created spell.
func (unit *Unit) RegisterSpell(config SpellConfig) *Spell {
	if len(unit.Spellbook) > 100 {
		panic(fmt.Sprintf("Over 100 registered spells when registering %s! There is probably a spell being registered every iteration.", config.ActionID))
	}

	// Default the other damage multiplier to 1 if only one or the other is set.
	if config.DamageMultiplier != 0 && config.DamageMultiplierAdditive == 0 {
		config.DamageMultiplierAdditive = 1
	}
	if config.DamageMultiplierAdditive != 0 && config.DamageMultiplier == 0 {
		config.DamageMultiplier = 1
	}

	spell := &Spell{
		ActionID:     config.ActionID,
		Unit:         unit,
		SpellSchool:  config.SpellSchool,
		ProcMask:     config.ProcMask,
		Flags:        config.Flags,
		MissileSpeed: config.MissileSpeed,
		ResourceType: config.ResourceType,
		BaseCost:     config.BaseCost,

		DefaultCast: config.Cast.DefaultCast,
		CD:          config.Cast.CD,
		SharedCD:    config.Cast.SharedCD,

		ApplyEffects: config.ApplyEffects,

		BonusHitRating:           config.BonusHitRating,
		BonusCritRating:          config.BonusCritRating,
		BonusSpellPower:          config.BonusSpellPower,
		BonusExpertiseRating:     config.BonusExpertiseRating,
		BonusArmorPenRating:      config.BonusArmorPenRating,
		CastTimeMultiplier:       1,
		CostMultiplier:           1,
		DamageMultiplier:         config.DamageMultiplier,
		DamageMultiplierAdditive: config.DamageMultiplierAdditive,
		CritMultiplier:           config.CritMultiplier,

		ThreatMultiplier:   config.ThreatMultiplier,
		FlatThreatBonus:    config.FlatThreatBonus,
		DynamicThreatBonus: config.DynamicThreatBonus,
	}

	if (spell.DamageMultiplier != 0 || spell.ThreatMultiplier != 0) && spell.ProcMask == ProcMaskUnknown {
		panic("Unknown proc mask on " + spell.ActionID.String())
	}

	switch spell.ResourceType {
	case stats.Mana:
		spell.ResourceMetrics = spell.Unit.NewManaMetrics(spell.ActionID)
	case stats.Rage:
		spell.ResourceMetrics = spell.Unit.NewRageMetrics(spell.ActionID)
	case stats.Energy:
		spell.ResourceMetrics = spell.Unit.NewEnergyMetrics(spell.ActionID)
	case stats.RunicPower:
		spell.ResourceMetrics = spell.Unit.NewRunicPowerMetrics(spell.ActionID)
	case stats.BloodRune:
		spell.ResourceMetrics = spell.Unit.NewBloodRuneMetrics(spell.ActionID)
	case stats.FrostRune:
		spell.ResourceMetrics = spell.Unit.NewFrostRuneMetrics(spell.ActionID)
	case stats.UnholyRune:
		spell.ResourceMetrics = spell.Unit.NewUnholyRuneMetrics(spell.ActionID)
	case stats.DeathRune:
		spell.ResourceMetrics = spell.Unit.NewDeathRuneMetrics(spell.ActionID)
	}

	if spell.ResourceType != 0 && spell.DefaultCast.Cost == 0 {
		panic("ResourceType set for spell " + spell.ActionID.String() + " but no cost")
	}

	if spell.ResourceType == 0 && spell.DefaultCast.Cost != 0 {
		panic("Cost set for spell " + spell.ActionID.String() + " but no ResourceType")
	}

	spell.castFn = spell.makeCastFunc(config.Cast, spell.applyEffects)

	if spell.ApplyEffects == nil {
		spell.ApplyEffects = func(*Simulation, *Unit, *Spell) {}
	}

	unit.Spellbook = append(unit.Spellbook, spell)

	for _, handler := range unit.spellRegistrationHandlers {
		handler(spell)
	}

	if unit.Env != nil && unit.Env.IsFinalized() {
		spell.finalize()
	}

	return spell
}

// Returns the first registered spell with the given ID, or nil if there are none.
func (unit *Unit) GetSpell(actionID ActionID) *Spell {
	for _, spell := range unit.Spellbook {
		if spell.ActionID.SameAction(actionID) {
			return spell
		}
	}
	return nil
}

// Retrieves an existing spell with the same ID as the config uses, or registers it if there is none.
func (unit *Unit) GetOrRegisterSpell(config SpellConfig) *Spell {
	registered := unit.GetSpell(config.ActionID)
	if registered == nil {
		return unit.RegisterSpell(config)
	} else {
		return registered
	}
}

// Metrics for the current iteration
func (spell *Spell) CurDamagePerCast() float64 {
	if spell.SpellMetrics[0].Casts == 0 {
		return 0
	} else {
		casts := int32(0)
		damage := 0.0
		for _, opponent := range spell.Unit.GetOpponents() {
			casts += spell.SpellMetrics[opponent.UnitIndex].Casts
			damage += spell.SpellMetrics[opponent.UnitIndex].TotalDamage
		}
		return damage / float64(casts)
	}
}

func (spell *Spell) finalize() {
	// Assert that user doesn't set dynamic fields during static initialization.
	if spell.CastTimeMultiplier != 1 {
		panic(spell.ActionID.String() + " has non-default CastTimeMultiplier during finalize!")
	}
	if spell.CostMultiplier != 1 {
		panic(spell.ActionID.String() + " has non-default CostMultiplier during finalize!")
	}
	spell.initialBonusHitRating = spell.BonusHitRating
	spell.initialBonusCritRating = spell.BonusCritRating
	spell.initialBonusSpellPower = spell.BonusSpellPower
	spell.initialDamageMultiplier = spell.DamageMultiplier
	spell.initialDamageMultiplierAdditive = spell.DamageMultiplierAdditive
	spell.initialCritMultiplier = spell.CritMultiplier
	spell.initialThreatMultiplier = spell.ThreatMultiplier
}

func (spell *Spell) reset(_ *Simulation) {
	if len(spell.SpellMetrics) != len(spell.Unit.Env.AllUnits) {
		spell.SpellMetrics = make([]SpellMetrics, len(spell.Unit.Env.AllUnits))
	} else {
		for i := range spell.SpellMetrics {
			spell.SpellMetrics[i] = SpellMetrics{}
		}
	}

	// Reset dynamic effects.
	spell.BonusHitRating = spell.initialBonusHitRating
	spell.BonusCritRating = spell.initialBonusCritRating
	spell.BonusSpellPower = spell.initialBonusSpellPower
	spell.CastTimeMultiplier = 1
	spell.CostMultiplier = 1
	spell.DamageMultiplier = spell.initialDamageMultiplier
	spell.DamageMultiplierAdditive = spell.initialDamageMultiplierAdditive
	spell.CritMultiplier = spell.initialCritMultiplier
	spell.ThreatMultiplier = spell.initialThreatMultiplier
}

func (spell *Spell) doneIteration() {
	if !spell.Flags.Matches(SpellFlagNoMetrics) {
		spell.Unit.Metrics.addSpell(spell)
	}
}

func (spell *Spell) ComboPointMetrics() *ResourceMetrics {
	if spell.comboPointMetrics == nil {
		spell.comboPointMetrics = spell.Unit.NewComboPointMetrics(spell.ActionID)
	}
	return spell.comboPointMetrics
}

func (spell *Spell) RunicPowerMetrics() *ResourceMetrics {
	if spell.runicPowerMetrics == nil {
		spell.runicPowerMetrics = spell.Unit.NewRunicPowerMetrics(spell.ActionID)
	}
	return spell.runicPowerMetrics
}

func (spell *Spell) BloodRuneMetrics() *ResourceMetrics {
	if spell.bloodRuneMetrics == nil {
		spell.bloodRuneMetrics = spell.Unit.NewBloodRuneMetrics(spell.ActionID)
	}
	return spell.bloodRuneMetrics
}

func (spell *Spell) FrostRuneMetrics() *ResourceMetrics {
	if spell.frostRuneMetrics == nil {
		spell.frostRuneMetrics = spell.Unit.NewFrostRuneMetrics(spell.ActionID)
	}
	return spell.frostRuneMetrics
}

func (spell *Spell) UnholyRuneMetrics() *ResourceMetrics {
	if spell.unholyRuneMetrics == nil {
		spell.unholyRuneMetrics = spell.Unit.NewUnholyRuneMetrics(spell.ActionID)
	}
	return spell.unholyRuneMetrics
}

func (spell *Spell) DeathRuneMetrics() *ResourceMetrics {
	if spell.deathRuneMetrics == nil {
		spell.deathRuneMetrics = spell.Unit.NewDeathRuneMetrics(spell.ActionID)
	}
	return spell.deathRuneMetrics
}

func (spell *Spell) HealthMetrics(target *Unit) *ResourceMetrics {
	if spell.healthMetrics == nil {
		spell.healthMetrics = make([]*ResourceMetrics, len(spell.Unit.AttackTables))
	}
	if spell.healthMetrics[target.UnitIndex] == nil {
		spell.healthMetrics[target.UnitIndex] = target.NewHealthMetrics(spell.ActionID)
	}
	return spell.healthMetrics[target.UnitIndex]
}

func (spell *Spell) ReadyAt() time.Duration {
	return BothTimersReadyAt(spell.CD.Timer, spell.SharedCD.Timer)
}

func (spell *Spell) IsReady(sim *Simulation) bool {
	if spell == nil {
		return false
	}
	return BothTimersReady(spell.CD.Timer, spell.SharedCD.Timer, sim)
}

func (spell *Spell) TimeToReady(sim *Simulation) time.Duration {
	return MaxTimeToReady(spell.CD.Timer, spell.SharedCD.Timer, sim)
}

func (spell *Spell) Cast(sim *Simulation, target *Unit) bool {
	if target == nil {
		target = spell.Unit.CurrentTarget
	}
	return spell.castFn(sim, target)
}

// Skips the actual cast and applies spell effects immediately.
func (spell *Spell) SkipCastAndApplyEffects(sim *Simulation, target *Unit) {
	if sim.Log != nil && !spell.Flags.Matches(SpellFlagNoLogs) {
		spell.Unit.Log(sim, "Casting %s (Cost = %0.03f, Cast Time = %s)",
			spell.ActionID, spell.DefaultCast.Cost, time.Duration(0))
		spell.Unit.Log(sim, "Completed cast %s", spell.ActionID)
	}
	spell.applyEffects(sim, target)
}

func (spell *Spell) applyEffects(sim *Simulation, target *Unit) {
	if spell.SpellMetrics == nil {
		spell.reset(sim)
	}
	if target == nil {
		target = spell.Unit.CurrentTarget
	}
	spell.SpellMetrics[target.UnitIndex].Casts++
	spell.ApplyEffects(sim, target, spell)
}

func (spell *Spell) ApplyAOEThreatIgnoreMultipliers(threatAmount float64) {
	numTargets := spell.Unit.Env.GetNumTargets()
	for i := int32(0); i < numTargets; i++ {
		spell.SpellMetrics[i].TotalThreat += threatAmount
	}
}
func (spell *Spell) ApplyAOEThreat(threatAmount float64) {
	spell.ApplyAOEThreatIgnoreMultipliers(threatAmount * spell.Unit.PseudoStats.ThreatMultiplier)
}
