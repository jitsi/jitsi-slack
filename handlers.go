package jitsi

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
)

const (
	userTemplate = `{"attachments":[{"fallback":"You have sent invites to your meeting.","title":"You have sent invites to your meeting.","color":"#3AA3E3","attachment_type":"default","actions":[{"name":"join","text":"Join","type":"button","url":"%s","style":"primary"}]}]}`
	roomTemplate = `{"response_type":"in_channel","attachments":[{"fallback":"Meeting starting %s","title":"Meeting starting %s","color":"#3AA3E3","attachment_type":"default","actions":[{"name":"join","text":"Join","type":"button","url":"%s","style":"primary"}]}]}`
)

var atMentionRE = regexp.MustCompile(`<@([^>|]+)`)

type logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

// ConferenceTokenGenerator provides an interface for creating video conference
// authenticated access via JWT.
type ConferenceTokenGenerator interface {
	CreateJWT(tenantID, tenantName, roomClaim, userID, userName, avatarURL string) (string, error)
}

// SlashCommandHandlers provides http handlers for Slack slash commands
// that integrate with Jitsi Meet.
type SlashCommandHandlers struct {
	Log                logger
	ConferenceHost     string
	TokenGenerator     ConferenceTokenGenerator
	SlackSigningSecret string
	// TODO convert to an interface; opt-in
	BotClient *slack.Client
}

func (s *SlashCommandHandlers) inviteUser(hostID, userID, teamID, teamName, room string) error {
	userInfo, err := s.BotClient.GetUserInfo(userID)
	if err != nil {
		s.Log.Errorf("retrieving user info from slack: %v", err)
		return err
	}
	userToken, err := s.TokenGenerator.CreateJWT(
		strings.ToLower(teamID),
		strings.ToLower(teamName),
		room,
		userInfo.ID,
		userInfo.Name,
		userInfo.Profile.Image192,
	)

	channel, _, _, err := s.BotClient.OpenConversation(
		&slack.OpenConversationParameters{
			Users: []string{userID},
		},
	)
	if err != nil {
		s.Log.Errorf("opening slack conversation: %v", err)
		return err
	}

	confURL := fmt.Sprintf(
		"%s/%s/%s?jwt=%s",
		s.ConferenceHost,
		strings.ToLower(teamName),
		room,
		userToken,
	)

	params := slack.PostMessageParameters{}
	msg := fmt.Sprintf("<@%s> would like you to join a meeting.", hostID)
	attachment := slack.Attachment{
		Fallback: msg,
		Title:    msg,
		Color:    "#3AA3E3",
		Actions: []slack.AttachmentAction{
			slack.AttachmentAction{
				Name:  "join",
				Text:  "Join",
				Type:  "button",
				Style: "primary",
				URL:   confURL,
			},
		},
	}
	params.Attachments = []slack.Attachment{attachment}
	s.BotClient.PostMessage(
		channel.ID,
		"",
		params,
	)
	return nil
}

// Jitsi will create a conference and dispatch an invite message to both users.
// It is a slash command for Slack.
func (s *SlashCommandHandlers) Jitsi(w http.ResponseWriter, r *http.Request) {
	ts := r.Header.Get(RequestTimestampHeader)
	sig := r.Header.Get(RequestSignatureHeader)
	if ts == "" || sig == "" {
		s.Log.Errorf("recieved an invalid request; sig or ts missing")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.Log.Errorf("reading req body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if !ValidRequest(s.SlackSigningSecret, string(body), ts, sig) {
		s.Log.Errorf("recieved an invalid request; sig mismatch")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Restore the body after it was read for verification so we can
	// easily parse the form data.
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	r.ParseForm()
	callerID := r.PostFormValue("user_id")
	teamID := r.PostFormValue("team_id")
	teamName := r.PostFormValue("team_domain")
	text := r.PostFormValue("text")

	room := RandomName()

	matches := atMentionRE.FindAllStringSubmatch(text, -1)
	if matches == nil {
		meetingURL := fmt.Sprintf(
			"%s/%s/%s",
			s.ConferenceHost,
			strings.ToLower(teamName),
			room,
		)

		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := fmt.Sprintf(roomTemplate, meetingURL, meetingURL, meetingURL)
		w.Write([]byte(resp))
		return
	}

	for _, match := range matches {
		err := s.inviteUser(callerID, match[1], teamID, teamName, room)
		if err != nil {
			s.Log.Errorf("inviting user: %v", err)
		}
	}

	callerInfo, err := s.BotClient.GetUserInfo(callerID)
	if err != nil {
		s.Log.Errorf("retrieving user info from slack: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// TODO: make a function for auth'ed urls.
	callerToken, err := s.TokenGenerator.CreateJWT(
		strings.ToLower(teamID),
		strings.ToLower(teamName),
		room,
		callerID,
		callerInfo.Name,
		callerInfo.Profile.Image192,
	)

	callerConfURL := fmt.Sprintf(
		"%s/%s/%s?jwt=%s",
		s.ConferenceHost,
		strings.ToLower(teamName),
		room,
		callerToken,
	)

	// TODO: determine what's an error that gets exposed to the user.
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := fmt.Sprintf(userTemplate, callerConfURL)
	w.Write([]byte(resp))
}
