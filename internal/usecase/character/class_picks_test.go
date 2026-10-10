package character_test

import (
	"context"
	"slices"
	"testing"

	domain "github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/rules"
)

// built is a character of one class at a level, with its subclass taken.
func built(t *testing.T, class, subclass rules.Slug, subclassAt, level int) *builder {
	t.Helper()
	// Through the registry, which is what files a subclass under its class:
	// the class rows name only the SRD's one each.
	b := spellBuilderForTest(t).
		add("desired level", domain.Event{Type: domain.EventChange, Changes: []domain.Change{{Path: "identity.desiredLevel", Op: domain.OpSet, Value: domain.IntValue(level)}}}).
		add("class", domain.Event{Type: domain.EventClass, Ref: ref(rules.RefClass, class), Level: 1})
	if subclass != "" {
		b.add("subclass", domain.Event{Type: domain.EventSubclass, Ref: ref(rules.RefSubclass, subclass), Level: subclassAt})
	}
	return b
}

// pick answers one prompt a class level poses.
func (b *builder) pick(class rules.Slug, level int, prompt rules.Slug, picks ...rules.Slug) *builder {
	b.t.Helper()
	return b.add(prompt.String(), levelAnswer(class, level, prompt, picks...))
}

func levelAnswer(class rules.Slug, level int, prompt rules.Slug, picks ...rules.Slug) domain.Event {
	return domain.Event{Type: domain.EventLevel, Ref: ref(rules.RefClass, class), Level: level, Choices: []domain.Answer{answer(prompt, picks...)}}
}

func (b *builder) prompts() []domain.Prompt {
	b.t.Helper()
	prompts, err := b.s.Prompts(context.Background(), testOwner, b.id, rules.DefaultLocale)
	if err != nil {
		b.t.Fatal(err)
	}
	return prompts
}

func (b *builder) prompt(id rules.Slug) domain.Prompt {
	b.t.Helper()
	for _, p := range b.prompts() {
		if p.Choice.Prompt == id {
			return p
		}
	}
	b.t.Fatalf("%s is not asked", id)
	return domain.Prompt{}
}

func (b *builder) features() []rules.Slug {
	b.t.Helper()
	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		b.t.Fatal(err)
	}
	state, err := domain.Project(b.log(), cat)
	if err != nil {
		b.t.Fatal(err)
	}
	return state.Features
}

// A Battle Master learns three maneuvers and two more at each of three later
// levels, and never the same one twice.
func TestBattleMasterLearnsManeuversAtEveryTier(t *testing.T) {
	t.Parallel()
	b := built(t, "fighter", "battle-master", 3, 15)
	for prompt, want := range map[rules.Slug]int{
		"maneuvers/subfeature/0":                          3,
		"additional-maneuvers/subfeature/0":               2,
		"battle-master-additional-maneuvers/subfeature/0": 2,
		"fighter-additional-maneuvers/subfeature/0":       2,
	} {
		if got := b.prompt(prompt).Choice.Choose; got != want {
			t.Errorf("%s chooses %d, want %d", prompt, got, want)
		}
	}
	b.pick("fighter", 3, "maneuvers/subfeature/0", "maneuver-parry", "maneuver-riposte", "maneuver-trip-attack")
	repeat := levelAnswer("fighter", 7, "additional-maneuvers/subfeature/0", "maneuver-parry", "maneuver-rally")
	if _, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, repeat); err == nil {
		t.Fatal("accepted a maneuver the fighter already knows")
	}
	b.pick("fighter", 7, "additional-maneuvers/subfeature/0", "maneuver-ambush", "maneuver-rally")
	features := b.features()
	for _, want := range []rules.Slug{"maneuver-parry", "maneuver-riposte", "maneuver-trip-attack", "maneuver-ambush", "maneuver-rally"} {
		if !slices.Contains(features, want) {
			t.Errorf("missing %s", want)
		}
	}
}

