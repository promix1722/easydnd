// Package pack serves pack authoring, access and catalogue browsing.
package pack

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/gin-gonic/gin"

	"github.com/promix1722/easydnd/internal/api/http/helpers"
	catalogapi "github.com/promix1722/easydnd/internal/api/http/v1/catalog"
	"github.com/promix1722/easydnd/internal/domain/catalog"
	domain "github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/types"
)

type browsePack struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Version  string   `json:"version"`
	Versions []string `json:"versions"`
}
type browseSource struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	PackID string `json:"packId"`
}
type unavailable struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Reason  string `json:"reason"`
}
type browseData struct {
	Packs        []browsePack       `json:"packs"`
	Sources      []browseSource     `json:"sources"`
	Schools      []catalogapi.Entry `json:"schools"`
	Classes      []catalogapi.Entry `json:"classes"`
	Unavailable  []unavailable      `json:"unavailable"`
	spells       []catalogapi.Spell
	domainSpells []catalog.Spell
}

// Each release gets its own compiled dependency closure: browsing does not compose rules.
func (h *Handler) browse(c *gin.Context) (browseData, error) {
	out := browseData{Packs: []browsePack{}, Sources: []browseSource{}, Schools: []catalogapi.Entry{}, Classes: []catalogapi.Entry{}, Unavailable: []unavailable{}}
	rows, err := h.service.List(c.Request.Context(), actor(c).ID)
	if err != nil {
		return out, err
	}
	overrides := map[string]string{}
	if q := c.Query("versions"); q != "" {
		for _, part := range strings.Split(q, ",") {
			id, version, ok := strings.Cut(part, "@")
			if !ok || overrides[id] != "" {
				return out, types.NewValidationError("invalid version selection").Because("pack.invalid")
			}
			overrides[id] = version
		}
	}
	sourceSeen, schoolSeen, classSeen := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		if row.Archived || len(row.Releases) == 0 {
			continue
		}
		slices.SortFunc(row.Releases, func(a, b domain.Document) int {
			av, ae := semver.StrictNewVersion(a.Release.Version)
			bv, be := semver.StrictNewVersion(b.Release.Version)
			if ae != nil || be != nil {
				return strings.Compare(a.Release.Version, b.Release.Version)
			}
			return av.Compare(bv)
		})
		chosen := row.Releases[len(row.Releases)-1].Release
		versions := []string{}
		for _, r := range row.Releases {
			versions = append(versions, r.Release.Version)
			if overrides[row.ID] == r.Release.Version {
				chosen = r.Release
			}
		}
		if version := overrides[row.ID]; version != "" {
			if !slices.Contains(versions, version) {
				return out, types.NewNotFoundError("release unavailable").Because("pack.unavailable")
			}
			delete(overrides, row.ID)
		}
		out.Packs = append(out.Packs, browsePack{ID: row.ID, Title: row.Title, Version: chosen.Version, Versions: versions})
		lock, err := h.service.Resolve(c.Request.Context(), actor(c).ID, []domain.Release{chosen})
		if err != nil {
			out.Unavailable = append(out.Unavailable, unavailable{row.ID, chosen.Version, "pack.unavailable"})
			continue
		}
		cat, err := catalog.LoadLocked(c.Request.Context(), h.source, helpers.Locale(c), lock)
		if err != nil {
			out.Unavailable = append(out.Unavailable, unavailable{row.ID, chosen.Version, "pack.invalid"})
			continue
		}
		conv := catalogapi.NewConverter(cat)
		context := []string{}
		for _, r := range lock.Packs {
			context = append(context, r.ID+"@"+r.Version)
		}
		for _, spell := range cat.Spells.All() {
			if spell.Provenance == nil || spell.Provenance.PackID != row.ID {
				continue
			}
			wire := conv.SpellSummary(spell)
			wire.CatalogPacks = strings.Join(context, ",")
			out.spells = append(out.spells, wire)
			out.domainSpells = append(out.domainSpells, spell)
			for _, source := range spell.Provenance.Sources {
				if !sourceSeen[source.ID] {
					sourceSeen[source.ID] = true
					out.Sources = append(out.Sources, browseSource{source.ID, source.Name, row.ID})
				}
			}
		}
		for _, school := range cat.MagicSchools.All() {
			key := school.Slug.String()
			if !schoolSeen[key] {
				schoolSeen[key] = true
				out.Schools = append(out.Schools, conv.Entry(school.Entry))
			}
		}
		for _, class := range cat.Classes.All() {
			key := class.Slug.String()
			if !classSeen[key] {
				classSeen[key] = true
				out.Classes = append(out.Classes, conv.Entry(class.Entry))
			}
		}
	}
	if len(overrides) > 0 {
		return out, types.NewNotFoundError("release unavailable").Because("pack.unavailable")
	}
	slices.SortFunc(out.Sources, func(a, b browseSource) int { return strings.Compare(a.Name+"/"+a.ID, b.Name+"/"+b.ID) })
	return out, nil
}

// SpellFilters lists accessible releases and their spell filter options.
func (h *Handler) SpellFilters(c *gin.Context) { out, err := h.browse(c); respond(c, out, err) }

// Spells searches accessible releases independently of character rules.
func (h *Handler) Spells(c *gin.Context) {
	filter, limit, offset, err := catalogapi.ParseSpellSearch(c)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	data, err := h.browse(c)
	if err != nil {
		helpers.FormatError(c, err)
		return
	}
	matches := []catalogapi.Spell{}
	for i, spell := range data.domainSpells {
		if filter.Matches(spell) {
			matches = append(matches, data.spells[i])
		}
	}
	slices.SortFunc(matches, func(a, b catalogapi.Spell) int {
		if a.Level != b.Level {
			return a.Level - b.Level
		}
		if n := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); n != 0 {
			return n
		}
		return strings.Compare(fmt.Sprint(a.Provenance.PackID, "/", a.Provenance.Version, "/", a.Slug), fmt.Sprint(b.Provenance.PackID, "/", b.Provenance.Version, "/", b.Slug))
	})
	total := len(matches)
	if offset > total {
		offset = total
	}
	matches = matches[offset:]
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	c.JSON(200, gin.H{"spells": matches, "total": total, "unavailable": data.Unavailable})
}
