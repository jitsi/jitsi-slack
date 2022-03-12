module github.com/jitsi/jitsi-slack

go 1.16

require (
	github.com/aws/aws-sdk-go-v2 v1.2.0
	github.com/aws/aws-sdk-go-v2/config v1.1.1
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.0.2
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression v1.0.2
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.1.1
	github.com/caarlos0/env/v6 v6.5.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/jitsi/prometheus-stats v0.1.0
	github.com/justinas/alice v1.2.0
	github.com/prometheus/client_golang v1.9.0
	github.com/rs/zerolog v1.20.0
	github.com/slack-go/slack v0.10.2
	github.com/vincent-petithory/dataurl v0.0.0-20191104211930-d1553a71de50
	golang.org/x/sys v0.0.0-20210303074136-134d130e1a04 // indirect
)