// The totem animal is chosen three times and independently; before, every
// animal was granted at every tier.
func TestTotemWarriorHoldsOnlyTheChosenAnimals(t *testing.T) {
	t.Parallel()
	b := built(t, "barbarian", "totem-warrior", 3, 14).
		pick("barbarian", 3, "totem-spirit/subfeature/0", "bear").
		pick("barbarian", 6, "aspect-of-the-beast/subfeature/0", "totem-warrior-eagle").
		pick("barbarian", 14, "totemic-attunement/subfeature/0", "barbarian-wolf")
	features := b.features()
	var held []rules.Slug
	for _, animal := range []rules.Slug{"bear", "eagle", "elk", "tiger", "wolf"} {
		for _, slug := range []rules.Slug{animal, "totem-warrior-" + animal, "barbarian-" + animal} {
			if slices.Contains(features, slug) {
				held = append(held, slug)
			}
		}
	}
	if want := []rules.Slug{"bear", "totem-warrior-eagle", "barbarian-wolf"}; !slices.Equal(held, want) {
		t.Fatalf("holds %v, want %v", held, want)
	}
}

// A Storm Herald chooses an environment once, and the later tiers follow it.
func TestStormHeraldTiersFollowTheEnvironment(t *testing.T) {
	t.Parallel()
	b := built(t, "barbarian", "storm-herald", 3, 14).pick("barbarian", 3, "storm-aura/subfeature/0", "sea")
	features := b.features()
	for _, want := range []rules.Slug{"sea", "storm-herald-sea", "barbarian-sea"} {
		if !slices.Contains(features, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, other := range []rules.Slug{"desert", "storm-herald-desert", "barbarian-tundra"} {
		if slices.Contains(features, other) {
			t.Errorf("holds %s without choosing it", other)
		}
	}
}

func TestCircleOfTheLandAsksForItsTerrain(t *testing.T) {
	t.Parallel()
	b := built(t, "druid", "land", 2, 2)
	if !hasOpenPrompt(b.prompts(), "circle-of-the-land/subfeature/0") {
		t.Fatal("a Land druid is not asked for a terrain")
	}
}

// An invocation's prerequisites are read against the character as they stand:
// the level they have reached and the pact they made.
func TestInvocationPrerequisitesBlockOptions(t *testing.T) {
	t.Parallel()
	const (
		thirsting = rules.Slug("eldritch-invocation-thirsting-blade") // 5th level, Pact of the Blade
		lifedrink = rules.Slug("eldritch-invocation-lifedrinker")     // 12th level, Pact of the Blade
		secrets   = rules.Slug("eldritch-invocation-book-of-ancient-secrets")
		sight     = rules.Slug("eldritch-invocation-devils-sight")
	)
	b := built(t, "warlock", "fiend", 1, 5).pick("warlock", 3, "pact-boon/subfeature/0", "pact-of-the-blade")
	blocked := b.prompt("eldritch-invocations-5/subfeature/0").Blocked
	if slices.Contains(blocked, thirsting) || slices.Contains(blocked, sight) {
		t.Errorf("blocked an invocation the warlock qualifies for: %v", blocked)
	}
	for _, want := range []rules.Slug{lifedrink, secrets} {
		if !slices.Contains(blocked, want) {
			t.Errorf("%s is offered to a fifth-level blade warlock", want)
		}
	}
	refused := levelAnswer("warlock", 5, "eldritch-invocations-5/subfeature/0", lifedrink)
	if _, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, refused); err == nil {
		t.Fatal("accepted an invocation whose prerequisite is not met")
	}
	b.pick("warlock", 5, "eldritch-invocations-5/subfeature/0", thirsting)

	// At twelfth level the pick second level opened may hold Lifedrinker:
	// that is the whole of "replace an invocation when you gain a level".
	b = built(t, "warlock", "fiend", 1, 12).pick("warlock", 3, "pact-boon/subfeature/0", "pact-of-the-blade")
	if slices.Contains(b.prompt("eldritch-invocations/subfeature/0").Blocked, lifedrink) {
		t.Fatal("Lifedrinker is blocked at twelfth level")
	}
	b.pick("warlock", 2, "eldritch-invocations/subfeature/0", lifedrink, sight)
}

func TestArtificerLearnsInfusions(t *testing.T) {
	t.Parallel()
	b := built(t, "artificer", "", 0, 6)
	if got := b.prompt("infusions-known/subfeature/0").Choice.Choose; got != 4 {
		t.Errorf("second level infuses %d, want 4", got)
	}
	p := b.prompt("infusions-known-6/subfeature/0")
	if p.Choice.Choose != 2 || slices.Contains(p.Blocked, "infusion-radiant-weapon") || !slices.Contains(p.Blocked, "infusion-helm-of-awareness") {
		t.Errorf("sixth level: choose %d, blocked %v", p.Choice.Choose, p.Blocked)
	}
}

func (b *builder) state() domain.State {
	b.t.Helper()
	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		b.t.Fatal(err)
	}
	state, err := domain.Project(b.log(), cat)
	if err != nil {
		b.t.Fatal(err)
	}
	return state
}

