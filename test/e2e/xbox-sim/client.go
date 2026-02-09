package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/xbslink/xbslink-ng/internal/logging"
	"github.com/xbslink/xbslink-ng/internal/protocol"
	"github.com/xbslink/xbslink-ng/internal/transport"
	"github.com/xbslink/xbslink-ng/test/testutil"
)

func runClient() {
	fs := flag.NewFlagSet("client", flag.ExitOnError)

	address := fs.String("address", "", "Server address (host:port)")
	key := fs.String("key", "", "Encryption key (empty for insecure)")
	logLevel := fs.String("log", "info", "Log level (error, warn, info, debug, trace)")
	sendFrames := fs.Bool("send-frames", true, "Send simulated Xbox frames")
	frameInterval := fs.Duration("frame-interval", 50*time.Millisecond, "Interval between sent frames")
	latencyBase := fs.Duration("latency-base", 0, "Base simulated latency for PONG replies")
	latencyJitter := fs.Duration("latency-jitter", 0, "Jitter range (±) for simulated latency")
	latencyStep := fs.Duration("latency-step", 5*time.Millisecond, "Step size for interactive +/- adjustment")

	fs.Parse(os.Args[2:])

	if *address == "" {
		fmt.Fprintln(os.Stderr, "Error: --address is required")
		os.Exit(1)
	}

	level, err := logging.ParseLevel(*logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid log level: %v\n", err)
		os.Exit(1)
	}
	logger := logging.NewLogger(level)

	var keyBytes []byte
	if *key != "" {
		keyBytes = []byte(*key)
	}
	codec := protocol.NewCodec(keyBytes)

	latCfg := NewLatencyConfig(*latencyBase, *latencyJitter, *latencyStep)

	trans, err := transport.New(transport.Config{
		Mode:     transport.ModeConnect,
		PeerAddr: *address,
		Codec:    codec,
		Logger:   logger,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating transport: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			logger.Info("received signal, shutting down")
			cancel()
		case <-ctx.Done():
		}
	}()

	logger.Info("connecting to %s ...", *address)
	if err := trans.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: handshake failed: %v\n", err)
		os.Exit(1)
	}
	logger.Info("connected (handshake complete)")
	logger.Info("latency config: %s", latCfg)

	// Start recv loop.
	go clientRecvLoop(ctx, trans, codec, latCfg, logger, cancel)

	// Start frame sender if requested.
	if *sendFrames {
		go clientFrameSender(ctx, trans, codec, *frameInterval, logger)
	}

	// Start interactive key reader if attached to a TTY.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		go clientKeyReader(ctx, latCfg, logger, cancel)
		logger.Info("interactive mode: +/= increase latency, - decrease, q quit")
	}

	<-ctx.Done()

	logger.Info("shutting down")
	_ = trans.SendBye()
	_ = trans.Close()
}

func clientRecvLoop(ctx context.Context, trans *transport.Transport, codec *protocol.Codec, latCfg *LatencyConfig, logger *logging.Logger, cancel context.CancelFunc) {
	buf := make([]byte, 65536)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = trans.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, _, err := trans.Recv(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			logger.Debug("recv error: %v", err)
			continue
		}

		msg, err := codec.Decode(buf[:n])
		if err != nil {
			logger.Debug("decode error: %v", err)
			continue
		}

		switch msg.Type {
		case protocol.MsgPing:
			ts := msg.Timestamp
			delay := latCfg.Delay()
			logger.Debug("PING ts=%d, replying with delay %s", ts, delay)
			go func() {
				if delay > 0 {
					time.Sleep(delay)
				}
				pong := codec.EncodePong(ts)
				if err := trans.Send(pong); err != nil {
					logger.Debug("send PONG error: %v", err)
				}
			}()

		case protocol.MsgBye:
			logger.Info("received BYE from server")
			cancel()
			return

		case protocol.MsgFrame:
			logger.Trace("received frame (%d bytes)", len(msg.Frame))

		case protocol.MsgPong:
			logger.Trace("received PONG ts=%d", msg.Timestamp)

		default:
			logger.Debug("received unknown message type: 0x%02X", msg.Type)
		}
	}
}

func clientFrameSender(ctx context.Context, trans *transport.Transport, codec *protocol.Codec, interval time.Duration, logger *logging.Logger) {
	srcMAC := testutil.RandomXboxMAC()
	dstMAC := testutil.BroadcastMAC()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("sending frames every %s (src=%s)", interval, srcMAC)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			frame := testutil.RandomEthernetFrame(srcMAC, dstMAC, testutil.EtherTypeIPv4, 64)
			encoded, err := codec.EncodeFrame(frame)
			if err != nil {
				logger.Debug("encode frame error: %v", err)
				continue
			}
			if err := trans.Send(encoded); err != nil {
				logger.Debug("send frame error: %v", err)
			}
		}
	}
}

func clientKeyReader(ctx context.Context, latCfg *LatencyConfig, logger *logging.Logger, cancel context.CancelFunc) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		logger.Warn("failed to set raw terminal mode: %v", err)
		return
	}
	defer func() {
		_ = term.Restore(fd, oldState)
	}()

	keyCh := make(chan byte, 1)
	go func() {
		b := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(b)
			if n > 0 {
				keyCh <- b[0]
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case k := <-keyCh:
			switch k {
			case '+', '=':
				newBase := latCfg.IncreaseBase()
				fmt.Fprintf(os.Stderr, "\r\nlatency base: %s\r\n", newBase)
			case '-':
				newBase := latCfg.DecreaseBase()
				fmt.Fprintf(os.Stderr, "\r\nlatency base: %s\r\n", newBase)
			case 'q', 3: // 'q' or Ctrl-C
				fmt.Fprintf(os.Stderr, "\r\nquitting...\r\n")
				cancel()
				return
			}
		}
	}
}
