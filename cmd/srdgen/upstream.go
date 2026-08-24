package main

import "encoding/json"

// The 5e-bits/5e-database shapes, transcribed from the Zod schemas vendored at
// docs/reference_srd_5.1/data/5e-database-2014-en/schemas/.
//
// Those schemas are the authoritative spec for optionality, which is why every
// field the schema marks .optional() is a pointer or a slice here. Guessing is
// how a level-up calculator ends up reading "this cantrip has no scaling" as
// "this cantrip does zero damage".

// apiRef is the {index, name, url} triple upstream uses for every reference.
type apiRef struct {
	Index string `json:"index"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

type upAbilityScore struct {
	Index    string   `json:"index"`
	Name     string   `json:"name"`
	FullName string   `json:"full_name"`
	Desc     textList `json:"desc"`
	Skills   []apiRef `json:"skills"`
}

type upSkill struct {
	Index        string   `json:"index"`
	Name         string   `json:"name"`
	Desc         textList `json:"desc"`
	AbilityScore apiRef   `json:"ability_score"`
}

type upAlignment struct {
	Index        string   `json:"index"`
	Name         string   `json:"name"`
	Abbreviation string   `json:"abbreviation"`
	Desc         textList `json:"desc"`
}

type upNamed struct {
	Index string   `json:"index"`
	Name  string   `json:"name"`
	Desc  textList `json:"desc"`
}

type upLanguage struct {
	Index           string   `json:"index"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	TypicalSpeakers []string `json:"typical_speakers"`
	Script          string   `json:"script"`
	Desc            textList `json:"desc"`
}