// rule answers a question a pack rule poses.
func (b *builder) rule(id rules.Slug, prompt rules.Slug, picks ...rules.Slug) *builder {
	b.t.Helper()
	return b.add(prompt.String(), domain.Event{Type: domain.EventRule, Ref: ref(rules.RefRule, id), Choices: []domain.Answer{answer(prompt, picks...)}})
}

// feat takes a feat with a fighter's fourth-level improvement.
func withFeat(t *testing.T, feat rules.Slug) *builder {
	t.Helper()
	improvement := rules.Slug("fighter/ability-score-improvement/4")
	return built(t, "fighter", "champion", 3, 4).
		pick("fighter", 4, improvement, "feat").
		pick("fighter", 4, improvement+"/1", feat)
}

// A rule's question is filed with what owns the rule, at the level that
// brought it: a background's with the background, a feature's with its level.
func TestRuleQuestionsAreFiledWithTheirOwner(t *testing.T) {
	t.Parallel()
	b := built(t, "fighter", "battle-master", 3, 3).
		add("criminal", domain.Event{Type: domain.EventBackground, Ref: ref(rules.RefBackground, "criminal")})
	if p := b.prompt("criminal/tool/0"); p.Group != domain.GroupBackground {
		t.Errorf("the criminal's gaming set is asked under %s", p.Group)
	}
	if p := b.prompt("student-of-war/proficiency/0"); p.Group != domain.GroupClass || p.Level != 3 {
		t.Errorf("Student of War is asked under %s at level %d", p.Group, p.Level)
	}
	b.rule("criminal-tool", "criminal/tool/0", "dice-set").rule("student-of-war", "student-of-war/proficiency/0", "smiths-tools")
	for _, want := range []rules.Slug{"dice-set", "smiths-tools"} {
		if !slices.Contains(b.state().Proficiencies, want) {
			t.Errorf("not proficient with %s", want)
		}
	}
}

// Skill Expert is a proficiency and an expertise, and the expertise may only
// double a skill the character is proficient in.
func TestSkillExpertDoublesAHeldSkill(t *testing.T) {
	t.Parallel()
	b := withFeat(t, "skill-expert").rule("skill-expert", "skill-expert/proficiency/0", "skill-stealth")
	unheld := domain.Event{Type: domain.EventRule, Ref: ref(rules.RefRule, "skill-expert"), Choices: []domain.Answer{answer("skill-expert/expertise/0", "skill-arcana")}}
	if _, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, unheld); err == nil {
		t.Fatal("accepted expertise in a skill the character is not proficient in")
	}
	b.rule("skill-expert", "skill-expert/expertise/0", "skill-stealth")
	if got := b.state().Skills.BySkill["stealth"].Proficiency; got != rules.Expertise {
		t.Fatalf("stealth is %v, want expertise", got)
	}
}

// Resilient is one pick with two halves: the score and its saving throw.
func TestResilientGrantsTheSaveOfItsAbility(t *testing.T) {
	t.Parallel()
	b := withFeat(t, "resilient")
	before := b.state().Abilities.Scores[rules.Wisdom]
	b.rule("resilient-asi", "resilient/ability/0", "wis")
	state := b.state()
	if state.Abilities.Scores[rules.Wisdom] != before+1 {
		t.Errorf("wisdom %d, want %d", state.Abilities.Scores[rules.Wisdom], before+1)
	}
	if !state.SavingThrows.ByAbility[rules.Wisdom].Proficient {
		t.Error("not proficient in Wisdom saves")
	}
}

