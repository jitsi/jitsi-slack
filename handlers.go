package jitsi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
	"github.com/rs/zerolog/hlog"
)

const (
	roomTemplate   = `{"response_type":"in_channel","attachments":[{"fallback":"Meeting started %s","title":"Meeting started %s","color":"#3AA3E3","attachment_type":"default","actions":[{"name":"join","text":"Join","type":"button","url":"%s","style":"primary"}]}]}`
	userTemplate   = `{"response_type":"ephemeral","attachments":[{"fallback":"Invitations have been sent for your meeting.","title":"Invitations have been sent for your meeting.","color":"#3AA3E3","attachment_type":"default","actions":[{"name":"join","text":"Join","type":"button","url":"%s","style":"primary"}]}]}`
	helpMessage    = `{"response_type":"ephemeral","text":"How to use /jitsi...","attachments":[{"text":"To share a conference link with the channel, use '/jitsi'. Now everyone can join.\nTo share a conference link with users, use 'jitsi @bob @alice'. Now you can meet with Bob and Alice."}]}`
	installMessage = `{"response_type":"ephemeral","text":"Please install the jitsi meet app to integrate with your slack workspace.","attachments":[{"text":"%s"}]}`

	// error strings from slack api
	errInvalidAuth      = "invalid_auth"
	errInactiveAccount  = "account_inactive"
	errMissingAuthToken = "not_authed"
	errCannotDMBot      = "cannot_dm_bot"
	errAccessDenied     = "access_denied"
)

var atMentionRE = regexp.MustCompile(`<@([^>|]+)`)

// ConferenceTokenGenerator provides an interface for creating video conference
// authenticated access via JWT.
type ConferenceTokenGenerator interface {
	CreateJWT(tenantID, tenantName, roomClaim, userID, userName, avatarURL string) (string, error)
}

// TokenReader provides an interface for reading access token data from
// a token store.
type TokenReader interface {
	GetFirstBotTokenForTeam(teamID string) (string, error)
}

func handleRequestValidation(w http.ResponseWriter, r *http.Request, SlackSigningSecret string) bool {
	ts := r.Header.Get(RequestTimestampHeader)
	sig := r.Header.Get(RequestSignatureHeader)
	if ts == "" || sig == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
	defer r.Body.Close()

	if !ValidRequest(SlackSigningSecret, string(body), ts, sig) {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	return true
}

func help(w http.ResponseWriter) {
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(helpMessage))
}

func install(w http.ResponseWriter, sharableURL string) {
	installMsg := fmt.Sprintf(installMessage, sharableURL)
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(installMsg))
}

// SlashCommandHandlers provides http handlers for Slack slash commands
// that integrate with Jitsi Meet.
type SlashCommandHandlers struct {
	ConferenceHost     string
	TokenGenerator     ConferenceTokenGenerator
	SlackSigningSecret string
	TokenReader        TokenReader
	SharableURL        string
}

