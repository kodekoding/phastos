package slack

type (
	BaseIdName struct {
		Name string `json:"name"`
		Id   string `json:"id"`
	}

	AuthedUser struct {
		Id          string `json:"id"`
		Scope       string `json:"scope"`
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}

	OauthAccess struct {
		AccessToken         string     `json:"access_token"`
		TokenType           string     `json:"token_type"`
		Scope               string     `json:"scope"`
		BotUserId           string     `json:"bot_user_id"`
		AppId               string     `json:"app_id"`
		ExpiresIn           int        `json:"expires_in"`
		RefreshToken        string     `json:"refresh_token"`
		Team                BaseIdName `json:"team"`
		Enterprise          BaseIdName `json:"enterprise"`
		IsEnterpriseInstall bool       `json:"is_enterprise_install"`
		AuthedUser          AuthedUser `json:"authed_user"`
		Error               string     `json:"error,omitempty"`
	}
)
