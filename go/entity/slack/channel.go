package slack

type (
	ChannelCreateRequest struct {
		Name      string `json:"name"`
		IsPrivate bool   `json:"is_private"`
	}

	Topic struct {
		Value   string `json:"value"`
		Creator string `json:"creator"`
		LastSet int    `json:"last_set"`
	}
	Purpose struct {
		Value   string `json:"value"`
		Creator string `json:"creator"`
		LastSet int    `json:"last_set"`
	}

	Latest struct {
		Type string `json:"type"`
		User string `json:"user"`
		Text string `json:"text"`
		Ts   string `json:"ts"`
	}

	ChannelMemberResponse struct {
		Members          []string `json:"members"`
		ResponseMetadata struct {
			NextCursor string `json:"next_cursor"`
		} `json:"response_metadata"`
	}

	Channel struct {
		ID                 string        `json:"id"`
		Name               string        `json:"name"`
		IsChannel          bool          `json:"is_channel"`
		IsGroup            bool          `json:"is_group"`
		IsIm               bool          `json:"is_im"`
		Created            int           `json:"created"`
		Creator            string        `json:"creator"`
		IsArchived         bool          `json:"is_archived"`
		IsGeneral          bool          `json:"is_general"`
		Unlinked           int           `json:"unlinked"`
		NameNormalized     string        `json:"name_normalized"`
		IsShared           bool          `json:"is_shared"`
		IsExtShared        bool          `json:"is_ext_shared"`
		IsOrgShared        bool          `json:"is_org_shared"`
		PendingShared      []interface{} `json:"pending_shared"`
		IsPendingExtShared bool          `json:"is_pending_ext_shared"`
		IsMember           bool          `json:"is_member"`
		IsPrivate          bool          `json:"is_private"`
		IsMpim             bool          `json:"is_mpim"`
		LastRead           string        `json:"last_read"`
		Latest             Latest        `json:"latest"`
		UnreadCount        int           `json:"unread_count"`
		UnreadCountDisplay int           `json:"unread_count_display"`
		Topic              Topic         `json:"topic"`
		Purpose            Purpose       `json:"purpose"`
		PreviousNames      []string      `json:"previous_names"`
		Priority           int           `json:"priority"`
		Locale             string        `json:"locale"`
		NumMembers         int           `json:"num_members"`
		Error              string        `json:"error,omitempty"`
		ChannelMemberResponse
	}
)