// A Divine Soul learns one spell by the affinity of their magic, not all five.
func TestDivineSoulChoosesOneAffinitySpell(t *testing.T) {
	t.Parallel()
	b := built(t, "sorcerer", "divine-soul", 1, 1).rule("divine-magic", "divine-magic/spell/0", "bless")
	known := b.state().Spells.Known
	if !slices.Contains(known, "bless") || slices.Contains(known, "bane") {
		t.Fatalf("knows %v, want bless and not bane", known)
	}
}

func TestBlessingsOfKnowledgeAsksItsThreeQuestions(t *testing.T) {
	t.Parallel()
	b := built(t, "cleric", "knowledge", 1, 1)
	for _, prompt := range []rules.Slug{"blessings-of-knowledge/language/0", "blessings-of-knowledge/proficiency/0", "blessings-of-knowledge/expertise/0"} {
		if got := b.prompt(prompt).Choice.Choose; got != 2 {
			t.Errorf("%s chooses %d, want 2", prompt, got)
		}
	}
	b.rule("blessings-of-knowledge", "blessings-of-knowledge/proficiency/0", "skill-arcana", "skill-history").
		rule("blessings-of-knowledge", "blessings-of-knowledge/expertise/0", "skill-arcana", "skill-history")
	if got := b.state().Skills.BySkill["arcana"].Proficiency; got != rules.Expertise {
		t.Fatalf("arcana is %v, want expertise", got)
	}
}

// A favored enemy is held like any feature: the sheet can name it, a later
// tier cannot choose it again, and a kind that speaks brings a language.
func TestFavoredEnemyIsHeldAndNotRepeated(t *testing.T) {
	t.Parallel()
	b := built(t, "ranger", "hunter", 3, 6).pick("ranger", 1, "favored-enemy-1-type/enemy-type/0", "favored-enemy-dragons")
	if !slices.Contains(b.features(), "favored-enemy-dragons") {
		t.Fatal("the favored enemy is not on the character")
	}
	again := levelAnswer("ranger", 6, "favored-enemy-2-types/enemy-type/0", "favored-enemy-dragons")
	if _, err := b.s.Apply(context.Background(), testOwner, b.id, rules.DefaultLocale, b.seq, again); err == nil {
		t.Fatal("accepted the same favored enemy twice")
	}
	b.pick("ranger", 6, "favored-enemy-2-types/enemy-type/0", "favored-enemy-beasts")
	prompts := b.prompts()
	if !hasOpenPrompt(prompts, "favored-enemy-dragons/language/0") || hasOpenPrompt(prompts, "favored-enemy-beasts/language/0") {
		t.Fatal("dragons bring a language and beasts do not")
	}
}

// A variant human's feat is a rule's question, and the feat asks its own.
func TestVariantHumanTakesAFeat(t *testing.T) {
	t.Parallel()
	b := spellBuilderForTest(t).add("race", domain.Event{Type: domain.EventRace, Ref: ref(rules.RefRace, "variant-human")})
	if p := b.prompt("variant-human/feat/0"); p.Group != domain.GroupRace {
		t.Errorf("the feat is asked under %s", p.Group)
	}
	b.rule("variant-human-feat", "variant-human/feat/0", "skilled")
	if !slices.Contains(b.state().Feats, "skilled") || !hasOpenPrompt(b.prompts(), "skilled/proficiency/0") {
		t.Fatal("the feat was not taken, or did not ask for its three proficiencies")
	}
	for _, prompt := range []rules.Slug{"variant-human/ability-bonus/0", "variant-human-skills/proficiency/0", "variant-human/language/0"} {
		if !hasOpenPrompt(b.prompts(), prompt) {
			t.Errorf("%s is not asked", prompt)
		}
	}
}

