package teams

import (
	"errors"
	"sort"
	"strings"

	"github.com/keybase/client/go/libkb"
	"github.com/keybase/client/go/protocol/keybase1"
	"golang.org/x/net/context"
)

type implicitTeamConflict struct {
	TeamID       keybase1.TeamID `json:"team_id"`
	Generation   int             `json:"generation"`
	ConflictDate string          `json:"conflict_date"`
}

type implicitTeam struct {
	TeamID      keybase1.TeamID        `json:"team_id"`
	DisplayName string                 `json:"display_name"`
	Private     bool                   `json:"is_private"`
	Conflicts   []implicitTeamConflict `json:"conflicts,omitempty"`
	Status      libkb.AppStatus        `json:"status"`
}

func (i *implicitTeam) GetAppStatus() *libkb.AppStatus {
	return &i.Status
}

func LookupImplicitTeam(ctx context.Context, g *libkb.GlobalContext, name string, public bool) (res keybase1.TeamID, err error) {
	impTeamName, err := libkb.ParseImplicitTeamName(g.MakeAssertionContext(), name, public)
	if err != nil {
		return res, err
	}
	impTeamMembers := make(map[string]bool)
	for _, u := range impTeamName.KeybaseUsers {
		impTeamMembers[u] = true
	}

	arg := libkb.NewRetryAPIArg("team/implicit")
	arg.NetContext = ctx
	arg.SessionType = libkb.APISessionTypeREQUIRED
	arg.Args = libkb.HTTPArgs{
		"display_name": libkb.S{Val: name},
		"public":       libkb.B{Val: public},
	}
	var imp implicitTeam
	if err = g.API.GetDecode(arg, &imp); err != nil {
		return res, err
	}

	team, err := Load(ctx, g, keybase1.LoadTeamArg{
		ID: imp.TeamID,
	})
	if err != nil {
		return res, err
	}
	teamDisplayName, err := team.ImplicitTeamDisplayName(ctx)
	if err != nil {
		return res, err
	}
	if teamDisplayName != FormatImplicitTeamName(ctx, g, impTeamName) {
		return res, errors.New("implicit team name mismatch")
	}

	return imp.TeamID, nil
}

func LookupOrCreateImplicitTeam(ctx context.Context, g *libkb.GlobalContext, name string, public bool) (res keybase1.TeamID, err error) {
	teamID, err := LookupImplicitTeam(ctx, g, name, public)
	if err != nil {
		if _, ok := err.(TeamDoesNotExistError); ok {
			// If the team does not exist, then let's create it
			impTeamName, err := libkb.ParseImplicitTeamName(g.MakeAssertionContext(), name, public)
			if err != nil {
				return res, err
			}
			return CreateImplicitTeam(ctx, g, impTeamName)
		}
		return res, err
	}
	return teamID, nil
}

func FormatImplicitTeamName(ctx context.Context, g *libkb.GlobalContext, impTeamName keybase1.ImplicitTeamName) string {
	// Sort
	var unresolvedStrs []string
	sort.Slice(impTeamName.KeybaseUsers, func(i, j int) bool {
		return impTeamName.KeybaseUsers[i] < impTeamName.KeybaseUsers[j]
	})
	sort.Slice(impTeamName.UnresolvedUsers, func(i, j int) bool {
		return impTeamName.UnresolvedUsers[i].String() < impTeamName.UnresolvedUsers[j].String()
	})
	for _, u := range impTeamName.UnresolvedUsers {
		unresolvedStrs = append(unresolvedStrs, u.String())
	}
	return strings.Join(append(impTeamName.KeybaseUsers, unresolvedStrs...), ",")
}