func (s *SlashCommandHandlers) inviteUser(client *slack.Client, hostID, userID, teamID, teamName, room string) error {
	userInfo, err := client.GetUserInfo(userID)
	if err != nil {
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
	if err != nil {
		return err
	}

	channel, _, _, err := client.OpenConversation(
		&slack.OpenConversationParameters{
			Users: []string{userID},
		},
	)
	if err != nil {
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
	_, _, err = client.PostMessage(
		channel.ID,
		"",
		params,
	)
	if err != nil {
		return err
	}
	return nil
}

// Jitsi will create a conference and dispatch an invite message to both users.
// It is a slash command for Slack.
func (s *SlashCommandHandlers) Jitsi(w http.ResponseWriter, r *http.Request) {
	if !handleRequestValidation(w, r, s.SlackSigningSecret) {
		return
	}
	err := r.ParseForm()
	if err != nil {
		hlog.FromRequest(r).Error().
			Err(err).
			Msg("unable to parse form data")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	callerID := r.PostFormValue("user_id")
	teamID := r.PostFormValue("team_id")
	teamName := r.PostFormValue("team_domain")
	text := r.PostFormValue("text")

	if strings.ToLower(text) == "help" {
		help(w)
		return
	}

	// Grab an access token after validating request and body
	// so we can fail early if we don't have one.
	token, err := s.TokenReader.GetFirstBotTokenForTeam(teamID)
	if err != nil {
		switch err.Error() {
		case errMissingAuthToken:
			install(w, s.SharableURL)
		default:
			hlog.FromRequest(r).Error().
				Err(err).
				Msg("retrieving token")
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

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

	slackClient := slack.New(token)
	for _, match := range matches {
		err = s.inviteUser(slackClient, callerID, match[1], teamID, teamName, room)
		if err != nil {
			switch err.Error() {
			case errInvalidAuth, errInactiveAccount, errMissingAuthToken:
				install(w, s.SharableURL)
				return
			case errCannotDMBot:
				hlog.FromRequest(r).Warn().
					Err(err).
					Msg("inviting user")
			default:
				hlog.FromRequest(r).Error().
					Err(err).
					Msg("inviting user")
			}
		}
	}

	callerInfo, err := slackClient.GetUserInfo(callerID)
	if err != nil {
		switch err.Error() {
		case errInvalidAuth, errInactiveAccount, errMissingAuthToken:
			install(w, s.SharableURL)
		default:
			hlog.FromRequest(r).Error().
				Err(err).
				Msg("retrieving user info from slack")
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
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

// TokenWriter provides an interface to write access token data to the
// token store.
type TokenWriter interface {
	Store(data *TokenData) error
}

// SlackOAuthHandlers is used for handling Slack OAuth validation.
type SlackOAuthHandlers struct {
	AccessURLTemplate string
	ClientID          string
	ClientSecret      string
	AppID             string
	TokenWriter       TokenWriter
}

type botToken struct {
	BotUserID      string `json:"bot_user_id"`
	BotAccessToken string `json:"bot_access_token"`
}

type accessResponse struct {
	OK          bool     `json:"ok"`
	AccessToken string   `json:"access_token"`
	Scope       string   `json:"scope"`
	UserID      string   `json:"user_id"`
	TeamName    string   `json:"team_name"`
	TeamID      string   `json:"team_id"`
	Bot         botToken `json:"bot"`
}

// Auth validates OAuth access tokens.
func (o *SlackOAuthHandlers) Auth(w http.ResponseWriter, r *http.Request) {
	params, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		hlog.FromRequest(r).Error().
			Err(err).
			Msg("parsing query params")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// An error is received when a user declines to install
	// or an unexpected issue occurs. The app treats a
	// declined install gracefully.
	if params["error"] != nil {
		switch params["error"][0] {
		case errAccessDenied:
			hlog.FromRequest(r).Info().
				Err(errors.New(params["error"][0])).
				Msg("user declined install")
			w.WriteHeader(http.StatusOK)
			return
		default:
			hlog.FromRequest(r).Error().
				Err(errors.New(params["error"][0])).
				Msg("failed install")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	code := params["code"]
	if len(code) != 1 {
		hlog.FromRequest(r).Error().
			Err(err).
			Msg("code not provided")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: inject an http client with http logging.
	resp, err := http.Get(fmt.Sprintf(
		o.AccessURLTemplate,
		o.ClientID,
		o.ClientSecret,
		code[0],
	))
	if err != nil {
		hlog.FromRequest(r).Error().
			Err(err).
			Msg("oauth req error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var access accessResponse
	if err := json.NewDecoder(resp.Body).Decode(&access); err != nil {
		hlog.FromRequest(r).Error().
			Err(err).
			Msg("unable to decode slack access response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !access.OK {
		hlog.FromRequest(r).Warn().
			Msg("access not ok")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err = o.TokenWriter.Store(&TokenData{
		TeamID:      access.TeamID,
		UserID:      access.UserID,
		BotToken:    access.Bot.BotAccessToken,
		BotUserID:   access.Bot.BotUserID,
		AccessToken: access.AccessToken,
	})
	if err != nil {
		hlog.FromRequest(r).Error().
			Err(err).
			Msg("unable to store token")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	redirect := fmt.Sprintf("https://slack.com/app_redirect?app=%s", o.AppID)
	http.Redirect(w, r, redirect, http.StatusFound)
}
