package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	relaygo "crosspointapp.com/relay-go"
	"github.com/gobwas/ws"
)

var addr = flag.String("addr", ":8080", "http service address")

func main() {
	flag.Parse()

	http.HandleFunc("/testworkflow", func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer conn.Close()

		stopC := make(chan struct{})
		sendC := make(chan []byte, 5)
		recvC := make(chan []byte, 5)

		go func() {
			relay, err := relaygo.NewRelayDevice(sendC, recvC, stopC)
			if err != nil {
				log.Fatal("Failed to initialize relay", err)
				return
			}

			relay.Vibrate()
			deviceName, _ := relay.GetName()
			relay.Say("What is your name?")
			user, _ := relay.Listen([]string{})

			response := fmt.Sprintf("Hello %s! Your device name is %s", user, deviceName)
			relay.Say(response)
			relay.Terminate()

		}()
		handleWS(conn, recvC, sendC, stopC)
	})

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
