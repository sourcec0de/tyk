package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"net/url"
  "time"
	"io/ioutil"

	"github.com/justinas/alice"
)

func createAuthKeyAuthSession() SessionState {
	var thisSession SessionState
	// essentially non-throttled
	thisSession.Rate = 100.0
	thisSession.Allowance = thisSession.Rate
	thisSession.LastCheck = time.Now().Unix()
	thisSession.Per = 1.0
	thisSession.Expires = 0
	thisSession.QuotaRenewalRate = 300 // 5 minutes
	thisSession.QuotaRenews = time.Now().Unix()
	thisSession.QuotaRemaining = 10
	thisSession.QuotaMax = 10

	thisSession.AccessRights = map[string]AccessDefinition{"31": AccessDefinition{APIName: "Tyk Auth Key Test", APIID: "31", Versions: []string{"default"}}}

	return thisSession
}

func getAuthKeyChain(spec APISpec) http.Handler {
	remote, _ := url.Parse(spec.Proxy.TargetURL)
	proxy := TykNewSingleHostReverseProxy(remote, &spec)
	proxyHandler := http.HandlerFunc(ProxyHandler(proxy, &spec))
	tykMiddleware := &TykMiddleware{&spec, proxy}
	chain := alice.New(
		CreateMiddleware(&IPWhiteListMiddleware{tykMiddleware}, tykMiddleware),
		CreateMiddleware(&AuthKey{tykMiddleware}, tykMiddleware),
		CreateMiddleware(&VersionCheck{TykMiddleware: tykMiddleware}, tykMiddleware),
		CreateMiddleware(&KeyExpired{tykMiddleware}, tykMiddleware),
		CreateMiddleware(&AccessRightsCheck{tykMiddleware}, tykMiddleware),
		CreateMiddleware(&RateLimitAndQuotaCheck{tykMiddleware}, tykMiddleware)).Then(proxyHandler)

	return chain
}


func TestBearerTokenAuthKeySession(t *testing.T) {
  spec := createDefinitionFromString(authKeyDef)
	redisStore := RedisClusterStorageManager{KeyPrefix: "apikey-"}
	healthStore := &RedisClusterStorageManager{KeyPrefix: "apihealth."}
	orgStore := &RedisClusterStorageManager{KeyPrefix: "orgKey."}
	spec.Init(&redisStore, &redisStore, healthStore, orgStore)
  thisSession := createAuthKeyAuthSession()
	customToken := "54321111"
  // AuthKey sessions are stored by {token}
  spec.SessionManager.UpdateSession(customToken, thisSession, 60)

  recorder := httptest.NewRecorder()
  req, err := http.NewRequest("GET", "/auth_key_test/", nil)

  if err != nil {
    log.Error("Problem creating new request object.", err)
  }

  req.Header.Add("authorization", "Bearer " + customToken)

  chain := getAuthKeyChain(spec)
  chain.ServeHTTP(recorder, req)

  if recorder.Code != 200 {
    t.Error("Initial request failed with non-200 code, should have gone through!: \n", recorder.Code)
		t.Error(ioutil.ReadAll(recorder.Body))
  }
}

var authKeyDef string = `
  {
		"name": "Tyk Auth Key Test",
		"api_id": "31",
		"org_id": "default",
    "use_keyless": false,
		"definition": {
			"location": "header",
			"key": "version"
		},
		"auth": {
			"auth_header_name": "authorization"
		},
		"version_data": {
			"not_versioned": true,
			"versions": {
				"Default": {
					"name": "Default",
					"use_extended_paths": true,
					"expires": "3000-01-02 15:04",
					"paths": {
						"ignored": [],
						"white_list": [],
						"black_list": []
					}
				}
			}
		},
		"proxy": {
			"listen_path": "/auth_key_test/",
			"target_url": "http://example.com/",
			"strip_listen_path": true
		}
	}`
