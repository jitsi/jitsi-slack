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

	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/hlog"
)

const (
	// error strings from slack api
	errInvalidAuth      = "invalid_auth"
	errInactiveAccount  = "account_inactive"
	errMissingAuthToken = "not_authed"
	errCannotDMBot      = "cannot_dm_bot"
	errAccessDenied     = "access_denied"
)

var (
	atMentionRE    = regexp.MustCompile(`<@([^>|]+)`)
	serverCmdRE    = regexp.MustCompile(`^server`)
	serverConfigRE = regexp.MustCompile(`^server\s+(<https?:\/\/\S+>)`)
	helpCmdRE      = regexp.MustCompile(`^help`)
)

// TokenReader provides an interface for reading access token data from
// a token store.
type TokenReader interface {
	GetTokenForTeam(teamID string) (*TokenData, error)
}

// ServerConfigWriter provides an interface for writing server configuration
// data for a team's workspace.
type ServerConfigWriter interface {
	Store(*ServerCfgData) error
	Remove(string) error
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

type eventChallenge struct {
	Token     string `mapstructure:"token"`
	Challenge string `mapstructure:"challenge"`
	Type      string `mapstructure:"type"`
}

type eventCallback struct {
	Token string `mapstructure:"token"`
	Event struct {
		Type   string `mapstructure:"type"`
		Tokens struct {
			OAuth []string `mapstructure:"oauth"`
			Bot   []string `mapstructure:"bot"`
		} `mapstructure:"tokens"`
	} `mapstructure:"event"`
}

// EventHandler is used to handle event callbacks from Slack api.
type EventHandler struct {
	SlackSigningSecret string
	TokenWriter        TokenWriter
}

// Handle handles event callbacks for the integration.
func (e *EventHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var rawEvent map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&rawEvent)
	if err != nil {
		hlog.FromRequest(r).Error().Err(err).Msg("evhandle")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	eventType, ok := rawEvent["type"]
	if !ok {
		hlog.FromRequest(r).Error().Msg(fmt.Sprintf("unexpected: %#v", rawEvent))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	switch eventType {
	case "url_verification":
		var ev eventChallenge
		err = mapstructure.Decode(rawEvent, &ev)
		if err != nil {
			hlog.FromRequest(r).Error().Err(err).Msg("challengemap")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(ev.Challenge))
	case "event_callback":
		var ev eventCallback
		err = mapstructure.Decode(rawEvent, &ev)
		if err != nil {
			hlog.FromRequest(r).Error().Err(err).Msg("challengemap")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, each := range ev.Event.Tokens.OAuth {
			err := e.TokenWriter.Remove(each)
			// Log the errors but don't provide a 500 because there's
			// nothing slack will do anything and we should attempt to
			// remove all of them.
			if err != nil {
				hlog.FromRequest(r).
					Error().
					Err(err).
					Msg(fmt.Sprintf("removing for %s", each))
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

// SlashCommandHandlers provides http handlers for Slack slash commands
// that integrate with Jitsi Meet.
type SlashCommandHandlers struct {
	MeetingGenerator   *MeetingGenerator
	SlackSigningSecret string
	TokenReader        TokenReader
	TokenWriter        TokenWriter
	SharableURL        string
	ServerConfigWriter ServerConfigWriter
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

	text := r.PostFormValue("text")
	if helpCmdRE.MatchString(text) {
		help(w)
	} else if serverCmdRE.MatchString(text) {
		s.configureServer(w, r)
	} else {
		s.dispatchInvites(w, r)
	}
}

func (s *SlashCommandHandlers) configureServer(w http.ResponseWriter, r *http.Request) {
	teamID := r.PostFormValue("team_id")
	text := r.PostFormValue("text")

	// First check if the default is being requested.
	configuration := strings.Split(text, " ")
	if configuration[1] == "default" {
		err := s.ServerConfigWriter.Remove(teamID)
		if err != nil {
			hlog.FromRequest(r).Error().
				Err(err).
				Msg("defaulting server")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Your team's conferences will now be hosted on https://meet.jit.si")
		return
	}

	if !serverConfigRE.MatchString(text) {
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "A proper conference host must be provided.")
		return
	}

	host := serverConfigRE.FindAllStringSubmatch(text, -1)[0][1]
	host = strings.Trim(host, "<>")
	err := s.ServerConfigWriter.Store(&ServerCfgData{
		TeamID: teamID,
		Server: host,
	})
	if err != nil {
		hlog.FromRequest(r).Error().
			Err(err).
			Msg("configuring server")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Your team's conferences will now be hosted on %s\nRun `/jitsi server default` if you'd like to continue using https://meet.jit.si", host)
}

func (s *SlashCommandHandlers) dispatchInvites(w http.ResponseWriter, r *http.Request) {
	// Generate the meeting data.
	teamID := r.PostFormValue("team_id")
	teamName := r.PostFormValue("team_domain")
	meeting, err := s.MeetingGenerator.New(teamID, teamName)
	if err != nil {
		hlog.FromRequest(r).Error().
			Err(err).
			Msg("generating meeting")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// If nobody was @-mentioned then just send a generic invite to the channel.
	text := r.PostFormValue("text")
	matches := atMentionRE.FindAllStringSubmatch(text, -1)
	if matches == nil {
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := fmt.Sprintf(roomTemplate, meeting.Host, meeting.Host, meeting.URL)
		w.Write([]byte(resp))
		return
	}

	// Grab a oauth token for the slack workspace.
	token, err := s.TokenReader.GetTokenForTeam(teamID)
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

	// Dispatch a personal invite to each user @-mentioned.
	callerID := r.PostFormValue("user_id")
	for _, match := range matches {
		err = sendPersonalizedInvite(token.BotToken, callerID, match[1], &meeting)
		if err != nil {
			switch err.Error() {
			case errInactiveAccount, errMissingAuthToken:
				install(w, s.SharableURL)
				return
			case errInvalidAuth:
				err := s.TokenWriter.Remove(token.UserID)
				if err != nil {
					hlog.FromRequest(r).
						Error().
						Err(err).
						Msg("removing invalid token")
				}
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

	// Create a personalized response for the meeting initiator.
	resp, err := joinPersonalMeetingMsg(token.BotToken, callerID, &meeting)
	if err != nil {
		switch err.Error() {
		case errInvalidAuth, errInactiveAccount, errMissingAuthToken:
			install(w, s.SharableURL)
			return
		default:
			hlog.FromRequest(r).Error().
				Err(err).
				Msg("inviting user")
		}
	}
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp))
}

// TokenWriter provides an interface to write access token data to the
// token store.
type TokenWriter interface {
	Store(data *TokenData) error
	Remove(userID string) error
}

// SlackOAuthHandlers is used for handling Slack OAuth validation.
type SlackOAuthHandlers struct {
	AccessURLTemplate   string
	AccessV2URLTemplate string
	ClientID            string
	ClientSecret        string
	AppID               string
	TokenWriter         TokenWriter
}

type botToken struct {
	BotUserID      string `json:"bot_user_id"`
	BotAccessToken string `json:"bot_access_token"`
}

type accessResponseV2 struct {
	OK         bool `json:"ok"`
	AuthedUser struct {
		ID          string `json:"id"`
		AccessToken string `json:"access_token"`
	} `json:"authed_user"`
	Team struct {
		ID string `json:"id"`
	} `json:"team"`
	BotUserID      string `json:"bot_user_id"`
	BotAccessToken string `json:"access_token"`
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

// AuthV2 is used for Slack's v2 of OAuth 2.0 with granular scopes.
func (o *SlackOAuthHandlers) AuthV2(w http.ResponseWriter, r *http.Request) {
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
		o.AccessV2URLTemplate,
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

	var access accessResponseV2
	if err = json.NewDecoder(resp.Body).Decode(&access); err != nil {
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
		TeamID:      access.Team.ID,
		UserID:      access.AuthedUser.ID,
		BotToken:    access.BotAccessToken,
		BotUserID:   access.BotUserID,
		AccessToken: access.AuthedUser.AccessToken,
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
	if err = json.NewDecoder(resp.Body).Decode(&access); err != nil {
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
