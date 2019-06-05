package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gostones/goboot/web"
	"github.com/gostones/goboot/web/gorilla"
	"github.com/justinas/alice"
	"io"
	"io/ioutil"
	_ "net/http/pprof"
	"os"
	"path"
	"testing"
)

type localFsDataAccess struct {
	accountId string
	dataDir   string
}

func (da localFsDataAccess) Ls(path string) *FileNode {
	var root FileNode
	root.Name = da.accountId

	root.Path = fmt.Sprintf("/home/%v/data/%v", da.accountId, da.accountId)

	d, _ := ioutil.ReadDir(da.dataDir)
	root.Children = make([]*FileNode, len(d))
	for i, v := range d {
		n := "/" + v.Name()
		root.Children[i] = &FileNode{Name: n, Path: root.Path + n}
	}

	return &root
}

func (da localFsDataAccess) Get(key string) (io.ReadCloser, error) {

	filename := path.Base(key)

	fp := path.Join(da.dataDir, filename)

	log.Debugf("!!! readFile: %v path: %v", key, fp)

	dat, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}

	return ioutil.NopCloser(bytes.NewReader(dat)), nil
}

func (da localFsDataAccess) GetDataFor(w io.Writer, schemaName, tableName string, accountCode string, startDt, endDt string, engines []string) error {
	return nil
}

func (da localFsDataAccess) isFiltering(schema, table, account string) bool {
	return false
}

func init() {
	log.Debugln("init ...")
}

func TestNewProxyServer(t *testing.T) {
	flag.Parse()
	if !*integration {
		t.Skip("skipping TestNewProxyServer")
	}

	//
	os.Setenv("R_CMD", "/usr/local/bin/R")

	//http://localhost:8080/app/shiny/test/dashboard/?account=test&app=dashboard

	appId := "dashboard"

	accountId := "test"

	var router = mux.NewRouter()
	var gs = gorilla.NewGorillaServer(router)

	cwd, _ := os.Getwd()

	log.Debugln("cwd : ", cwd)

	appHome := path.Join(cwd, "local")

	authKey := ""
	shinyGuard := alice.New()

	zip := "/tmp/" + appId + ".zip"

	dao := localFsDataAccess{
		accountId: accountId,
		dataDir:   "/tmp/data/test",
	}

	file, _ := os.Open(zip)

	unzipShinyApps(file, appHome, appId)

	ps := NewProxyServer(gs.Port(), appHome, appId, accountId, authKey)
	ps.Handle("/app/shiny", router, shinyGuard)

	router.HandleFunc("/internal/fs/ls", createHandleFsLs(dao)).Methods("GET")
	router.HandleFunc("/internal/fs/content", createHandleFsGet(dao))

	web.Run(gs)
}
