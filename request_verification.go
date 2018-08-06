package jitsi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"time"
)

const (
	// RequestTimestampHeader is the header key value for the slack request timestamp.
	RequestTimestampHeader = "X-Slack-Request-Timestamp"
	// RequestSignatureHeader is the header key value for the slack request signature.
	RequestSignatureHeader = "X-Slack-Signature"
	// SignatureVersion is the version of signature validation that is preformed.
	SignatureVersion = "v0"
)

// ValidRequest returns a boolean indicating that a request is validated as originating
// from Slack.
func ValidRequest(slackSigningSecret, requestBody, timestamp, slackSignature string) bool {
	// Check that timestamp is < 5 minutes old
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	now := time.Now().Unix()
	if math.Abs(float64(now)-float64(ts)) > 60*5 {
		return false
	}

	// Concatenate the version number, timestamp and request body
	// using : as a delimiter
	sigBaseString := fmt.Sprintf("%s:%s:%s", SignatureVersion, timestamp, requestBody)

	// Attempt to replicate the signature for ourselves.
	hasher := hmac.New(sha256.New, []byte(slackSigningSecret))
	hasher.Write([]byte(sigBaseString))
	mySignature := fmt.Sprintf(
		"%s=%s",
		SignatureVersion,
		hex.EncodeToString(hasher.Sum(nil)),
	)

	// Compare our signature with Slack's.
	return mySignature == slackSignature
}
