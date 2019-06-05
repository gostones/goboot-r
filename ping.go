package main

import (
	"net/http"

	"github.com/gostones/goboot/web"
)

func pingHandler(rw http.ResponseWriter, req *http.Request) {
	type health struct {
		Status    string `json:"status"`
		Timestamp int64  `json:"timestamp"`
	}
	m := health{Status: "UP", Timestamp: web.CurrentTimestamp()}
	web.HandleJson(&m, rw, req)
}
