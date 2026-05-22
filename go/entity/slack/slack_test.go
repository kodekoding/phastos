package slack

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChannelCreateRequest(t *testing.T) {
	req := ChannelCreateRequest{
		Name:      "test-channel",
		IsPrivate: false,
	}
	assert.Equal(t, "test-channel", req.Name)
	assert.False(t, req.IsPrivate)
}

func TestChannelCreateRequestJSON(t *testing.T) {
	req := ChannelCreateRequest{Name: "my-channel", IsPrivate: true}
	b, err := json.Marshal(req)
	assert.NoError(t, err)
	assert.Contains(t, string(b), "my-channel")
	assert.Contains(t, string(b), "is_private")
}

func TestChannelStruct(t *testing.T) {
	ch := Channel{
		ID:         "C123",
		Name:       "general",
		IsChannel:  true,
		IsPrivate:  false,
		NumMembers: 50,
	}
	assert.Equal(t, "C123", ch.ID)
	assert.Equal(t, "general", ch.Name)
	assert.True(t, ch.IsChannel)
	assert.Equal(t, 50, ch.NumMembers)
}

func TestChannelWithTopicPurpose(t *testing.T) {
	ch := Channel{
		ID: "C456",
		Topic: Topic{
			Value:   "Discussion",
			Creator: "U123",
			LastSet: 1234567890,
		},
		Purpose: Purpose{
			Value:   "General chat",
			Creator: "U456",
			LastSet: 1234567890,
		},
	}
	assert.Equal(t, "Discussion", ch.Topic.Value)
	assert.Equal(t, "General chat", ch.Purpose.Value)
}

func TestChannelMemberResponse(t *testing.T) {
	ch := Channel{
		ChannelMemberResponse: ChannelMemberResponse{
			Members: []string{"U1", "U2", "U3"},
		},
	}
	assert.Len(t, ch.Members, 3)
}

func TestOauthAccessStruct(t *testing.T) {
	oauth := OauthAccess{
		AccessToken: "xoxb-test",
		TokenType:   "bot",
		Scope:       "chat:write",
		Team:        BaseIdName{Name: "MyTeam", Id: "T123"},
		AuthedUser: AuthedUser{
			Id:          "U123",
			AccessToken: "xoxp-test",
		},
	}
	assert.Equal(t, "xoxb-test", oauth.AccessToken)
	assert.Equal(t, "MyTeam", oauth.Team.Name)
	assert.Equal(t, "U123", oauth.AuthedUser.Id)
}

func TestReminderStruct(t *testing.T) {
	rem := Reminder{
		Id:        "R123",
		Creator:   "U123",
		User:      "U456",
		Text:      "Standup meeting",
		Recurring: true,
		Time:      900,
	}
	assert.Equal(t, "R123", rem.Id)
	assert.Equal(t, "Standup meeting", rem.Text)
	assert.True(t, rem.Recurring)
}

func TestResponseTypeChannel(t *testing.T) {
	resp := Response[Channel]{
		Ok:    true,
		Data:  Channel{ID: "C1", Name: "test"},
		Error: "",
	}
	assert.True(t, resp.Ok)
	assert.Equal(t, "C1", resp.Data.ID)
}

func TestResponseTypeOauthAccess(t *testing.T) {
	resp := Response[OauthAccess]{
		Ok: true,
		Data: OauthAccess{
			AccessToken: "token",
		},
	}
	assert.True(t, resp.Ok)
	assert.Equal(t, "token", resp.Data.AccessToken)
}

func TestResponseTypeReminder(t *testing.T) {
	resp := Response[Reminder]{
		Ok:    true,
		Data:  Reminder{Id: "R1"},
		Error: "",
	}
	assert.True(t, resp.Ok)
	assert.Equal(t, "R1", resp.Data.Id)
}

func TestBaseIdNameStruct(t *testing.T) {
	bin := BaseIdName{Name: "TestApp", Id: "A123"}
	assert.Equal(t, "TestApp", bin.Name)
	assert.Equal(t, "A123", bin.Id)
}

func TestLatestStruct(t *testing.T) {
	latest := Latest{
		Type: "message",
		User: "U123",
		Text: "Hello world",
		Ts:   "1234567890.123456",
	}
	assert.Equal(t, "message", latest.Type)
	assert.Equal(t, "Hello world", latest.Text)
}

func TestChannelJSONRoundTrip(t *testing.T) {
	ch := Channel{
		ID:   "C789",
		Name: "test-channel",
	}
	b, err := json.Marshal(ch)
	assert.NoError(t, err)

	var result Channel
	err = json.Unmarshal(b, &result)
	assert.NoError(t, err)
	assert.Equal(t, "C789", result.ID)
	assert.Equal(t, "test-channel", result.Name)
}
