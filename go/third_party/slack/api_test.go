package slack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	slackentity "github.com/kodekoding/phastos/v2/go/entity/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errorTransport always fails HTTP requests, used to test resty error paths.
type errorTransport struct{}

func (errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network error")
}

func TestNewSlack(t *testing.T) {
	s := NewSlack("xoxb-test-token", "client-id", "client-secret")
	assert.NotNil(t, s)
	assert.Equal(t, "xoxb-test-token", s.botToken)
	assert.Equal(t, "client-id", s.clientID)
	assert.Equal(t, "client-secret", s.clientSecret)
}

func TestSlackStruct(t *testing.T) {
	s := &slack{
		botToken:     "xoxb-123",
		clientID:     "cid",
		clientSecret: "csecret",
	}
	assert.Equal(t, "xoxb-123", s.botToken)
	assert.Equal(t, "cid", s.clientID)
	assert.Equal(t, "csecret", s.clientSecret)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "https://slack.com/api", prefixURL)
	assert.Equal(t, "channel", channelField)
}

func TestNewCURLDefaultContentType(t *testing.T) {
	s := NewSlack("xoxb-test", "cid", "csecret")
	req := s.newCURL(context.Background())
	assert.NotNil(t, req)
}

func TestNewCURLCustomContentType(t *testing.T) {
	s := NewSlack("xoxb-test", "cid", "csecret")
	req := s.newCURL(context.Background(), "application/x-www-form-urlencoded")
	assert.NotNil(t, req)
}

func TestCreateNewChannel_Error(t *testing.T) {
	s := NewSlack("xoxb-test", "cid", "csecret")
	s.client.SetTransport(errorTransport{})
	_, err := s.CreateNewChannel(context.Background(), "test-channel")
	assert.Error(t, err)
}

func TestInviteUserToChannel_Error(t *testing.T) {
	s := NewSlack("xoxb-test", "cid", "csecret")
	s.client.SetTransport(errorTransport{})
	err := s.InviteUserToChannel(context.Background(), "C123", "U1")
	assert.Error(t, err)
}

func TestArchiveChannel_Error(t *testing.T) {
	s := NewSlack("xoxb-test", "cid", "csecret")
	s.client.SetTransport(errorTransport{})
	err := s.ArchiveChannel(context.Background(), "C123")
	assert.Error(t, err)
}

func TestAddReminderToChannel_Error(t *testing.T) {
	s := NewSlack("xoxb-test", "cid", "csecret")
	s.client.SetTransport(errorTransport{})
	err := s.AddReminderToChannel(context.Background(), "C123", "standup", "10:00")
	assert.Error(t, err)
}

func TestPostMessageText_Error(t *testing.T) {
	s := NewSlack("xoxb-test", "cid", "csecret")
	s.client.SetTransport(errorTransport{})
	err := s.PostMessageText(context.Background(), "C123", "hello")
	assert.Error(t, err)
}

func TestPostMessageBlocks_Error(t *testing.T) {
	s := NewSlack("xoxb-test", "cid", "csecret")
	s.client.SetTransport(errorTransport{})
	err := s.PostMessageBlocks(context.Background(), "C123", "{}")
	assert.Error(t, err)
}

func TestGetOauthAccess_Error(t *testing.T) {
	s := NewSlack("xoxb-test", "cid", "csecret")
	s.client.SetTransport(errorTransport{})
	_, err := s.GetOauthAccess(context.Background(), "code")
	assert.Error(t, err)
}

// redirectTransport is a custom http.RoundTripper that redirects all requests
// to the test server, preserving the original request method, headers, and body.
type redirectTransport struct {
	server *httptest.Server
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Build the redirect URL using test server + original path
	redirectURL := rt.server.URL + req.URL.Path
	if req.URL.RawQuery != "" {
		redirectURL += "?" + req.URL.RawQuery
	}
	// Create a new request with the same method, body, and headers but different URL
	newReq := req.Clone(req.Context())
	newReq.URL, _ = url.Parse(redirectURL)
	return rt.server.Client().Transport.RoundTrip(newReq)
}

// newSlackWithTestServer creates a slack instance that redirects all requests to the test server
func newSlackWithTestServer(handler http.HandlerFunc) *slack {
	server := httptest.NewServer(handler)
	s := NewSlack("xoxb-test", "cid", "csecret")
	s.client.SetTransport(&redirectTransport{server: server})
	return s
}

func TestCreateNewChannel(t *testing.T) {
	s := newSlackWithTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "conversation.created")

		var body slackentity.ChannelCreateRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "test-channel", body.Name)
		assert.False(t, body.IsPrivate)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"C123","name":"test-channel"}}`))
	})

	result, err := s.CreateNewChannel(context.Background(), "test-channel")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Ok)
}

func TestCreateNewChannelPrivate(t *testing.T) {
	s := newSlackWithTestServer(func(w http.ResponseWriter, r *http.Request) {
		var body slackentity.ChannelCreateRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "private-channel", body.Name)
		assert.True(t, body.IsPrivate)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"C456","name":"private-channel","is_private":true}}`))
	})

	result, err := s.CreateNewChannel(context.Background(), "private-channel", true)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestInviteUserToChannel(t *testing.T) {
	s := newSlackWithTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "conversation.invite")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	err := s.InviteUserToChannel(context.Background(), "C123", "U1", "U2")
	assert.NoError(t, err)
}

func TestArchiveChannel(t *testing.T) {
	s := newSlackWithTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "conversation.archive")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	err := s.ArchiveChannel(context.Background(), "C123")
	assert.NoError(t, err)
}

func TestAddReminderToChannel(t *testing.T) {
	s := newSlackWithTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "reminders.add")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	err := s.AddReminderToChannel(context.Background(), "C123", "standup meeting", "10:00")
	assert.NoError(t, err)
}

func TestPostMessageText(t *testing.T) {
	s := newSlackWithTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "chat.postMessage")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	err := s.PostMessageText(context.Background(), "C123", "hello world")
	assert.NoError(t, err)
}

func TestPostMessageBlocks(t *testing.T) {
	s := newSlackWithTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "chat.postMessage")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	err := s.PostMessageBlocks(context.Background(), "C123", `[{"type":"section"}]`)
	assert.NoError(t, err)
}

func TestGetOauthAccess(t *testing.T) {
	s := newSlackWithTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "oauth.v2.access")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"access_token":"xoxb-12345","token_type":"bot","scope":"chat:write","bot_user_id":"U123","app_id":"A123","team":{"name":"TestTeam","id":"T123"},"enterprise":{"name":"","id":""},"is_enterprise_install":false,"authed_user":{"id":"U456","scope":"chat:write","access_token":"xoxp-123","token_type":"user"}}`))
	})

	result, err := s.GetOauthAccess(context.Background(), "code-callback-123")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Ok)
	assert.Equal(t, "xoxb-12345", result.Data.AccessToken)
}

// Note: The production code uses resty's Post() which returns error only on network/transport errors,
// not on HTTP status errors. Resty treats HTTP 5xx as valid responses unless SetError() is used.
// So we can't easily trigger the error paths without modifying production code.
// The success path tests already cover most of the code.

func TestGetOauthAccessWithError(t *testing.T) {
	s := newSlackWithTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"access_token":"","token_type":"","scope":"","bot_user_id":"","app_id":"","team":{"name":"","id":""},"enterprise":{"name":"","id":""},"is_enterprise_install":false,"authed_user":{"id":"","scope":"","access_token":"","token_type":""},"error":"invalid_code"}`))
	})

	result, err := s.GetOauthAccess(context.Background(), "invalid-code")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "invalid_code", result.Error)
}
