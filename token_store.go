package jitsi

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	// KeyTeamID is the dynamo key for storing the team id.
	// this key is set as the secondary index.
	KeyTeamID = "team-id"
	// KeyUserID is the dynamo key for storing the user id.
	// this is the primary.
	KeyUserID = "user-id"
	// KeyBotToken is the dynamo key for storing the bot token.
	KeyBotToken = "bot-token"
	// KeyBotUserID is the dynamo key for storing the bot user id.
	KeyBotUserID = "bot-user-id"
	// KeyAccessToken is the dynamo ke for storing the access token.
	KeyAccessToken = "access-token"
)

// TokenData is the access token data stored from oauth.
type TokenData struct {
	TeamID      string `json:"team-id"`
	UserID      string `json:"user-id"`
	BotToken    string `json:"bot-token"`
	BotUserID   string `json:"bot-user-id"`
	AccessToken string `json:"access-token"`
}

// TokenStore stores and retrieves access tokens from aws dynamodb.
type TokenStore struct {
	TableName string
	DB        *dynamodb.DynamoDB
}

// GetTokenForTeam retrieves the first bot token stored with the provided team id.
func (t *TokenStore) GetTokenForTeam(teamID string) (*TokenData, error) {
	teamIDKey := KeyTeamID
	queryLimit := int64(1)
	queryInput := &dynamodb.QueryInput{
		TableName:                aws.String(t.TableName),
		IndexName:                aws.String(fmt.Sprintf("%s-index", KeyTeamID)),
		ExpressionAttributeNames: map[string]*string{"#teamid": &teamIDKey},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":v1": {
				S: aws.String(teamID),
			},
		},
		KeyConditionExpression: aws.String("#teamid = :v1"),
		Limit:                  &queryLimit,
	}
	result, err := t.DB.Query(queryInput)
	if err != nil {
		return nil, err
	}

	if len(result.Items) < 1 {
		return nil, errors.New(errMissingAuthToken)
	}

	var d TokenData
	err = dynamodbattribute.UnmarshalMap(result.Items[0], &d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Store will store access token data.
func (t *TokenStore) Store(data *TokenData) error {
	input := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			KeyTeamID: {
				S: aws.String(data.TeamID),
			},
			KeyUserID: {
				S: aws.String(data.UserID),
			},
			KeyBotToken: {
				S: aws.String(data.BotToken),
			},
			KeyBotUserID: {
				S: aws.String(data.BotUserID),
			},
			KeyAccessToken: {
				S: aws.String(data.AccessToken),
			},
		},
		TableName: aws.String(t.TableName),
	}

	_, err := t.DB.PutItem(input)
	return err
}

// Remove will remove access token data for the user.
func (t *TokenStore) Remove(userID string) error {
	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			KeyUserID: {
				S: aws.String(userID),
			},
		},
		TableName: aws.String(t.TableName),
	}
	_, err := t.DB.DeleteItem(input)
	return err
}