type upProficiency struct {
	Index     string   `json:"index"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Classes   []apiRef `json:"classes"`
	Races     []apiRef `json:"races"`
	Reference *apiRef  `json:"reference"`
}

type upEquipmentCategory struct {
	Index     string   `json:"index"`
	Name      string   `json:"name"`
	Equipment []apiRef `json:"equipment"`
}

type upAbilityBonus struct {
	AbilityScore apiRef `json:"ability_score"`
	Bonus        int    `json:"bonus"`
}

type upRace struct {
	Index                 string           `json:"index"`
	Name                  string           `json:"name"`
	Speed                 int              `json:"speed"`
	AbilityBonuses        []upAbilityBonus `json:"ability_bonuses"`
	AbilityBonusOptions   *upChoice        `json:"ability_bonus_options"`
	Alignment             string           `json:"alignment"`
	Age                   string           `json:"age"`
	Size                  string           `json:"size"`
	SizeDescription       string           `json:"size_description"`
	StartingProficiencies []apiRef         `json:"starting_proficiencies"`
	StartingProfOptions   *upChoice        `json:"starting_proficiency_options"`
	Languages             []apiRef         `json:"languages"`
	LanguageDesc          string           `json:"language_desc"`
	LanguageOptions       *upChoice        `json:"language_options"`
	Traits                []apiRef         `json:"traits"`
	Subraces              []apiRef         `json:"subraces"`
}

type upSubrace struct {
	Index                 string           `json:"index"`
	Name                  string           `json:"name"`
	Race                  apiRef           `json:"race"`
	Desc                  textList         `json:"desc"`
	AbilityBonuses        []upAbilityBonus `json:"ability_bonuses"`
	StartingProficiencies []apiRef         `json:"starting_proficiencies"`
	LanguageOptions       *upChoice        `json:"language_options"`
	RacialTraits          []apiRef         `json:"racial_traits"`
}

type upTrait struct {
	Index              string    `json:"index"`
	Name               string    `json:"name"`
	Desc               []string  `json:"desc"`
	Races              []apiRef  `json:"races"`
	Subraces           []apiRef  `json:"subraces"`
	Proficiencies      []apiRef  `json:"proficiencies"`
	ProficiencyChoices *upChoice `json:"proficiency_choices"`
	LanguageOptions    *upChoice `json:"language_options"`
	TraitSpecific      *struct {
		BreathWeapon    json.RawMessage `json:"breath_weapon"`
		SubtraitOptions *upChoice       `json:"subtrait_options"`
		SpellOptions    *upChoice       `json:"spell_options"`
		DamageType      *apiRef         `json:"damage_type"`
	} `json:"trait_specific"`
}

type upClass struct {
	Index                    string     `json:"index"`
	Name                     string     `json:"name"`
	HitDie                   int        `json:"hit_die"`
	ProficiencyChoices       []upChoice `json:"proficiency_choices"`
	Proficiencies            []apiRef   `json:"proficiencies"`
	SavingThrows             []apiRef   `json:"saving_throws"`
	StartingEquipment        []upCount  `json:"starting_equipment"`
	StartingEquipmentOptions []upChoice `json:"starting_equipment_options"`
	Subclasses               []apiRef   `json:"subclasses"`
	Spellcasting             *struct {
		Level               int    `json:"level"`
		SpellcastingAbility apiRef `json:"spellcasting_ability"`
		Info                []struct {
			Name string   `json:"name"`
			Desc []string `json:"desc"`
		} `json:"info"`
	} `json:"spellcasting"`
	MultiClassing *struct {
		Prerequisites []struct {
			AbilityScore apiRef `json:"ability_score"`
			MinimumScore int    `json:"minimum_score"`
		} `json:"prerequisites"`
		Proficiencies      []apiRef   `json:"proficiencies"`
		ProficiencyChoices []upChoice `json:"proficiency_choices"`
	} `json:"multi_classing"`
}

// upCount is upstream's {quantity, equipment} starting-equipment entry.
// upCount is a quantity of one item. Upstream spells the item reference two
// different ways for the same shape: starting equipment uses "equipment",
// while an equipment pack's contents use "item". Both are decoded here and
// ref() picks whichever is set, because the alternative -- one type per
// spelling -- is how 66 pack contents came to be generated with empty slugs.
type upCount struct {
	Equipment apiRef `json:"equipment"`
	Item      apiRef `json:"item"`
	Quantity  int    `json:"quantity"`
}

// ref returns the item this count refers to, under either upstream spelling.
func (u upCount) ref() apiRef {
	if u.Equipment.Index != "" {
		return u.Equipment
	}
	return u.Item
}

type upLevel struct {
	Level               int      `json:"level"`
	AbilityScoreBonuses int      `json:"ability_score_bonuses"`
	ProfBonus           int      `json:"prof_bonus"`
	Features            []apiRef `json:"features"`
	Index               string   `json:"index"`
	Class               apiRef   `json:"class"`
	Subclass            *apiRef  `json:"subclass"`
	Spellcasting        *struct {
		CantripsKnown int `json:"cantrips_known"`
		SpellsKnown   int `json:"spells_known"`
		Slots         [10]int
	} `json:"-"`
	RawSpellcasting map[string]int             `json:"spellcasting"`
	ClassSpecific   map[string]json.RawMessage `json:"class_specific"`
}

type upSubclass struct {
	Index          string   `json:"index"`
	Name           string   `json:"name"`
	Class          apiRef   `json:"class"`
	SubclassFlavor string   `json:"subclass_flavor"`
	Desc           textList `json:"desc"`
	Spells         []struct {
		Spell         apiRef `json:"spell"`
		Prerequisites []struct {
			Type  string `json:"type"`
			Index string `json:"index"`
			Name  string `json:"name"`
		} `json:"prerequisites"`
	} `json:"spells"`
}

type upFeature struct {
	Index         string   `json:"index"`
	Name          string   `json:"name"`
	Level         int      `json:"level"`
	Desc          textList `json:"desc"`
	Class         *apiRef  `json:"class"`
	Subclass      *apiRef  `json:"subclass"`
	Parent        *apiRef  `json:"parent"`
	Prerequisites []struct {
		Type    string `json:"type"`
		Level   int    `json:"level"`
		Feature string `json:"feature"`
		Spell   string `json:"spell"`
	} `json:"prerequisites"`
	FeatureSpecific *struct {
		ExpertiseOptions   *upChoice `json:"expertise_options"`
		SubfeatureOptions  *upChoice `json:"subfeature_options"`
		EnemyTypeOptions   *upChoice `json:"enemy_type_options"`
		TerrainTypeOptions *upChoice `json:"terrain_type_options"`
		Invocations        []apiRef  `json:"invocations"`
	} `json:"feature_specific"`
}

type upBackground struct {
	Index                    string     `json:"index"`
	Name                     string     `json:"name"`
	StartingProficiencies    []apiRef   `json:"starting_proficiencies"`
	LanguageOptions          *upChoice  `json:"language_options"`
	StartingEquipment        []upCount  `json:"starting_equipment"`
	StartingEquipmentOptions []upChoice `json:"starting_equipment_options"`
	Feature                  *struct {
		Name string   `json:"name"`
		Desc []string `json:"desc"`
	} `json:"feature"`
	PersonalityTraits *upChoice `json:"personality_traits"`
	Ideals            *upChoice `json:"ideals"`
	Bonds             *upChoice `json:"bonds"`
	Flaws             *upChoice `json:"flaws"`
}

type upFeat struct {
	Index         string   `json:"index"`
	Name          string   `json:"name"`
	Desc          textList `json:"desc"`
	Prerequisites []struct {
		AbilityScore apiRef `json:"ability_score"`
		MinimumScore int    `json:"minimum_score"`
	} `json:"prerequisites"`
}

type upDamage struct {
	DamageDice string  `json:"damage_dice"`
	DamageType *apiRef `json:"damage_type"`
}

type upEquipment struct {
	Index             string   `json:"index"`
	Name              string   `json:"name"`
	EquipmentCategory apiRef   `json:"equipment_category"`
	Desc              textList `json:"desc"`
	Cost              struct {
		Quantity int    `json:"quantity"`
		Unit     string `json:"unit"`
	} `json:"cost"`
	Weight float64 `json:"weight"`

	WeaponCategory  string    `json:"weapon_category"`
	WeaponRange     string    `json:"weapon_range"`
	Damage          *upDamage `json:"damage"`
	TwoHandedDamage *upDamage `json:"two_handed_damage"`
	Range           *struct {
		Normal int  `json:"normal"`
		Long   *int `json:"long"`
	} `json:"range"`
	ThrowRange *struct {
		Normal int `json:"normal"`
		Long   int `json:"long"`
	} `json:"throw_range"`
	Properties []apiRef `json:"properties"`

	ArmorCategory string `json:"armor_category"`
	ArmorClass    *struct {
		Base     int  `json:"base"`
		DexBonus bool `json:"dex_bonus"`
		MaxBonus *int `json:"max_bonus"`
	} `json:"armor_class"`
	StrMinimum          int  `json:"str_minimum"`
	StealthDisadvantage bool `json:"stealth_disadvantage"`

	GearCategory *apiRef   `json:"gear_category"`
	Quantity     int       `json:"quantity"`
	Contents     []upCount `json:"contents"`

	ToolCategory string `json:"tool_category"`

	VehicleCategory string `json:"vehicle_category"`
	Speed           *struct {
		Quantity float64 `json:"quantity"`
		Unit     string  `json:"unit"`
	} `json:"speed"`
	Capacity string `json:"capacity"`
}

type upMagicItem struct {
	Index             string   `json:"index"`
	Name              string   `json:"name"`
	EquipmentCategory apiRef   `json:"equipment_category"`
	Desc              textList `json:"desc"`
	Rarity            *struct {
		Name string `json:"name"`
	} `json:"rarity"`
	Variants []apiRef `json:"variants"`
	Variant  bool     `json:"variant"`
}

type upSpell struct {
	Index         string   `json:"index"`
	Name          string   `json:"name"`
	Desc          textList `json:"desc"`
	HigherLevel   []string `json:"higher_level"`
	Range         string   `json:"range"`
	Components    []string `json:"components"`
	Material      string   `json:"material"`
	Ritual        bool     `json:"ritual"`
	Duration      string   `json:"duration"`
	Concentration bool     `json:"concentration"`
	CastingTime   string   `json:"casting_time"`
	Level         int      `json:"level"`
	AttackType    string   `json:"attack_type"`
	School        apiRef   `json:"school"`
	Classes       []apiRef `json:"classes"`
	Subclasses    []apiRef `json:"subclasses"`
	Damage        *struct {
		DamageType             *apiRef           `json:"damage_type"`
		DamageAtSlotLevel      map[string]string `json:"damage_at_slot_level"`
		DamageAtCharacterLevel map[string]string `json:"damage_at_character_level"`
	} `json:"damage"`
	HealAtSlotLevel map[string]string `json:"heal_at_slot_level"`
	DC              *struct {
		DCType    apiRef `json:"dc_type"`
		DCSuccess string `json:"dc_success"`
	} `json:"dc"`
	AreaOfEffect *struct {
		Type string `json:"type"`
		Size int    `json:"size"`
	} `json:"area_of_effect"`
}

// upChoice is the recursive "choose N" grammar. From is decoded by hand
// because option_set_type and option_type are discriminators JSON cannot
// dispatch on its own.
type upChoice struct {
	Desc   string      `json:"desc"`
	Choose int         `json:"choose"`
	Type   string      `json:"type"`
	From   upOptionSet `json:"from"`
}

type upOptionSet struct {
	OptionSetType     string            `json:"option_set_type"`
	Options           []json.RawMessage `json:"options"`
	EquipmentCategory *apiRef           `json:"equipment_category"`
	ResourceListURL   string            `json:"resource_list_url"`
}

// textList is upstream's inconsistently-typed prose field: most collections
// give `desc` as an array of paragraphs, a few give a bare string. Decoding
// both into one type keeps that inconsistency out of every call site.
type textList []string

// UnmarshalJSON accepts either a string or an array of strings.
func (t *textList) UnmarshalJSON(raw []byte) error {
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		*t = list
		return nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err != nil {
		return err
	}
	if one == "" {
		*t = nil
		return nil
	}
	*t = []string{one}
	return nil
}
