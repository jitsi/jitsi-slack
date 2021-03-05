package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	env "github.com/caarlos0/env/v6"
	jitsi "github.com/jitsi/jitsi-slack"
	stats "github.com/jitsi/prometheus-stats"
	"github.com/justinas/alice"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	TokenTable     string `env:"TOKEN_TABLE,required"`
	ServerCfgTable string `env:"SERVER_CFG_TABLE,required"`
	DynamoRegion   string `env:"DYNAMO_REGION,required"`
	// application configuration
	HTTPPort  string `env:"HTTP_PORT" envDefault:"8080"`
	StatsPort string `env:"STATS_PORT" envDefault:"0"`
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

	// set up acces to dynamodb stores
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(app.DynamoRegion))
	if err != nil {
		log.Fatal().Err(err).Msg("cannot start service w/o aws session")
	}
	svc := dynamodb.NewFromConfig(cfg)
	tokenStore := jitsi.TokenStore{
		TableName: app.TokenTable,
		DB:        svc,
	}

	authTenantSupportTest := func(srv string) bool {
		if srv == app.JitsiConferenceHost {
			return true
		}
		return false
	}

	srvCfgStore := jitsi.ServerCfgStore{
		TableName:               app.ServerCfgTable,
		DB:                      svc,
		DefaultServer:           app.JitsiConferenceHost,
		TenantScopedURLs:        authTenantSupportTest,
		AuthenticatedURLSupport: authTenantSupportTest,
	}

	// Setup handlers for slash commands.
	slashCmd := jitsi.SlashCommandHandlers{
		MeetingGenerator: &jitsi.MeetingGenerator{
			ServerConfigReader: &srvCfgStore,
			MeetingTokenGenerator: jitsi.TokenGenerator{
				Lifetime:   time.Hour * 24,
				PrivateKey: app.JitsiTokenSigningKey,
				Issuer:     app.JitsiTokenIssuer,
				Audience:   app.JitsiTokenAudience,
				Kid:        app.JitsiTokenKid,
			},
		},
		SlackSigningSecret: app.SlackSigningSecret,
		SharableURL:        app.SlackAppSharableURL,
		TokenReader:        &tokenStore,
		TokenWriter:        &tokenStore,
		ServerConfigWriter: &srvCfgStore,
	}

	evHandle := jitsi.EventHandler{
		SlackSigningSecret: app.SlackSigningSecret,
		TokenWriter:        &tokenStore,
	}

	oauthHandler := jitsi.SlackOAuthHandlers{
		ClientID:     app.SlackClientID,
		ClientSecret: app.SlackClientSecret,
		AppID:        app.SlackAppID,
		TokenWriter:  &tokenStore,
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
	slackEvent := chain.ThenFunc(evHandle.Handle)

	// wrap metrics collection and publish endpoint
	statsPort, err := strconv.ParseInt(app.StatsPort, 10, 16)
	if err != nil || statsPort > 65535 || statsPort < 0 {
		log.Fatal().Err(err).Msg("bad port for stats server")
	}
	if statsPort > 0 {
		slashJitsi = stats.WrapHTTPHandler("slashJitsi", slashJitsi)
		slackOAuth = stats.WrapHTTPHandler("slackOAuth", slackOAuth)
		slackEvent = stats.WrapHTTPHandler("slackEvent", slackEvent)
		http.Handle("/metrics", promhttp.Handler())
	}

	// Add routes and wrapped handlers to mux.
	handler.Handle("/slash/jitsi", slashJitsi) // slash command handler
	handler.Handle("/slack/auth", slackOAuth)  // handles "Add to Slack"
	handler.Handle("/slack/event", slackEvent) // handles workspace removal of app
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

	// Start stats server
	if statsPort > 0 {
		go func() {
			log.Info().Msgf("stats listening on :%s", app.StatsPort)
			log.Fatal().Err(http.ListenAndServe(":"+app.StatsPort, nil)).Msg("shutting stat server down")
		}()
	}
	<-stop
	log.Info().Msg("shutting server down")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to shutdown cleanly")
	}
}
