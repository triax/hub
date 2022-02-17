package models

import "github.com/slack-go/slack"

type (
	// slack.User を使いたいが、
	// map[string]interface{} を含み、
	// これはdatastoreにPUTできないので、
	// 独自のstructを持つ必要がある.
	// https://api.slack.com/methods/users.list#examples
	// https://github.com/slack-go/slack/blob/979adb8fbdf5c51ae1e5021c50c625ccd2051cf8/users.go#L103-L130
	SlackUser struct {
		ID                string       `json:"id"`
		TeamID            string       `json:"team_id"`
		Name              string       `json:"name"`
		Deleted           bool         `json:"deleted"`
		Color             string       `json:"color"`
		RealName          string       `json:"real_name"`
		TZ                string       `json:"tz,omitempty"`
		TZLabel           string       `json:"tz_label"`
		TZOffset          int          `json:"tz_offset"`
		Profile           SlackProfile `json:"profile"`
		IsBot             bool         `json:"is_bot"`
		IsAdmin           bool         `json:"is_admin"`
		IsOwner           bool         `json:"is_owner"`
		IsPrimaryOwner    bool         `json:"is_primary_owner"`
		IsRestricted      bool         `json:"is_restricted"`
		IsUltraRestricted bool         `json:"is_ultra_restricted"`
		IsStranger        bool         `json:"is_stranger"`
		IsAppUser         bool         `json:"is_app_user"`
		IsInvitedUser     bool         `json:"is_invited_user"`
		Has2FA            bool         `json:"has_2fa"`
		HasFiles          bool         `json:"has_files"`
		Presence          string       `json:"presence"`
		Locale            string       `json:"locale"`
		Updated           int64        `json:"updated"`
		// Enterprise EnterpriseUser `json:"enterprise_user,omitempty"`
	}
	SlackProfile struct {
		FirstName             string `json:"first_name"`
		LastName              string `json:"last_name"`
		RealName              string `json:"real_name"`
		RealNameNormalized    string `json:"real_name_normalized"`
		DisplayName           string `json:"display_name"`
		DisplayNameNormalized string `json:"display_name_normalized"`
		Email                 string `json:"email"`
		Skype                 string `json:"skype"`
		Phone                 string `json:"phone"`
		Image24               string `json:"image_24"`
		Image32               string `json:"image_32"`
		Image48               string `json:"image_48"`
		Image72               string `json:"image_72"`
		Image192              string `json:"image_192"`
		Image512              string `json:"image_512"`
		ImageOriginal         string `json:"image_original"`
		Title                 string `json:"title"`
		BotID                 string `json:"bot_id,omitempty"`
		ApiAppID              string `json:"api_app_id,omitempty"`
		StatusText            string `json:"status_text,omitempty"`
		StatusEmoji           string `json:"status_emoji,omitempty"`
		StatusExpiration      int    `json:"status_expiration"`
		Team                  string `json:"team"`
	}

	// slack.TeamInfo を使いたいが、
	// map[string]interface{} を含み、
	// これはdatastoreにPUTできないので、
	// 独自のstructを持つ必要がある.
	// https://api.slack.com/methods/team.info#examples
	// https://github.com/slack-go/slack/blob/93fe17cfad827ebb34316c683f29377c9be910e3/team.go#L19-L25
	SlackTeam struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Domain      string `json:"domain"`
		EmailDomain string `json:"email_domain"`

		// Icon map[string]interface{} `json:"icon"`
		Icon TeamIcon `json:"icon"`
	}
	TeamIcon struct {
		Image34      string `json:"image_34"`
		Image68      string `json:"image_68"`
		Image132     string `json:"image_132"`
		ImageDefault bool   `json:"image_default"`
	}
)

func ConvertSlackAPIUserToInternalUser(user slack.User) SlackUser {
	return SlackUser{
		ID:       user.ID,
		TeamID:   user.TeamID,
		Name:     user.Name,
		Deleted:  user.Deleted,
		Color:    user.Color,
		RealName: user.RealName,
		TZ:       user.TZ,
		TZLabel:  user.TZLabel,
		TZOffset: user.TZOffset,
		Profile: SlackProfile{
			FirstName:             user.Profile.FirstName,
			LastName:              user.Profile.LastName,
			RealName:              user.Profile.RealName,
			RealNameNormalized:    user.Profile.RealNameNormalized,
			DisplayName:           user.Profile.DisplayName,
			DisplayNameNormalized: user.Profile.DisplayNameNormalized,
			Email:                 user.Profile.Email,
			Skype:                 user.Profile.Skype,
			Phone:                 user.Profile.Phone,
			Image24:               user.Profile.Image24,
			Image32:               user.Profile.Image32,
			Image48:               user.Profile.Image48,
			Image72:               user.Profile.Image72,
			Image192:              user.Profile.Image192,
			Image512:              user.Profile.Image512,
			ImageOriginal:         user.Profile.ImageOriginal,
			Title:                 user.Profile.Title,
			BotID:                 user.Profile.BotID,
			ApiAppID:              user.Profile.ApiAppID,
			StatusText:            user.Profile.StatusText,
			StatusEmoji:           user.Profile.StatusEmoji,
			StatusExpiration:      user.Profile.StatusExpiration,
			Team:                  user.Profile.Team,
		},
		IsBot:             user.IsBot,
		IsAdmin:           user.IsAdmin,
		IsOwner:           user.IsOwner,
		IsPrimaryOwner:    user.IsPrimaryOwner,
		IsRestricted:      user.IsRestricted,
		IsUltraRestricted: user.IsUltraRestricted,
		IsStranger:        user.IsStranger,
		IsAppUser:         user.IsAppUser,
		IsInvitedUser:     user.IsInvitedUser,
		Has2FA:            user.Has2FA,
		HasFiles:          user.HasFiles,
		Presence:          user.Presence,
		Locale:            user.Locale,
		Updated:           int64(user.Updated),
	}
}

func ConvertSlackAPITeamToInternalTeam(team slack.TeamInfo) SlackTeam {
	t := SlackTeam{
		ID:          team.ID,
		Name:        team.Name,
		Domain:      team.Domain,
		EmailDomain: team.EmailDomain,
		Icon: TeamIcon{
			Image34:      team.Icon["image_34"].(string),
			Image68:      team.Icon["image_68"].(string),
			Image132:     team.Icon["image_132"].(string),
			ImageDefault: team.Icon["image_default"].(bool),
		},
	}
	if s, ok := team.Icon["image_34"].(string); ok {
		t.Icon.Image34 = s
	}
	if s, ok := team.Icon["image_68"].(string); ok {
		t.Icon.Image68 = s
	}
	if s, ok := team.Icon["image_132"].(string); ok {
		t.Icon.Image132 = s
	}
	if s, ok := team.Icon["image_default"].(bool); ok {
		t.Icon.ImageDefault = s
	}
	return t
}
