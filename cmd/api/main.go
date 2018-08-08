package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	jitsi "github.com/jitsi/jitsi-slack"
	"github.com/justinas/alice"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

const (
	// secrets and environment configuration
	envSlackJitsiSigningSecret = "SLACK_JITSI_SIGNING_SECRET"
	envSlackClientID           = "SLACK_CLIENT_ID"
	envSlackClientSecret       = "SLACK_CLIENT_SECRET"
	envSlackAppID              = "SLACK_APP_ID"
	envSlackAppSharableURL     = "SLACK_APP_SHARABLE_URL"
	envDynamoTable             = "DYNAMO_TABLE"
	envDynamoRegion            = "DYNAMO_REGION"
	envJitsiSigningKey         = "JITSI_TOKEN_SIGNING_KEY"
	envJitsiKID                = "JITSI_TOKEN_KID"
	envJitsiISS                = "JITSI_TOKEN_ISS"
	envJitsiAUD                = "JITSI_TOKEN_AUD"
	envJitsiConferenceHost     = "JITSI_CONFERENCE_HOST"
	envHTTPPort                = "HTTP_PORT"
)

type app struct {
	// Slack App/OAuth client configuration
	slackSigningSecret  string
	slackClientID       string
	slackClientSecret   string
	slackAppID          string
	slackAppSharableURL string

	// jitsi configuration
	jitsiTokenSigningKey string
	jitsiTokenKid        string
	jitsiTokenIssuer     string
	jitsiTokenAudience   string
	jitsiConferenceHost  string

	// dynamodb configuration
	dynamoTable  string
	dynamoRegion string

	// application configuration
	httpPort string
}

var log = zerolog.New(os.Stdout).With().
	Timestamp().
	Logger()

func newApp() (*app, error) {
	var a app

	retErr := func(envVarName string) error {
		return fmt.Errorf("%s must be set in env", envVarName)
	}

	table, ok := os.LookupEnv(envDynamoTable)
	if !ok {
		return nil, retErr(envDynamoTable)
	}
	a.dynamoTable = table

	region, ok := os.LookupEnv(envDynamoRegion)
	if !ok {
		return nil, retErr(envDynamoRegion)
	}
	a.dynamoRegion = region

	appID, ok := os.LookupEnv(envSlackAppID)
	if !ok {
		return nil, retErr(envSlackAppID)
	}
	a.slackAppID = appID

	appSharableURL, ok := os.LookupEnv(envSlackAppSharableURL)
	if !ok {
		return nil, retErr(envSlackAppSharableURL)
	}
	a.slackAppSharableURL = appSharableURL

	sss, ok := os.LookupEnv(envSlackJitsiSigningSecret)
	if !ok {
		return nil, retErr(envSlackJitsiSigningSecret)
	}
	a.slackSigningSecret = sss

	clientID, ok := os.LookupEnv(envSlackClientID)
	if !ok {
		return nil, retErr(envSlackClientID)
	}
	a.slackClientID = clientID

	clientSecret, ok := os.LookupEnv(envSlackClientSecret)
	if !ok {
		return nil, retErr(envSlackClientSecret)
	}
	a.slackClientSecret = clientSecret

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
		log.Fatal().Err(err).Msg("service is misconfigured")
	}

	// Setup dynamodb session and create a token store.
	cfg := aws.Config{
		Region: aws.String(app.dynamoRegion),
	}
	sess, err := session.NewSession(&cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot start service w/o aws session")
	}
	svc := dynamodb.New(sess)
	tokenStore := jitsi.TokenStore{
		TableName: app.dynamoTable,
		DB:        svc,
	}

	// Setup handlers for slash commands.
	slashCmd := jitsi.SlashCommandHandlers{
		ConferenceHost: app.jitsiConferenceHost,
		TokenGenerator: jitsi.TokenGenerator{
			Lifetime:   time.Hour * 24,
			PrivateKey: app.jitsiTokenSigningKey,
			Issuer:     app.jitsiTokenIssuer,
			Audience:   app.jitsiTokenAudience,
			Kid:        app.jitsiTokenKid,
		},
		SlackSigningSecret: app.slackSigningSecret,
		SharableURL:        app.slackAppSharableURL,
		TokenReader:        &tokenStore,
	}

	accessURL := "https://slack.com/api/oauth.access?client_id=%s&client_secret=%s&code=%s"
	oauthHandler := jitsi.SlackOAuthHandlers{
		AccessURLTemplate: accessURL,
		ClientID:          app.slackClientID,
		ClientSecret:      app.slackClientSecret,
		AppID:             app.slackAppID,
		TokenWriter:       &tokenStore,
	}

	// Create an http mux and a server for that mux.
	handler := http.NewServeMux()
	addr := fmt.Sprintf(":%s", app.httpPort)
	srv := &http.Server{
		// It's important to set http server timeouts for the publicly available service api.
		// 5 seconds between when connection is accepted to when the body is fully reaad.
		ReadTimeout: 5 * time.Second,
		// 10 seconds from end of request headers read to end of response write.
		WriteTimeout: 10 * time.Second,
		// 120 seconds for an idle KeeP-Alive connection.
		IdleTimeout: 120 * time.Second,
		Addr:        addr,
		Handler:     handler,
	}

	// Create a middleware chain setup to log http access and inject
	// a logger into the request context.
	chain := alice.New(
		hlog.NewHandler(log),
		hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
			hlog.FromRequest(r).Info().
				Str("method", r.Method).
				Str("url", r.URL.String()).
				Int("status", status).
				Int("size", size).
				Dur("duration", duration).
				Msg("")
		}),
		hlog.RemoteAddrHandler("ip"),
		hlog.UserAgentHandler("user_agent"),
		hlog.RefererHandler("referer"),
		hlog.RequestIDHandler("req_id", "Request-Id"),
	)

	// Wrap handlers with middleware chain.
	slashJitsi := chain.ThenFunc(slashCmd.Jitsi)
	slackOAuth := chain.ThenFunc(oauthHandler.Auth)

	// Add routes and wrapped handlers to mux.
	handler.Handle("/slash/jitsi", slashJitsi)
	handler.Handle("/slack/auth", slackOAuth)

	// Start the server and set it up for graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() {
		log.Info().Msgf("listening on :%s", app.httpPort)
		err = srv.ListenAndServe()
		log.Fatal().Err(err).Msg("shutting server down")
	}()
	<-stop
	log.Info().Msg("shutting server down")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
