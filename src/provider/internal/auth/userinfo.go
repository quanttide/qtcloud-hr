package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized")

type Principal struct {
	Subject string
}

type UserInfoAuthorizer interface {
	Authorize(ctx context.Context, authorization string) (Principal, error)
}

type RemoteUserInfoAuthorizer struct {
	Endpoint   string
	HTTPClient *http.Client
}

func NewRemoteUserInfoAuthorizer(endpoint string, client *http.Client) (*RemoteUserInfoAuthorizer, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, errors.New("userinfo endpoint is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &RemoteUserInfoAuthorizer{Endpoint: endpoint, HTTPClient: client}, nil
}

func (a *RemoteUserInfoAuthorizer) Authorize(ctx context.Context, authorization string) (Principal, error) {
	if !validBearerHeader(authorization) {
		return Principal{}, ErrUnauthorized
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.Endpoint, nil)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: build userinfo request", ErrUnauthorized)
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Accept", "application/json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: userinfo request failed", ErrUnauthorized)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Principal{}, ErrUnauthorized
	}

	var payload struct {
		Subject string `json:"sub"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Principal{}, fmt.Errorf("%w: invalid userinfo response", ErrUnauthorized)
	}
	payload.Subject = strings.TrimSpace(payload.Subject)
	if payload.Subject == "" {
		return Principal{}, fmt.Errorf("%w: userinfo subject is missing", ErrUnauthorized)
	}
	return Principal{Subject: payload.Subject}, nil
}

func validBearerHeader(value string) bool {
	parts := strings.Fields(value)
	return len(parts) == 2 && strings.EqualFold(parts[0], "bearer") && parts[1] != ""
}
