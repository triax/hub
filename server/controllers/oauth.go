package controllers

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/golang-jwt/jwt"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"
)

const (
	nonce = "xxxyyyzzz" // TODO: Fix
	state = "temp"      // TODO: Fix
)

func AuthStart(w http.ResponseWriter, req *http.Request) {
	// "https://slack.com/.well-known/openid-configuration"
	authorizationEndpoint := "https://slack.com/openid/connect/authorize"
	redirectURI := "https://" + req.Host + "/auth/callback"
	if destination := req.URL.Query().Get("goto"); destination != "" {
		redirectURI += ("?goto=" + url.QueryEscape(destination))
	}
	u, _ := url.Parse(authorizationEndpoint)
	// https://api.slack.com/authentication/sign-in-with-slack#request
	q := url.Values{
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"client_id":     {os.Getenv("SLACK_CLIENT_ID")},
		"team":          {os.Getenv("SLACK_INSTALLED_TEAM_ID")},
		"state":         {state},
		"nonce":         {nonce},
		"redirect_uri":  {redirectURI},
	}
	u.RawQuery = q.Encode()
	http.SetCookie(w, &http.Cookie{
		Name:  server.SessionCookieName,
		Value: "", Path: "/", Expires: time.Unix(0, 0),
		MaxAge: -1, HttpOnly: true,
	})

	http.Redirect(w, req, u.String(), http.StatusTemporaryRedirect)
}

func AuthCallback(w http.ResponseWriter, req *http.Request) {

	if errmsg := req.URL.Query().Get("error"); errmsg != "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `%s\n<a href="/login">Back to login</a>`, html.EscapeString(errmsg))
		return
	}

	code := req.URL.Query().Get("code")
	if code == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No oauth client code given."))
		return
	}

	redirectURI := "https://" + req.Host + "/auth/callback"
	destination := req.URL.Query().Get("goto")
	if destination != "" {
		redirectURI += ("?goto=" + url.QueryEscape(destination))
	}

	// https://api.slack.com/authentication/sign-in-with-slack#exchange
	tokenExchangeEndpoint := "https://slack.com/api/openid.connect.token"
	q := url.Values{
		"client_id":     {os.Getenv("SLACK_CLIENT_ID")},
		"client_secret": {os.Getenv("SLACK_CLIENT_SECRET")},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}

	exchange, err := http.NewRequest("POST", tokenExchangeEndpoint, strings.NewReader(q.Encode()))
	if err != nil {
		http.Redirect(w, req, fmt.Sprintf("/errors?code=%d&error=%s", 6001, err.Error()), http.StatusTemporaryRedirect)
		return
	}
	exchange.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(exchange)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		w.WriteHeader(res.StatusCode)
		w.Write([]byte(res.Status))
		return
	}

	token := models.SlackOpenIDConnectToken{}
	// token := map[string]interface{}{}
	if err := json.NewDecoder(res.Body).Decode(&token); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	if !token.OK {
		http.Redirect(w, req, fmt.Sprintf("/errors?code=%d&error=%+v", 6002, token), http.StatusTemporaryRedirect)
		return
	}

	// So far, we DO NOT use user's access_token from the server,
	// We DO NOT store the access_token, but just fetch user information
	// to generate session key as a JWT token.
	info, err := FetchCurrentUserInfo(token.AccessToken)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("GET userInfo: " + err.Error()))
		return
	}

	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		http.Redirect(w, req, fmt.Sprintf("/errors?code=%d&error=%s", 4001, err.Error()), http.StatusTemporaryRedirect)
		return
	}
	defer client.Close()

	member := models.Member{}
	if err := client.Get(ctx,
		datastore.NameKey(models.KindMember, info.Sub, nil),
		&member,
	); err != nil && !models.IsFiledMismatch(err) {
		if err == datastore.ErrNoSuchEntity {
			http.Redirect(
				w, req, fmt.Sprintf("/errors?code=%d", server.ErrorMemberNotSyncedYet),
				http.StatusTemporaryRedirect,
			)
			return
		}
		http.Redirect(w, req, fmt.Sprintf("/errors?code=%d&error=%s", 4002, err.Error()), http.StatusTemporaryRedirect)
		return
	}

	t := jwt.New(jwt.GetSigningMethod(os.Getenv("JWT_SIGNING_METHOD")))
	t.Claims = &models.SessionUserClaims{
		StandardClaims: &jwt.StandardClaims{
			ExpiresAt: time.Now().Add(server.ServerSessionExpire).Unix(),
		},
		SlackID: member.Slack.ID,
	}

	tokenstr, err := t.SignedString([]byte(os.Getenv("JWT_SIGNING_KEY")))
	if err != nil {
		http.Redirect(w, req, fmt.Sprintf("/errors?code=%d&error=%s", 6003, err.Error()), http.StatusTemporaryRedirect)
		return
	}

	fmt.Printf("[DEBUG] %s = length(%d)\n", member.Slack.RealName, len(tokenstr))

	http.SetCookie(w, &http.Cookie{
		Name:    server.SessionCookieName,
		Value:   tokenstr,
		Path:    "/",
		Expires: time.Now().Add(server.ServerSessionExpire),
	})

	if destination != "" {
		http.Redirect(w, req, destination, http.StatusTemporaryRedirect)
	} else {
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
	}
}

func FetchCurrentUserInfo(token string) (info *models.SlackOpenIDUserInfo, err error) {
	u := "https://slack.com/api/openid.connect.userInfo"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	info = new(models.SlackOpenIDUserInfo)
	if err := json.NewDecoder(res.Body).Decode(info); err != nil {
		return nil, err
	}
	if !info.OK {
		return nil, fmt.Errorf("error=%s,warning=%s", info.Error, info.Warning)
	}
	return info, nil
}
