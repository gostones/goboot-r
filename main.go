package main

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gostones/goboot/cf/redis"
	"github.com/gostones/goboot/config"
	"github.com/gostones/goboot/logging"
	"github.com/gostones/goboot/web"
	"github.com/gostones/goboot/web/gorilla"
	"github.com/justinas/alice"
)

var settings = config.AppSettings()
var log = logging.Logger()
var redisClient = redis.NewRedisClient()

func authHandler(authKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Debugf("internalAuthHandler begin: %v", r.URL)

			key := r.URL.Query().Get("key")
			if key == "" || key != authKey {
				http.Error(w, "internalAuthHandler", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)

			log.Debugf("internalAuthHandler end: %v", r.URL)
		})
	}
}

func main() {
	startServer()
}

func startServer() {

	var router = mux.NewRouter()
	var gs = gorilla.NewGorillaServer(router)

	//ping
	router.HandleFunc("/ping", pingHandler)

	//internal proxy access
	authKey := internalAuthKey()
	guard := alice.New(authHandler(authKey))
	router.Handle("/internal/fs/ls", guard.ThenFunc(HandleFsLsInternal())).Methods("GET")
	router.Handle("/internal/fs/content", guard.ThenFunc(HandleFsGetInternal()))

	//proxy
	appHome, err := createAppHome()
	if err != nil {
		panic(err)
	}

	appId := "dashboard"
	accountId := "test"
	shinyGuard := alice.New()

	ps := NewProxyServer(gs.Port(), appHome, appId, accountId, authKey)
	ps.Handle("/app/shiny", router, shinyGuard)

	web.Run(gs)
}
