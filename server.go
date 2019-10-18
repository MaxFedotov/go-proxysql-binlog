package main

import (
	"context"
	"errors"
	"net"

	log "github.com/siddontang/go-log/log"
)

type Server struct {
	Addr    string
	Handler Handler
	ctx     context.Context
}

type Handler interface {
	ServeTCP(conn net.Conn) error
}

func ListenAndServeTCP(addr string, handler Handler, ctx context.Context) error {
	server := &Server{Addr: addr, Handler: handler, ctx: ctx}
	return server.ListenAndServe()
}

func (srv Server) ListenAndServe() error {
	log.Infof("starting TCP server on %v", srv.Addr)
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	go func() {
		<-srv.ctx.Done()
		log.Info("stopping TCP server")
		listener.Close()
		wg.Done()
	}()
	go func() {
		wg.Add(1)
		errClosing := errors.New("use of closed network connection")
		for {
			conn, err := listener.Accept()
			if err != nil {
				if oe, ok := err.(*net.OpError); ok && oe.Err.Error() == errClosing.Error() {
					log.Debugf("stopping TCP listener on %v", srv.Addr)
					return
				}
				log.Errorf("error accepting connection %v", err)
				clientErrors.Inc()
				continue
			}
			clientsConnected.Inc()
			log.Infof("accepted connection from %v", conn.RemoteAddr())
			go srv.Handler.ServeTCP(conn)
		}
	}()
	return err
}
