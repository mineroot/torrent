package listener

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"net"
	"sync"
)

var errAlreadyListened = fmt.Errorf("already listened")
var errNothingToClose = fmt.Errorf("nothing to close")

func New(logger *zerolog.Logger) *Listener {
	if logger == nil {
		nop := zerolog.Nop()
		logger = &nop
	}
	return &Listener{
		logger: logger,
		connCh: make(chan net.Conn),
		done:   make(chan struct{}),
	}
}

type Listener struct {
	once   sync.Once
	lis    net.Listener
	logger *zerolog.Logger
	connCh chan net.Conn
	done   chan struct{}
}

func (l *Listener) Listen(port uint16) (connCh <-chan net.Conn, err error) {
	err = errAlreadyListened
	l.once.Do(func() {
		connCh, err = l.listen(port)
	})
	return
}

func (l *Listener) listen(port uint16) (<-chan net.Conn, error) {
	var err error
	l.lis, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("unable to listen port %d: %w", port, err)
	}
	go func() {
		for {
			conn, err := l.lis.Accept()
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
	if l.lis == nil {
		return errNothingToClose
	}
	err := l.lis.Close()
	<-l.done
	close(l.connCh)
	l.lis = nil
	return err
}
