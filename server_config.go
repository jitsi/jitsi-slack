package jitsi

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

const (
	// KeyTeamIDSrvCfg is the dynamo key for storing the team id.
	// This key is the primary index.
	KeyTeamIDSrvCfg = "team-id"
	// KeyServer is the dynamo key for storing the configured server.
	KeyServer = "server-url"
)

// ServerCfg is the server configuration for a team.
type ServerCfg struct {
	// Server is the host server for meetings. (e.g. https://meet.jit.si)
	Server string
	// TenantScopedURLs indicates whether or nto urls generated for meetings
	// on the server should be tenant scoped or not.
	// (e.g. https://meet.jit.si/team/room or https://meet.jit.si/room)
	TenantScopedURLs bool
	// AuthenticatedURLSupport indicates whether or not authenticated urls
	// are supported.
	AuthenticatedURLSupport bool
}

// ServerCfgData is the server configuration data that is stored for teams.
type ServerCfgData struct {
	TeamID string `json:"team-id"`
	Server string `json:"server-url"`
}

// ServerCfgStore is used to store server configuration for teams.
type ServerCfgStore struct {
	// TableName is the name of the dynamo table where configuration is stored.
	TableName string
	// DB is the client used to access dynamodb.
	DB *dynamodb.Client
	// DefaultServer is the server host to use if none has been configured.
	DefaultServer string
	// TenantScopedURLs returns whether or not meeting urls should be
	// scoped with a tenant (e.g. https://meet.jit.si/tenant/room)
	TenantScopedURLs func(string) bool
	// AuthenticatedURLSupport returns whether or not the server supports
	// authenticated urls.
	AuthenticatedURLSupport func(string) bool
}

// Store will persist a portion of the server configuration for a team.
func (s *ServerCfgStore) Store(data *ServerCfgData) error {
	av, err := attributevalue.MarshalMap(map[string]string{
		KeyTeamIDSrvCfg: data.TeamID,
		KeyServer:       data.Server,
	})
	if err != nil {
		return err
	}
	_, err = s.DB.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(s.TableName),
		Item:      av,
	})
	return err
}

// Remove will remove the persistent server configuration for a team. That
// team will use the defaults if no configuration is stored for the team.
func (s *ServerCfgStore) Remove(teamID string) error {
	av, err := attributevalue.MarshalMap(map[string]string{KeyTeamIDSrvCfg: teamID})
	if err != nil {
		return err
	}
	dii := &dynamodb.DeleteItemInput{
		TableName: aws.String(s.TableName),
		Key:       av,
	}
	_, err = s.DB.DeleteItem(context.TODO(), dii)
	return err
}

// Get retrieves the server configuration for a team. This will provide
// the default if no configuration is stored for the team.
func (s *ServerCfgStore) Get(teamID string) (ServerCfg, error) {
	keyCond := expression.Key(KeyTeamIDSrvCfg).Equal(expression.Value(teamID))
	builder := expression.NewBuilder().WithKeyCondition(keyCond)
	expr, err := builder.Build()
	if err != nil {
		return ServerCfg{}, err
	}
	queryInput := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		TableName:                 aws.String(s.TableName),
	}
	result, err := s.DB.Query(context.TODO(), queryInput)

	// return default server if an item is not found
	if len(result.Items) < 1 {
		cfg := ServerCfg{
			Server:                  s.DefaultServer,
			TenantScopedURLs:        s.TenantScopedURLs(s.DefaultServer),
			AuthenticatedURLSupport: s.AuthenticatedURLSupport(s.DefaultServer),
		}
		return cfg, nil
	}

	var server string
	err = attributevalue.Unmarshal(result.Items[0][KeyServer], &server)
	if err != nil {
		return ServerCfg{}, err
	}

	return ServerCfg{
		Server:                  server,
		TenantScopedURLs:        s.TenantScopedURLs(server),
		AuthenticatedURLSupport: s.AuthenticatedURLSupport(server),
	}, nil
}
