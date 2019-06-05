package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"

	"encoding/hex"
	"github.com/gostones/goboot/web"
	"github.com/leonelquinteros/gorand"

	"bufio"
	"github.com/gorilla/mux"
	"github.com/gostones/goboot/util"
	"github.com/justinas/alice"
	"github.com/koding/websocketproxy"
	"github.com/lox/httpcache"
	"io"
	"path/filepath"
)

const internalAppDownload = "redis_app_download"

const cacheTime = time.Minute * 60 * 24 //1 day

var memCache = httpcache.NewMemoryCache()

// var cacheOptions = CacheOptions{MaxAge: cacheTime, NoTransform: true}

var wsTimeout = time.Minute * 5

var proxyClient = &http.Client{
	Timeout: time.Second * 10,
}

type ProxyServer struct {
	port int

	sport   string
	app     string
	account string
	home    string

	authKey string

	started   int
	connected bool
	sync.RWMutex
}

func (r *ProxyServer) isStarted() bool {
	r.RLock()
	defer r.RUnlock()

	return r.started > 0
}

func (r *ProxyServer) isConnected() bool {
	r.RLock()
	defer r.RUnlock()

	return r.connected
}

func (r *ProxyServer) setConnected() {
	r.Lock()
	defer r.Unlock()

	r.connected = true
}

func (r *ProxyServer) start() {
	r.Lock()
	defer r.Unlock()

	//only start once
	if r.started > 0 {
		return
	}

	r.started++
	//
	go r.spawn()

	//test
	uri := fmt.Sprintf("http://127.0.0.1:%v/", r.port)
	err := util.Retry(func() error {
		b, e := isServerReady(uri)

		log.Debugf("##### RProxy start isServerReady %v %v port: %v ready: %v error: %v", r.account, r.app, r.port, b, e)
		return e
	}, util.NewBackOff(12, 1*time.Second))

	log.Debugf("##### RProxy start started: %v %v %v port: %v  error: %v", r.started, r.account, r.app, r.port, err)
}

func (r *ProxyServer) stopped() {
	r.Lock()
	defer r.Unlock()
	r.started--
	r.connected = false
}

func GenerateKey(prefix string) (string, error) {
	uuid, err := gorand.UUIDv4()
	if err != nil {
		return "", err
	}
	now := web.CurrentTimestamp()

	key := fmt.Sprintf("%s_%s_%v", prefix, uuid, now)
	return hex.EncodeToString([]byte(key)), nil
}

func internalAuthKey() string {
	key, err := GenerateKey(internalAppDownload)
	if err != nil {
		panic(err)
	}
	return key
}

func (r *ProxyServer) spawn() {

	log.Debugf("##### RProxy spawn app: %s port: %v account: %s home: %v", r.app, r.port, r.account, r.home)

	redisCred := redisClient.Credentials()
	log.Debugf("##### RProxy spawn redis: %v", redisCred)

	base_uri := fmt.Sprintf("http://127.0.0.1:%v", r.sport)

	err := util.Retry(func() error {
		//localhost 127.0.0.1 0.0.0.0
		e := r.runRScript("shiny.R",
			fmt.Sprintf("SAP_HOME=%v", r.home),
			fmt.Sprintf("SAP_APP=%v", r.app),
			fmt.Sprintf("SAP_PORT=%v", r.port),
			fmt.Sprintf("BASE_URI=%v", base_uri),
			fmt.Sprintf("ACCOUNT=%v", r.account),
			fmt.Sprintf("AUTH_KEY=%v", r.authKey),
			fmt.Sprintf("REDIS_CRED=%v", redisCred),
		)

		log.Debugf("##### RProxy spawn app: %v account: %v  %v", r.app, r.account, e)

		return e
	})

	//
	r.stopped()

	log.Debugf("##### RProxy spawn started: %v failed or exited. app: %v account: %v error: %v", r.started, r.app, r.account, err)
}

func (r *ProxyServer) Handle(path string, router *mux.Router, mw alice.Chain) {
	//path = /app/shiny
	wsp := fmt.Sprintf("%s/%s/%s/websocket/", path, r.account, r.app)
	htp := fmt.Sprintf("%s/%s/%s/", path, r.account, r.app)

	log.Debugf("proxy ws path: %s app: %s port: %v account: %s", wsp, r.app, r.port, r.account)
	log.Debugf("proxy http path: %s  app: %s port: %v account: %s", htp, r.app, r.port, r.account)

	r.enableWsProxy(router, mw, wsp, fmt.Sprintf("ws://127.0.0.1:%v/websocket/", r.port))
	r.enableHttpProxy(router, mw, htp, fmt.Sprintf("http://127.0.0.1:%v/", r.port))
}

