package jitsi

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	// KeyTeamIDSrvCfg is the dynamo key for storing the team id.
	// This key is the primary index.
	KeyTeamIDSrvCfg = "teamID"
	// KeyServer is the dynamo key for storing the configured server.
	KeyServer = "server"
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
	TeamID string `json:"teamID"`
	Server string `json:"server"`
}

// ServerCfgStore is used to store server configuration for teams.
type ServerCfgStore struct {
	// TableName is the name of the dynamo table where configuration is stored.
	TableName string
	// DB is the client used to access dynamodb.
	DB *dynamodb.DynamoDB
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
	input := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			KeyTeamIDSrvCfg: {
				S: aws.String(data.TeamID),
			},
			KeyServer: {
				S: aws.String(data.Server),
			},
		},
		TableName: aws.String(s.TableName),
	}

	_, err := s.DB.PutItem(input)
	return err
}

// Remove will remove the persistent server configuration for a team. That
// team will use the defaults if no configuration is stored for the team.
func (s *ServerCfgStore) Remove(teamID string) error {
	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			KeyTeamIDSrvCfg: {
				S: aws.String(teamID),
			},
		},
		TableName: aws.String(s.TableName),
	}
	_, err := s.DB.DeleteItem(input)
	return err
}

// Get retrieves the server configuration for a team. This will provide
// the default if no configuration is stored for the team.
func (s *ServerCfgStore) Get(teamID string) (ServerCfg, error) {
	teamIDKey := KeyTeamIDSrvCfg
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames: map[string]*string{"#teamid": &teamIDKey},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":t": {
				S: aws.String(teamID),
			},
		},
		KeyConditionExpression: aws.String("#teamid = :t"),
		TableName:              aws.String(s.TableName),
	}

	result, err := s.DB.Query(queryInput)
	if err != nil {
		return ServerCfg{}, err
	}

	if len(result.Items) < 1 {
		cfg := ServerCfg{
			Server:                  s.DefaultServer,
			TenantScopedURLs:        s.TenantScopedURLs(s.DefaultServer),
			AuthenticatedURLSupport: s.AuthenticatedURLSupport(s.DefaultServer),
		}
		return cfg, nil
	}

	var server string
	err = dynamodbattribute.Unmarshal(result.Items[0][KeyServer], &server)
	if err != nil {
		return ServerCfg{}, err
	}

	return ServerCfg{
		Server:                  server,
		TenantScopedURLs:        s.TenantScopedURLs(server),
		AuthenticatedURLSupport: s.AuthenticatedURLSupport(server),
	}, nil
}
