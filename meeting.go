package jitsi

import (
	"fmt"
	"strings"
)

// MeetingTokenGenerator provides an interface for creating video conference
// authenticated access via JWT.
type MeetingTokenGenerator interface {
	CreateJWT(JWTInput) (string, error)
}

// ServerConfigReader provides an interface for reading configure server data
// from the configuration store.
type ServerConfigReader interface {
	Get(string) (ServerCfg, error)
}

// MeetingGenerator provides an interface for generating meetings configured
// correctly for the slack team.
type MeetingGenerator struct {
	ServerConfigReader    ServerConfigReader
	MeetingTokenGenerator MeetingTokenGenerator
}

// Meeting contains the server specific info for a meeting.
type Meeting struct {
	RoomName         string
	URL              string
	Host             string
	AuthenticatedURL func(UserID, UserName, AvatarURL string) (string, error)
}

// New generates a new meeting for the provided team. Each team may either be
// using the default service, meet.jit.si, or their own installation.
func (m *MeetingGenerator) New(teamID, teamName string) (Meeting, error) {
	var mtg Meeting
	mtg.RoomName = RandomName()

	srv, err := m.ServerConfigReader.Get(teamID)
	if err != nil {
		return Meeting{}, err
	}
	mtg.Host = srv.Server

	if srv.TenantScopedURLs {
		mtg.URL = fmt.Sprintf("%s/%s/%s", srv.Server, strings.ToLower(teamName), mtg.RoomName)
	} else {
		mtg.URL = fmt.Sprintf("%s/%s", srv.Server, mtg.RoomName)
	}

	if srv.AuthenticatedURLSupport {
		mtg.AuthenticatedURL = func(userID, userName, avatarURL string) (string, error) {
			jwt, err := m.MeetingTokenGenerator.CreateJWT(JWTInput{
				TenantID:   teamID,
				TenantName: teamName,
				RoomClaim:  mtg.RoomName,
				UserID:     userID,
				UserName:   userName,
				AvatarURL:  avatarURL,
			})
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s?jwt=%s", mtg.URL, jwt), nil
		}
	} else {
		mtg.AuthenticatedURL = func(userID, userName, avatarURL string) (string, error) {
			return mtg.URL, nil
		}
	}
	return mtg, nil
}
