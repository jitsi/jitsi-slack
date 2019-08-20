# Jitsi Slack - Jitsi Meet Integration for Slack

This project provides a Slack integration to enable starting video conferences
from Slack and easily inviting Slack members to conferences.

Enables starting and joining [Jitsi Meet](https://meet.jit.si) meetings from within [Slack](https://slack.com/)

## Getting Started

These instructions will get you started with the ability to run the project
on your local machine for development purposes.

### Prerequisites

#### Go
A working setup for the Go Programming Language is needed. Here is a [getting started](https://golang.org/doc/install) guide. The project
is currently using go version 1.11 along with module support.

#### Slack

A slack account needs to be created as well as an [app](https://api.slack.com/apps). The app created is intended for development
purposes. The following functionality must be enabled in the `Add features and functionality` section of the slack app configuration:

* Slash Commands
* Bots

The slash command setup is `/jitsi` and the bot mention name is `@jitsi_meet`.

## Configuration

```
SLACK_SIGNING_SECRET=<signing secret of slack app>
SLACK_CLIENT_ID=<client id of slack app>
SLACK_CLIENT_SECRET=<client secret of slack app>
SLACK_APP_ID=<slack app id>
SLACK_APP_SHARABLE_URL=<slack app url for sharing install>
DYNAMO_TABLE=<dynamodb table name for storing oauth tokens>
DYNAMO_REGION=<dynamodb region used>
JITSI_TOKEN_SIGNING_KEY=<key used to sign conference asap jwts>
JITSI_TOKEN_KID=<key identifier for conference asap jwts>
JITSI_TOKEN_ISS=<issuer for conference asap jwts>
JITSI_TOKEN_AUD=<audience for conference asap jwts>
JITSI_CONFERENCE_HOST=<conference hosting service i.e. https://meet.jit.si>
```

## Development
Features are being worked on that assist with local development that remove the need for dynamodb and support a developer's Slack workspace.

## Running

Clone this project and build with `go build cmd/api/main.go` or build and run with `go run cmd/api/main.go`

## Dependency Management

Dependency management for this project uses go module as of go version 1.11. More information can be found at [go command documentation](https://golang.org/cmd/go/#hdr-Modules__module_versions__and_more).

## Versioning

This project uses [Semantic Versioning](https://semver.org) for the code and associated
docker containers. Versions are tracked as [tags](https://github.com/jitsi/jitsi-slack/tags) on this repository.

## License

This project is licensed under the Apache 2.0 License [LICENSE](LICENSE)