func (r *ProxyServer) enableWsProxy(router *mux.Router, mw alice.Chain, prefix string, remote string) {
	router.Handle(prefix, mw.Then(r.wsProxy(remote)))
}

func (r *ProxyServer) enableHttpProxy(router *mux.Router, mw alice.Chain, prefix string, remote string) {
	router.PathPrefix(prefix).Handler(mw.Then(http.StripPrefix(prefix, r.httpProxy(remote))))
}

func (r *ProxyServer) wsProxy(remoteUrl string) http.Handler {
	target := toUrl(remoteUrl)
	handler := websocketproxy.NewProxy(target)

	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if r.isStarted() && !r.isConnected() {
			r.setConnected()
		}
		log.Debugf("@@@ ##### wsProxy: %v started: %v connected: %v", req.URL, r.started, r.connected)

		handler.ServeHTTP(res, req)
	})
}

func (r *ProxyServer) httpProxy(remoteUrl string) http.Handler {
	handler := httputil.NewSingleHostReverseProxy(toUrl(remoteUrl))
	//ch := httpcache.NewHandler(memCache, handler)
	//ch.Shared = true

	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !r.isStarted() {
			r.start()
		}
		log.Debugf("@@@ ##### httpProxy: %s started: %v", req.URL, r.started)

		handler.ServeHTTP(res, req)

		//b := cacheOptions.isCacheable(req)
		//log.Debugf("@@@ CacheControl cacheable: %v %v", b, req.URL)
		//if b {
		//	cacheOptions.setCacheHeader(res)
		//	ch.ServeHTTP(res, req)
		//} else {
		//	handler.ServeHTTP(res, req)
		//}
	})
}

func NewProxyServer(sport string, home string, app string, account string, authKey string) *ProxyServer {
	s := ProxyServer{
		port:    util.FreePort(),
		sport:   sport,
		home:    home,
		app:     app,
		account: account,
		authKey: authKey,
		started: 0,
	}
	return &s
}

func toUrl(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func isServerReady(uri string) (bool, error) {
	res, err := proxyClient.Get(uri)

	if err != nil {
		log.Debugf("##### RProxy isServerReady: %v", err)
		return false, err
	}
	defer res.Body.Close()

	log.Debugf("##### RProxy isServerReady: %v", res)

	return (res.StatusCode == 200), nil
}

func (r *ProxyServer) runRScript(script string, env ...string) error {
	bin := os.Getenv("R_CMD")

	cmd := exec.Command(bin, "--vanilla", "-f", script)
	cmd.Env = append(os.Environ(), env...)

	outpipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Debugf("##### RProxy runRScript StdoutPipe %v", err)
		return err
	}
	stdout := bufio.NewScanner(outpipe)
	go func() {
		for stdout.Scan() {
			fmt.Printf("R> %s\n", stdout.Text())
		}
	}()

	errpipe, err := cmd.StderrPipe()
	if err != nil {
		log.Debugf("##### RProxy runRScript StderrPipe %v", err)
		return err
	}
	stderr := bufio.NewScanner(errpipe)
	go func() {
		for stderr.Scan() {
			fmt.Printf("STDERR R> %s\n", stderr.Text())
		}
	}()

	err = cmd.Start()
	if err != nil {
		log.Debugf("##### RProxy runRScript Start %v", err)
		return err
	}

	timer := time.NewTimer(wsTimeout)
	go func() {
		<-timer.C
		log.Debugf("##### RProxy runRScript Timer expired: %v", env)
		if !r.isConnected() {
			e := cmd.Process.Kill()
			log.Debugf("##### RProxy runRScript Timer expired: %v RShiny killed error: %v", env, e)
		}
	}()

	err = cmd.Wait()
	if err != nil {
		log.Debugf("##### RProxy runRScript Wait %v", err)
	}

	stop := timer.Stop()

	log.Debugf("##### RProxy runRScript RShiny exited due to error or inactivity, Timer stopped: %v", stop)
	return nil
}

func copyFile(path string, r io.Reader) error {
	w, err := os.Create(path)
	if err != nil {
		return err
	}
	defer w.Close()

	n, err := io.Copy(w, r)
	if err != nil {
		return err
	}
	log.Debugf("copyFile copied %v bytes to %v", n, path)

	return nil
}

func createAppHome() (string, error) {
	home, err := ioutil.TempDir("", "sap_home")
	return home, err
}

func unzipShinyApps(r io.Reader, sapHome string, app string) error {
	fn := filepath.FromSlash(fmt.Sprintf("%v/%v.zip", sapHome, app))
	// dest := filepath.FromSlash(fmt.Sprintf("%v/%v", sapHome, app))

	err := copyFile(fn, r)
	if err != nil {
		return err
	}

	//https://github.com/mholt/archiver
	//err = UnzipFile(fn, dest)
	if err != nil {
		return err
	}

	return nil
}
