package app

import (
	"context"
	"time"

	authdomain "github.com/promix1722/easydnd/internal/domain/auth"
	"github.com/promix1722/easydnd/internal/domain/character"
	"github.com/promix1722/easydnd/internal/domain/group"
	"github.com/promix1722/easydnd/internal/domain/pack"
	"github.com/promix1722/easydnd/internal/domain/rules"
	"github.com/promix1722/easydnd/internal/domain/user"
	"github.com/promix1722/easydnd/internal/types"
	charuc "github.com/promix1722/easydnd/internal/usecase/character"
	gameuc "github.com/promix1722/easydnd/internal/usecase/game"
)

// These IDs cannot collide with the base64url IDs issued by real sign-ins.
const devGroupID group.ID = "dev:gameplay"

var devAccounts = map[string]user.ID{
	"master": "dev:master", "player1": "dev:player1", "player2": "dev:player2",
}

// devLogin is constructed only for development. It can issue sessions for
// exactly the three seeded identities, never an arbitrary stored account.
type devLogin struct {
	users  user.Repository
	signer authdomain.Signer
	ttl    time.Duration
	games  []string
}

func (d *devLogin) Login(ctx context.Context, account string) (string, []string, error) {
	id, ok := devAccounts[account]
	if !ok {
		return "", nil, types.NewValidationError("unknown development account")
	}
	if _, err := d.users.ByID(ctx, id); err != nil {
		return "", nil, err
	}
	now := time.Now()
	token, err := d.signer.SignSession(authdomain.Session{UserID: id, IssuedAt: now, ExpiresAt: now.Add(d.ttl)})
	return token, d.games, err
}

// seedDevelopment reuses durable demo accounts and their group after an API
// restart. Characters and games are process-local, so every new process builds
// them through the normal usecases, with real ownership and permission checks.
func seedDevelopment(ctx context.Context, users user.Repository, groups group.Repository, chars *charuc.Service, games *gameuc.Service, signer authdomain.Signer, ttl time.Duration, seedRules pack.Lock) (*devLogin, error) {
	now := time.Now().UTC()
	for _, name := range []string{"master", "player1", "player2"} {
		id := devAccounts[name]
		if _, err := users.ByID(ctx, id); types.IsNotFound(err) {
			if err := users.Create(ctx, user.User{ID: id, DisplayName: name, CreatedAt: now}); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	}
	master := devAccounts["master"]
	if _, err := groups.ByID(ctx, devGroupID); types.IsNotFound(err) {
		if err := groups.Create(ctx, group.Group{ID: devGroupID, Name: "Development party", CreatedBy: master, CreatedAt: now}, master); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	for _, name := range []string{"player1", "player2"} {
		if _, err := groups.MemberRole(ctx, devGroupID, devAccounts[name]); types.IsNotFound(err) {
			if err := groups.AddMember(ctx, devGroupID, devAccounts[name], group.RolePlayer, now); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	}
	var players []character.ID
	var masterCharacter character.ID
	for _, account := range []string{"master", "player1", "player2"} {
		c, err := seedCharacter(ctx, chars, character.OwnerID(devAccounts[account]), account+" rogue", devRogue, seedRules)
		if err != nil {
			return nil, err
		}
		if account == "master" {
			masterCharacter = c.ID
		} else {
			players = append(players, c.ID)
			if err := games.Share(ctx, devAccounts[account], devGroupID, c.ID); err != nil {
				return nil, err
			}
		}
	}
	// Two casters for the consumables tracker, which a first-level rogue cannot
	// show.
	for _, seed := range []struct {
		account string
		build   devBuild
	}{{"player1", devPaladin}, {"player2", devCleric}} {
		c, err := seedCharacter(ctx, chars, character.OwnerID(devAccounts[seed.account]), seed.account+" "+string(seed.build.class), seed.build, seedRules)
		if err != nil {
			return nil, err
		}
		if err := games.Share(ctx, devAccounts[seed.account], devGroupID, c.ID); err != nil {
			return nil, err
		}
		players = append(players, c.ID)
	}
	login := &devLogin{users: users, signer: signer, ttl: ttl}
	for _, name := range []string{"Training encounter", "Locked encounter"} {
		g, err := games.Create(ctx, master, devGroupID, name)
		if err != nil {
			return nil, err
		}
		if err := games.AddCharacters(ctx, master, g.ID, players); err != nil {
			return nil, err
		}
		login.games = append(login.games, string(g.ID))
		if name == "Training encounter" {
			if err := games.AddMonster(ctx, master, g.ID, masterCharacter, rules.DefaultLocale); err != nil {
				return nil, err
			}
			if err := games.AddMonster(ctx, master, g.ID, "", rules.DefaultLocale); err != nil {
				return nil, err
			}
		}
		participants, err := games.Participants(ctx, master, g.ID, rules.DefaultLocale)
		if err != nil {
			return nil, err
		}
		for index, participant := range participants {
			initiative := 18 - index*4
			patch := gameuc.EntryPatch{InitiativeSet: true, Initiative: &initiative}
			if index == 0 {
				tags := []string{"Blessed"}
				temporary := 3
				patch.Tags, patch.TempHP = &tags, &temporary
			}
			if name == "Locked encounter" && index == 1 {
				locked := true
				patch.Locked = &locked
			}
			if err := games.PatchEntry(ctx, master, g.ID, participant.Entry.ID, patch); err != nil {
				return nil, err
			}
		}
	}
	return login, nil
}
