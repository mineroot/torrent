package listener

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"testing"
	"time"
)

const port = 12345

func TestListener_Listen(t *testing.T) {
	const numCons = 100
	const msg = "hello"
	listener := New(nil)
	connCh, err := listener.Listen(port)
	require.NoError(t, err)
	go func() {
		for i := 0; i < numCons; i++ {
			conn, err := net.Dial("tcp", fmt.Sprintf(":%d", port))
			require.NoError(t, err)
			_, err = conn.Write([]byte(msg))
			require.NoError(t, err)
			err = conn.Close()
			assert.NoError(t, err)
		}
		err := listener.Close()
		assert.NoError(t, err)
	}()
	for conn := range connCh {
		err = conn.SetReadDeadline(time.Now().Add(time.Millisecond))
		require.NoError(t, err)
		data, err := io.ReadAll(conn)
		assert.Equal(t, msg, string(data))
		require.NoError(t, err)
	}
	_, err = listener.Listen(port)
	assert.ErrorIs(t, err, errAlreadyListened)
}

func TestListener_Close(t *testing.T) {
	listener := New(nil)
	_, err := listener.Listen(port)
	require.NoError(t, err)

	err = listener.Close()
	require.NoError(t, err)
	err = listener.Close()
	require.ErrorIs(t, err, errNothingToClose)
}
