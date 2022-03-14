package jitsi

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

const (
	KeyTeamID      = "team-id"      // primary key; slack team id
	KeyAccessToken = "access-token" // oauth access token
)

// TokenData is the access token data stored from oauth.
type TokenData struct {
	TeamID      string `json:"team-id"`
	AccessToken string `json:"access-token"`
}

// TokenStore stores and retrieves access tokens from aws dynamodb.
type TokenStore struct {
	TableName string
	DB        *dynamodb.Client
}

// GetToken retrieves the access token stored with the provided team id.
func (t *TokenStore) GetTokenForTeam(teamID string) (*TokenData, error) {
	keyCond := expression.Key(KeyTeamID).Equal(expression.Value(teamID))
	builder := expression.NewBuilder().WithKeyCondition(keyCond)
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}
	queryInput := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		TableName:                 aws.String(t.TableName),
	}
	result, err := t.DB.Query(context.TODO(), queryInput)
	if err != nil {
		return nil, err
	}

	if len(result.Items) < 1 {
		return nil, errors.New(errMissingAuthToken)
	}

	var token string
	err = attributevalue.Unmarshal(result.Items[0][KeyAccessToken], &token)
	if err != nil {
		return nil, err
	}

	return &TokenData{
		TeamID:      teamID,
		AccessToken: token,
	}, nil
}

// Store will store access token data.
func (t *TokenStore) Store(data *TokenData) error {
	av, err := attributevalue.MarshalMap(map[string]string{
		KeyTeamID:      data.TeamID,
		KeyAccessToken: data.AccessToken,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%v TableNameTableName", t.TableName)
	fmt.Printf("%v avavavavavavavavav", av)

	_, err = t.DB.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(t.TableName),
		Item:      av,
	})
	return err
}

// Remove will remove access token data for the user.
func (t *TokenStore) Remove(teamID string) error {
	av, err := attributevalue.MarshalMap(map[string]string{
		KeyTeamID: teamID,
	})
	dii := &dynamodb.DeleteItemInput{
		TableName: aws.String(t.TableName),
		Key:       av,
	}
	_, err = t.DB.DeleteItem(context.TODO(), dii)
	return err
}
