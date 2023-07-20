package listener

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net"
)

func NewListenerFromContext(ctx context.Context) *Listener {
	return &Listener{
		logger: log.Ctx(ctx),
		connCh: make(chan net.Conn),
		done:   make(chan struct{}),
	}
}

type Listener struct {
	net.Listener
	logger *zerolog.Logger
	connCh chan net.Conn
	done   chan struct{}
}

func (l *Listener) Listen(port uint16) (<-chan net.Conn, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("unable to listen port %d: %w", port, err)
	}
	l.Listener = listener
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					close(l.done)
					return
				}
				continue
			}
			if conn.RemoteAddr().Network() != "tcp" {
				conn.Close()
				continue
			}
			l.logger.Info().
				Str("remote_peer", conn.RemoteAddr().String()).
				Msg("incoming connection from remote peer")
			l.connCh <- conn
		}
	}()

	return l.connCh, nil
}

func (l *Listener) Close() error {
	err := l.Listener.Close()
	<-l.done
	close(l.connCh)
	return err
}
