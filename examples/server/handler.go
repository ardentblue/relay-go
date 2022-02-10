package main

import (
	"net"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func handleWS(conn net.Conn, recvC, sendC chan []byte, stopC <-chan struct{}) error {
	var wg sync.WaitGroup
	wg.Add(2)
	errC := make(chan error, 2)

	go func() {
		defer wg.Done()
		readWS(recvC, stopC, errC, conn)
	}()

	go func() {
		defer wg.Done()
		sendWS(sendC, stopC, errC, conn)
	}()

	go func() {
		wg.Wait()
		close(errC)
	}()

	err := <-errC
	return err
}

func readWS(recvC chan []byte, stopC <-chan struct{}, errC chan<- error, conn net.Conn) {
	for {
		select {
		case <-stopC:
			return
		default:
			msg, _, err := wsutil.ReadClientData(conn)
			if err != nil {
				if err.Error() == "EOF" {
					errC <- nil
					return
				}
				errC <- err
				return
			}

			recvC <- msg
		}
	}
}

func sendWS(sendC chan []byte, stopC <-chan struct{}, errC chan<- error, conn net.Conn) {
	for {
		select {
		case <-stopC:
			return
		case msg := <-sendC:
			err := wsutil.WriteServerMessage(conn, ws.OpText, msg)
			if err != nil {
				errC <- err
				return
			}
		}
	}
}