// Magic Initiate names a class and then teaches from its list -- without
// making the character one.
func TestMagicInitiateTeachesFromTheChosenClass(t *testing.T) {
	t.Parallel()
	const cantrips = rules.Slug("magic-initiate-cantrips/spell/cantrip")
	b := withFeat(t, "magic-initiate")
	if hasOpenPrompt(b.prompts(), cantrips) {
		t.Fatal("cantrips are offered before a class is named")
	}
	b.rule("magic-initiate-class", "magic-initiate/class/0", "wizard")
	p := b.prompt(cantrips)
	pool := rules.OptionKeys(p.Choice.From)
	if p.Choice.Choose != 2 || !slices.Contains(pool, "fire-bolt") || slices.Contains(pool, "sacred-flame") {
		t.Fatalf("choose %d from a pool that is not the wizard's cantrips", p.Choice.Choose)
	}
	feat := ref(rules.RefFeat, "magic-initiate")
	b.add("cantrips", domain.Event{Type: domain.EventFeat, Ref: feat, Choices: []domain.Answer{answer(cantrips, "fire-bolt", "light")}}).
		add("spell", domain.Event{Type: domain.EventFeat, Ref: feat, Choices: []domain.Answer{answer("magic-initiate-spell/spell/known", "shield")}})
	state := b.state()
	if len(state.Identity.Classes) != 1 || state.Identity.Level() != 4 {
		t.Fatalf("the feat changed the character's classes: %+v", state.Identity.Classes)
	}
	if !slices.Contains(state.Spells.Cantrips, "fire-bolt") || !slices.Contains(state.Spells.Known, "shield") {
		t.Fatalf("spells %+v", state.Spells)
	}
	for _, source := range state.Spells.Sources {
		if source.Source == feat && source.Ability != rules.Intelligence {
			t.Errorf("cast with %q, want the wizard's Intelligence", source.Ability)
		}
	}
}

func TestFeyTouchedOffersItsTwoSchools(t *testing.T) {
	t.Parallel()
	b := withFeat(t, "fey-touched")
	if !slices.Contains(b.state().Spells.Known, "misty-step") {
		t.Error("Misty Step is not known")
	}
	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	pool := rules.OptionKeys(b.prompt("fey-touched-spell/spell/known").Choice.From)
	if len(pool) == 0 {
		t.Fatal("nothing is offered")
	}
	for _, slug := range pool {
		if spell, _ := cat.Spells.Get(slug); spell.Level != 1 || spell.School != "divination" && spell.School != "enchantment" {
			t.Errorf("%s is offered: level %d, %s", slug, spell.Level, spell.School)
		}
	}
}

func TestNatureClericLearnsADruidCantrip(t *testing.T) {
	t.Parallel()
	b := built(t, "cleric", "nature", 1, 1)
	pool := rules.OptionKeys(b.prompt("acolyte-of-nature/spell/cantrip").Choice.From)
	if !slices.Contains(pool, "druidcraft") || slices.Contains(pool, "sacred-flame") {
		t.Fatal("the pool is not the druid's cantrips")
	}
	b.pick("cleric", 1, "acolyte-of-nature/spell/cantrip", "druidcraft")
	if !slices.Contains(b.state().Spells.Cantrips, "druidcraft") {
		t.Fatal("the cantrip was not learned")
	}
}

// The sheet says where a feature came from, and the nearest source wins: a
// maneuver names the pick it was made in, not the subclass behind that.
func TestOriginsNameTheNearestSource(t *testing.T) {
	t.Parallel()
	b := built(t, "fighter", "battle-master", 3, 3).
		add("criminal", domain.Event{Type: domain.EventBackground, Ref: ref(rules.RefBackground, "criminal")}).
		pick("fighter", 3, "maneuvers/subfeature/0", "maneuver-parry", "maneuver-riposte", "maneuver-rally")
	cat, err := b.s.Catalog(context.Background(), rules.DefaultLocale)
	if err != nil {
		t.Fatal(err)
	}
	origins := domain.Origins(b.state(), cat)
	for feature, want := range map[rules.Slug]rules.Ref{
		"maneuver-parry":            ref(rules.RefFeature, "maneuvers"),
		"maneuvers":                 ref(rules.RefSubclass, "battle-master"),
		"second-wind":               ref(rules.RefClass, "fighter"),
		"criminal-criminal-contact": ref(rules.RefBackground, "criminal"),
	} {
		if got := origins[ref(rules.RefFeature, feature)]; got != want {
			t.Errorf("%s came from %v, want %v", feature, got, want)
		}
	}
}
