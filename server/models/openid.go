package models

type (
	/**
	 * - https://slack.com/.well-known/openid-configuration
	 * - https://api.slack.com/authentication/sign-in-with-slack#discover
	 */
	SlackOpenIDConfig struct {

		// e.g. "https://slack.com"
		Issuer string `json:"issuer"`

		// e.g. "https://slack.com/openid/connect/authorize"
		AuthorizationEndpoint string `json:"authorization_endpoint"`

		// e.g. "https://slack.com/api/openid.connect.token"
		TokenEndpoint string `json:"token_endpoint"`

		// e.g. "https://slack.com/api/openid.connect.userInfo"
		UserInfoEndpoint string `json:"userinfo_endpoint"`

		// e.g. ["openid","profile","email"]
		ScopesSupported []string `json:"scopes_supported"`

		// e.g. ["code"]
		ResponseTypeSupported []string `json:"response_types_supported"`

		// e.g. ["form_post"]
		ResponseModeSupported []string `json:"response_modes_supported"`

		// e.g. ["authorization_code"]
		GrantTypesSupported []string `json:"grant_types_supported"`

		JWKsURI                          string   `json:"jwks_uri"`
		SubjectTypesSupported            []string `json:"subject_types_supported"`
		IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`

		ClaimsSupported                   []string `json:"claims_supported"`
		ClaimsParameterSupported          bool     `json:"claims_parameter_supported"`
		RequestParameterSupported         bool     `json:"request_parameter_supported"`
		RequestURIParameterSupported      bool     `json:"request_uri_parameter_supported"`
		TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	}

	SlackOpenIDConnectToken struct {
		OK          bool   `json:"ok"`
		AccessToken string `json:"access_token"`
		TokenType   string `jsonn:"token_type"`
		IDToken     string `jsonn:"id_token"` // JWT

		// FIXME: Check behavior
		Error   string
		Warning string
	}

	SlackOpenIDUserInfo struct {
		// Meta
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Warning string `json:"warning"`
		// OpenID standard (== UserID)
		Sub string `json:"sub"`
		// User
		UserID            string `json:"https://slack.com/user_id"`
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		DateEmailVerified int    `json:"date_email_verified"`
		Name              string `json:"name"`
		Picture           string `json:"picture"`
		GivenName         string `json:"given_name"`
		FamilyName        string `json:"family_name"`
		Locale            string `json:"locale"`
		UserImage24       string `json:"https://slack.com/user_image_24"`
		UserImage32       string `json:"https://slack.com/user_image_32"`
		UserImage48       string `json:"https://slack.com/user_image_48"`
		UserImage72       string `json:"https://slack.com/user_image_72"`
		UserImage512      string `json:"https://slack.com/user_image_512"`
		UserImage192      string `json:"https://slack.com/user_image_192"`
		// Team
		TeamID           string `json:"https://slack.com/team_id"`
		TeamName         string `json:"https://slack.com/team_name"`
		TeamDomain       string `json:"https://slack.com/team_domain"`
		TeamImage34      string `json:"https://slack.com/team_image_34"`
		TeamImage44      string `json:"https://slack.com/team_image_44"`
		TeamImage68      string `json:"https://slack.com/team_image_68"`
		TeamImage88      string `json:"https://slack.com/team_image_88"`
		TeamImage102     string `json:"https://slack.com/team_image_102"`
		TeamImage132     string `json:"https://slack.com/team_image_132"`
		TeamImage230     string `json:"https://slack.com/team_image_230"`
		TeamImageDefault string `json:"https://slack.com/team_image_default"`
	}
	/**
	 * {
	 *   "ok": true,
	 *   "error": "",
	 *   "warning": "",
	 *   "sub": "U9MD7M0NS",
	 *   "https://slack.com/user_id": "U9MD7M0NS",
	 *   "email": "otiai10@xxxxxx.com",
	 *   "email_verified": true,
	 *   "date_email_verified": 1622282417,
	 *   "name": "Hiromu Ochiai",
	 *   "picture": "https://avatars.slack-edge.com/2018-03-16/331009079874_3dae47c3c219f519d3da_512.jpg",
	 *   "given_name": "Hiromu",
	 *   "family_name": "Ochiai",
	 *   "locale": "en-US",
	 *   "https://slack.com/user_image_24": "https://avatars.slack-edge.com/2018-03-16/331009079874_3dae47c3c219f519d3da_24.jpg",
	 *   "https://slack.com/user_image_32": "https://avatars.slack-edge.com/2018-03-16/331009079874_3dae47c3c219f519d3da_32.jpg",
	 *   "https://slack.com/user_image_48": "https://avatars.slack-edge.com/2018-03-16/331009079874_3dae47c3c219f519d3da_48.jpg",
	 *   "https://slack.com/user_image_72": "https://avatars.slack-edge.com/2018-03-16/331009079874_3dae47c3c219f519d3da_72.jpg",
	 *   "https://slack.com/user_image_512": "https://avatars.slack-edge.com/2018-03-16/331009079874_3dae47c3c219f519d3da_512.jpg",
	 *   "https://slack.com/user_image_192": "https://avatars.slack-edge.com/2018-03-16/331009079874_3dae47c3c219f519d3da_192.jpg",
	 *   "https://slack.com/team_id": "T9LHPRHA6",
	 *   "https://slack.com/team_name": "Triax",
	 *   "https://slack.com/team_domain": "triax-football",
	 *   "https://slack.com/team_image_34": "https://avatars.slack-edge.com/2018-03-08/326510858803_cfa1bba5e3de9862d0ac_34.png",
	 *   "https://slack.com/team_image_44": "https://avatars.slack-edge.com/2018-03-08/326510858803_cfa1bba5e3de9862d0ac_44.png",
	 *   "https://slack.com/team_image_68": "https://avatars.slack-edge.com/2018-03-08/326510858803_cfa1bba5e3de9862d0ac_68.png",
	 *   "https://slack.com/team_image_88": "https://avatars.slack-edge.com/2018-03-08/326510858803_cfa1bba5e3de9862d0ac_88.png",
	 *   "https://slack.com/team_image_102": "https://avatars.slack-edge.com/2018-03-08/326510858803_cfa1bba5e3de9862d0ac_102.png",
	 *   "https://slack.com/team_image_132": "https://avatars.slack-edge.com/2018-03-08/326510858803_cfa1bba5e3de9862d0ac_132.png",
	 *   "https://slack.com/team_image_230": "https://avatars.slack-edge.com/2018-03-08/326510858803_cfa1bba5e3de9862d0ac_230.png",
	 *   "https://slack.com/team_image_default": ""
	 * }
	 */
)
