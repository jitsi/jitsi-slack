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

	"github.com/rs/zerolog/hlog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
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

// EventHandler is used to handle event callbacks from Slack api.
type EventHandler struct {
	SlackSigningSecret string
	TokenWriter        TokenWriter
}

// Handle handles event callbacks for the integration.
// adapated from https://github.com/slack-go/slack/blob/master/examples/eventsapi/events.go
func (e *EventHandler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		hlog.FromRequest(r).Warn().Err(err).Msg("evhandle: malformed request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	sv, err := slack.NewSecretsVerifier(r.Header, e.SlackSigningSecret)
	if err != nil {
		hlog.FromRequest(r).Warn().Err(err).Msg("evhandle: signature failed")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(body); err != nil {
		hlog.FromRequest(r).Warn().Err(err).Msg("evhandle: write failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := sv.Ensure(); err != nil {
		hlog.FromRequest(r).Warn().Err(err).Msg("ensure failed: secrets may be not loading")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		hlog.FromRequest(r).Warn().Err(err).Msg("evhandle: parse failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if eventsAPIEvent.Type == slackevents.URLVerification {
		var cr *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &cr)
		if err != nil {
			hlog.FromRequest(r).Warn().Err(err).Msg("evhandle: challenge resp failed")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(cr.Challenge))
	}

	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		switch innerEvent.Data.(type) {
		case *slackevents.AppUninstalledEvent:
			{
				err := e.TokenWriter.Remove(eventsAPIEvent.TeamID)
				// do not error out or return 500 since this failing is non-critical
				if err != nil {
					hlog.FromRequest(r).Warn().
						Err(err).
						Msg(fmt.Sprintf("app_uninstalled failed for: %s", eventsAPIEvent.TeamID))
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
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
	if len(configuration) < 2 {
		fmt.Fprintf(w, "Run '/jitsi server default' or '/jitsi server [url]' with the URL of your team's server")
		return
	}

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
			hlog.FromRequest(r).Info().
				Err(err).
				Msg("missing auth token")
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
		err = sendPersonalizedInvite(token.AccessToken, callerID, match[1], &meeting)
		if err != nil {
			switch err.Error() {
			case errInactiveAccount, errMissingAuthToken:
				hlog.FromRequest(r).Info().
					Err(err).
					Msg(fmt.Sprintf("inactive or missing auth token"))
				install(w, s.SharableURL)
				return
			case errInvalidAuth:
				// catches the case where a workspace has removed the app but
				// someone tries to use the command anyways
				hlog.FromRequest(r).Info().
					Err(err).
					Msg("invalid auth")
				install(w, s.SharableURL)
				return
			case errCannotDMBot:
				hlog.FromRequest(r).Warn().
					Err(err).
					Msg("bot cannot DM")
			default:
				hlog.FromRequest(r).Error().
					Err(err).
					Msg("unexpected sendPersonalizedInvite error")
			}
		}
	}

	// Create a personalized response for the meeting initiator.
	resp, err := joinPersonalMeetingMsg(token.AccessToken, callerID, &meeting)
	if err != nil {
		switch err.Error() {
		case errInvalidAuth, errInactiveAccount, errMissingAuthToken:
			hlog.FromRequest(r).Info().
				Err(err).
				Msg("joinPersonalMeetingMsg invalid or missing token")
			install(w, s.SharableURL)
			return
		default:
			hlog.FromRequest(r).Error().
				Err(err).
				Msg("joinPersonalizedMeetingMsg error")
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
	Remove(teamID string) error
}

// SlackOAuthHandlers is used for handling Slack OAuth validation.
type SlackOAuthHandlers struct {
	ClientID     string
	ClientSecret string
	AppID        string
	TokenWriter  TokenWriter
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

	resp, err := slack.GetOAuthV2Response(
		http.DefaultClient,
		o.ClientID,
		o.ClientSecret,
		code[0],
		"")

	if err != nil {
		hlog.FromRequest(r).Error().
			Err(err).
			Msg("oauth req error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = o.TokenWriter.Store(&TokenData{
		TeamID:      resp.Team.ID,
		AccessToken: resp.AccessToken,
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
