package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/povilasv/prommod"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	lptt "github.com/sampleref/gortpdemo/ptt"
	lweb "github.com/sampleref/gortpdemo/web"
	lwebrtc "github.com/sampleref/gortpdemo/webrtc"
)

func init() {
	// Generate pem file for https
	lweb.GenPem()
	lptt.Initialize()
	fmt.Println("All good")
}

func main() {
	if err := prometheus.Register(prommod.NewCollector("sfu_ws")); err != nil {
		panic(err)
	}

	port := flag.String("p", "8443", "https port")
	lweb.HtmlFile = flag.String("html", "", "html file absolute path")
	lweb.PttHtml = flag.String("ptthtml", "", "html file absolute path")
	flag.Parse()

	http.Handle("/metrics", promhttp.Handler())

	// Websocket handle func
	http.HandleFunc("/ws", lwebrtc.WebsocketHandler)
	http.HandleFunc("/pttws", lptt.WsConn)

	// Html handle func
	http.HandleFunc("/", lweb.Web)
	http.HandleFunc("/ptt", lweb.WebPtt)

	// Init other state
	log.SetFlags(0)
	lwebrtc.TrackLocals = map[string]*webrtc.TrackLocalStaticRTP{}

	// request a keyframe every 3 seconds
	go func() {
		for range time.NewTicker(time.Second * 3).C {
			lwebrtc.DispatchKeyFrame()
		}
	}()

	// Support https, so we can test by lan
	fmt.Println("Web listening :" + *port)
	panic(http.ListenAndServeTLS(":"+*port, "cert.pem", "key.pem", nil))
}
