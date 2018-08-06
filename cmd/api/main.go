package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	jitsi "github.com/jitsi/jitsi-slack"
	"github.com/nlopes/slack"
	log "github.com/sirupsen/logrus"
)

func init() {
	// logrus setup
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

const (
	// secrets and environment configuration
	envSlackBotToken           = "SLACK_JITSI_BOT_TOKEN"
	envSlackJitsiSigningSecret = "SLACK_JITSI_SIGNING_SECRET"
	envJitsiSigningKey         = "JITSI_TOKEN_SIGNING_KEY"
	envJitsiKID                = "JITSI_TOKEN_KID"
	envJitsiISS                = "JITSI_TOKEN_ISS"
	envJitsiAUD                = "JITSI_TOKEN_AUD"
	envJitsiConferenceHost     = "JITSI_CONFERENCE_HOST"
	envHTTPPort                = "HTTP_PORT"
)

type app struct {
	// Slack App/OAuth client configuration
	slackBotToken      string
	slackSigningSecret string

	// jitsi configuration
	jitsiTokenSigningKey string
	jitsiTokenKid        string
	jitsiTokenIssuer     string
	jitsiTokenAudience   string
	jitsiConferenceHost  string

	// application configuration
	httpPort string
}

func newApp() (*app, error) {
	var a app

	retErr := func(envVarName string) error {
		return fmt.Errorf("%s must be set in env", envVarName)
	}

	botToken, ok := os.LookupEnv(envSlackBotToken)
	if !ok {
		return nil, retErr(envSlackBotToken)
	}
	a.slackBotToken = botToken

	sss, ok := os.LookupEnv(envSlackJitsiSigningSecret)
	if !ok {
		return nil, retErr(envSlackJitsiSigningSecret)
	}
	a.slackSigningSecret = sss

	jitsiTokenSigningKey, ok := os.LookupEnv(envJitsiSigningKey)
	if !ok {
		return nil, retErr(envJitsiSigningKey)
	}
	a.jitsiTokenSigningKey = jitsiTokenSigningKey

	jitsiTokenKid, ok := os.LookupEnv(envJitsiKID)
	if !ok {
		return nil, retErr(envJitsiKID)
	}
	a.jitsiTokenKid = jitsiTokenKid

	jitsiTokenIssuer, ok := os.LookupEnv(envJitsiISS)
	if !ok {
		return nil, retErr(envJitsiKID)
	}
	a.jitsiTokenIssuer = jitsiTokenIssuer

	jitsiTokenAudience, ok := os.LookupEnv(envJitsiAUD)
	if !ok {
		return nil, retErr(envJitsiKID)
	}
	a.jitsiTokenAudience = jitsiTokenAudience

	jitsiConferenceHost, ok := os.LookupEnv(envJitsiConferenceHost)
	if !ok {
		return nil, retErr(envJitsiConferenceHost)
	}
	a.jitsiConferenceHost = jitsiConferenceHost

	httpPort, ok := os.LookupEnv(envHTTPPort)
	if !ok {
		a.httpPort = "8080"
	} else {
		a.httpPort = httpPort
	}

	return &a, nil
}

func main() {
	app, err := newApp()
	if err != nil {
		log.Fatalf("mis-configuration %v", err)
	}

	slashCmd := jitsi.SlashCommandHandlers{
		Log:            log.New(),
		ConferenceHost: app.jitsiConferenceHost,
		TokenGenerator: jitsi.TokenGenerator{
			Lifetime:   time.Hour * 24,
			PrivateKey: app.jitsiTokenSigningKey,
			Issuer:     app.jitsiTokenIssuer,
			Audience:   app.jitsiTokenAudience,
			Kid:        app.jitsiTokenKid,
		},
		SlackSigningSecret: app.slackSigningSecret,
		BotClient:          slack.New(app.slackBotToken),
	}

	http.HandleFunc("/slash/jitsi", slashCmd.Jitsi)

	log.Infof("listening on :%s", app.httpPort)
	http.ListenAndServe(fmt.Sprintf(":%s", app.httpPort), nil)
}
