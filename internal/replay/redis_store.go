package replay

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

var errRedisNil = errors.New("redis: nil")

type RedisRepository struct {
	client *redisClient
}

func NewRedisRepository(addr, password string, db int, timeout time.Duration) *RedisRepository {
	return &RedisRepository{
		client: newRedisClient(addr, password, db, timeout),
	}
}

func (r *RedisRepository) Get(ctx context.Context, key string) (StoredResponse, bool, error) {
	payload, err := r.client.Get(ctx, key)
	if err != nil {
		if errors.Is(err, errRedisNil) {
			return StoredResponse{}, false, nil
		}
		return StoredResponse{}, false, err
	}
	response, err := decodeStoredResponse(payload)
	if err != nil {
		return StoredResponse{}, false, err
	}
	return response, true, nil
}

func (r *RedisRepository) Set(ctx context.Context, key string, value StoredResponse, overwrite bool) error {
	payload, err := encodeStoredResponse(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, payload, overwrite)
}

func (r *RedisRepository) Close() error {
	return r.client.Close()
}

type redisClient struct {
	addr     string
	password string
	db       int
	timeout  time.Duration

	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
}

func newRedisClient(addr, password string, db int, timeout time.Duration) *redisClient {
	return &redisClient{
		addr:     addr,
		password: password,
		db:       db,
		timeout:  timeout,
	}
}

func (c *redisClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.reader = nil
	return err
}

func (c *redisClient) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConn(ctx); err != nil {
		return nil, err
	}

	if err := c.writeCommand(ctx, "GET", key); err != nil {
		c.reset()
		return nil, err
	}

	reply, err := c.readReply(ctx)
	if err != nil {
		c.reset()
		return nil, err
	}

	switch reply.kind {
	case replyBulk:
		return reply.data, nil
	case replyNil:
		return nil, errRedisNil
	case replyError:
		return nil, fmt.Errorf("redis error: %s", reply.text)
	default:
		return nil, fmt.Errorf("unexpected redis reply: %v", reply.kind)
	}
}

func (c *redisClient) Set(ctx context.Context, key string, value []byte, overwrite bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConn(ctx); err != nil {
		return err
	}

	args := []string{"SET", key, string(value)}
	if !overwrite {
		args = append(args, "NX")
	}
	if err := c.writeCommand(ctx, args...); err != nil {
		c.reset()
		return err
	}

	reply, err := c.readReply(ctx)
	if err != nil {
		c.reset()
		return err
	}
	if reply.kind == replyNil {
		return nil
	}
	if reply.kind == replyError {
		return fmt.Errorf("redis error: %s", reply.text)
	}
	if reply.kind != replySimple {
		return fmt.Errorf("unexpected redis reply: %v", reply.kind)
	}
	return nil
}

func (c *redisClient) ensureConn(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}
	return c.connect(ctx)
}

func (c *redisClient) connect(ctx context.Context) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return err
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)

	if c.password != "" {
		if err := c.simpleCommand(ctx, "AUTH", c.password); err != nil {
			c.reset()
			return err
		}
	}
	if c.db != 0 {
		if err := c.simpleCommand(ctx, "SELECT", strconv.Itoa(c.db)); err != nil {
			c.reset()
			return err
		}
	}
	return nil
}

func (c *redisClient) simpleCommand(ctx context.Context, args ...string) error {
	if err := c.writeCommand(ctx, args...); err != nil {
		return err
	}
	reply, err := c.readReply(ctx)
	if err != nil {
		return err
	}
	if reply.kind == replyError {
		return fmt.Errorf("redis error: %s", reply.text)
	}
	if reply.kind != replySimple {
		return fmt.Errorf("unexpected redis reply: %v", reply.kind)
	}
	return nil
}

func (c *redisClient) reset() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.reader = nil
}

func (c *redisClient) writeCommand(ctx context.Context, args ...string) error {
	if c.conn == nil {
		return errors.New("redis connection is not established")
	}
	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		deadline = ctxDeadline
	}
	_ = c.conn.SetDeadline(deadline)
	_, err := c.conn.Write(buildRESPCommand(args...))
	return err
}

type replyKind int

const (
	replyUnknown replyKind = iota
	replySimple
	replyError
	replyInt
	replyBulk
	replyNil
)

type redisReply struct {
	kind replyKind
	text string
	data []byte
}

func (c *redisClient) readReply(ctx context.Context) (redisReply, error) {
	if c.reader == nil {
		return redisReply{kind: replyUnknown}, errors.New("redis reader is not initialized")
	}
	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		deadline = ctxDeadline
	}
	_ = c.conn.SetDeadline(deadline)

	line, err := c.reader.ReadString('\n')
	if err != nil {
		return redisReply{kind: replyUnknown}, err
	}
	line = strings.TrimSuffix(line, "\r\n")
	if len(line) == 0 {
		return redisReply{kind: replyUnknown}, errors.New("empty redis reply")
	}

	prefix := line[0]
	payload := line[1:]

	switch prefix {
	case '+':
		return redisReply{kind: replySimple, text: payload}, nil
	case '-':
		return redisReply{kind: replyError, text: payload}, nil
	case ':':
		return redisReply{kind: replyInt, text: payload}, nil
	case '$':
		size, err := strconv.Atoi(payload)
		if err != nil {
			return redisReply{kind: replyUnknown}, err
		}
		if size == -1 {
			return redisReply{kind: replyNil}, nil
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return redisReply{kind: replyUnknown}, err
		}
		return redisReply{kind: replyBulk, data: buf[:size]}, nil
	default:
		return redisReply{kind: replyUnknown}, fmt.Errorf("unknown redis reply prefix: %q", prefix)
	}
}

func buildRESPCommand(args ...string) []byte {
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&buffer, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return buffer.Bytes()
}
