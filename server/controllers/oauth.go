package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/triax/hub/server/models"
)

const (
	nonce = "xxxyyyzzz" // TODO: Fix
	state = "temp"      // TODO: Fix
)

func AuthStart(w http.ResponseWriter, req *http.Request) {
	// "https://slack.com/.well-known/openid-configuration"
	authorizationEndpoint := "https://slack.com/openid/connect/authorize"
	u, _ := url.Parse(authorizationEndpoint)
	// https://api.slack.com/authentication/sign-in-with-slack#request
	q := url.Values{
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"client_id":     {os.Getenv("SLACK_CLIENT_ID")},
		"state":         {state},
		"nonce":         {nonce},
		"redirect_uri":  {"https://" + req.Host + "/auth/callback"},
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, req, u.String(), http.StatusTemporaryRedirect)
}

func AuthCallback(w http.ResponseWriter, req *http.Request) {

	if errmsg := req.URL.Query().Get("error"); errmsg != "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `%s\n<a href="/login">Back to login</a>`, errmsg)
		return
	}

	code := req.URL.Query().Get("code")
	if code == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No oauth client code given."))
		return
	}
	// https://api.slack.com/authentication/sign-in-with-slack#exchange
	tokenExchangeEndpoint := "https://slack.com/api/openid.connect.token"
	q := url.Values{
		"client_id":     {os.Getenv("SLACK_CLIENT_ID")},
		"client_secret": {os.Getenv("SLACK_CLIENT_SECRET")},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {"https://" + req.Host + "/auth/callback"},
	}
	exchange, err := http.NewRequest("POST", tokenExchangeEndpoint, strings.NewReader(q.Encode()))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
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
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("OAuth handshake failed: " + token.Error))
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

	t := jwt.New(jwt.GetSigningMethod(os.Getenv("JWT_SIGNING_METHOD")))
	t.Claims = &models.SessionUserClaims{
		StandardClaims: &jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 48).Unix(),
		},
		Info: *info,
	}

	tokenstr, err := t.SignedString([]byte(os.Getenv("JWT_SIGNING_KEY")))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Tokenize session: " + err.Error()))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "hub-identity-token",
		Value:   tokenstr,
		Path:    "/",
		Expires: time.Now().Add(time.Hour * 36),
	})
	http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
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
