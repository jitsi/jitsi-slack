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
	"github.com/caarlos0/env"
	jitsi "github.com/jitsi/jitsi-slack"
	"github.com/justinas/alice"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

type appCfg struct {
	// Slack App/OAuth client configuration
	SlackSigningSecret  string `env:"SLACK_SIGNING_SECRET,required"`
	SlackClientID       string `env:"SLACK_CLIENT_ID,required"`
	SlackClientSecret   string `env:"SLACK_CLIENT_SECRET,required"`
	SlackAppID          string `env:"SLACK_APP_ID,required"`
	SlackAppSharableURL string `env:"SLACK_APP_SHARABLE_URL,required"`
	// jitsi configuration
	JitsiTokenSigningKey string `env:"JITSI_TOKEN_SIGNING_KEY,required"`
	JitsiTokenKid        string `env:"JITSI_TOKEN_KID,required"`
	JitsiTokenIssuer     string `env:"JITSI_TOKEN_ISS,required"`
	JitsiTokenAudience   string `env:"JITSI_TOKEN_AUD,required"`
	JitsiConferenceHost  string `env:"JITSI_CONFERENCE_HOST,required"`
	// dynamodb configuration
	DynamoTable  string `env:"DYNAMO_TABLE,required"`
	DynamoRegion string `env:"DYNAMO_REGION,required"`
	// application configuration
	HTTPPort string `env:"HTTP_PORT" envDefault:"8080"`
}

var (
	log = zerolog.New(os.Stdout).With().
		Timestamp().
		Logger()
)

func main() {
	// Extract app configuration from env variables.
	app := appCfg{}
	err := env.Parse(&app)
	if err != nil {
		log.Fatal().Err(err).Msg("service is misconfigured")
	}

	// Setup dynamodb session and create a token store.
	cfg := aws.Config{
		Region: aws.String(app.DynamoRegion),
	}
	sess, err := session.NewSession(&cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot start service w/o aws session")
	}
	svc := dynamodb.New(sess)
	tokenStore := jitsi.TokenStore{
		TableName: app.DynamoTable,
		DB:        svc,
	}

	// Setup handlers for slash commands.
	slashCmd := jitsi.SlashCommandHandlers{
		ConferenceHost: app.JitsiConferenceHost,
		TokenGenerator: jitsi.TokenGenerator{
			Lifetime:   time.Hour * 24,
			PrivateKey: app.JitsiTokenSigningKey,
			Issuer:     app.JitsiTokenIssuer,
			Audience:   app.JitsiTokenAudience,
			Kid:        app.JitsiTokenKid,
		},
		SlackSigningSecret: app.SlackSigningSecret,
		SharableURL:        app.SlackAppSharableURL,
		TokenReader:        &tokenStore,
	}

	accessURL := "https://slack.com/api/oauth.access?client_id=%s&client_secret=%s&code=%s"
	oauthHandler := jitsi.SlackOAuthHandlers{
		AccessURLTemplate: accessURL,
		ClientID:          app.SlackClientID,
		ClientSecret:      app.SlackClientSecret,
		AppID:             app.SlackAppID,
		TokenWriter:       &tokenStore,
	}

	// Create an http mux and a server for that mux.
	handler := http.NewServeMux()
	addr := fmt.Sprintf(":%s", app.HTTPPort)
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
	handler.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "health check passed")
	})

	// Start the server and set it up for graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() {
		log.Info().Msgf("listening on :%s", app.HTTPPort)
		err = srv.ListenAndServe()
		log.Fatal().Err(err).Msg("shutting server down")
	}()
	<-stop
	log.Info().Msg("shutting server down")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to shutdown cleanly")
	}
}
