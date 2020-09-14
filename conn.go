package k8s_exec_pod

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog"
)

var upGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 1024 * 10,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Proxy interface {
	ReadPump()
	WritePump()
	Close()
	Recv() (*message, error)
	Send(messageType int, data []byte) error
}

func NewProxy(ctx context.Context, w http.ResponseWriter, r *http.Request) (Proxy, error) {
	conn, err := upGrader.Upgrade(w, r, nil)
	if err != nil {
		klog.V(2).Info(err)
		return nil, err
	}
	p := &proxy{
		conn:      conn,
		status:    proxyAlive,
		readChan:  make(chan *message, 4096),
		writeChan: make(chan *message, 4096),
		ctx:       ctx,
	}
	go p.ReadPump()
	go p.WritePump()
	return p, nil
}

type proxyStatus int

const (
	proxyAlive proxyStatus = iota
	proxyClose proxyStatus = 1
)

type proxy struct {
	conn      *websocket.Conn
	status    proxyStatus
	readChan  chan *message
	writeChan chan *message
	closeOnce sync.Once
	ctx       context.Context
}

type message struct {
	messageType int
	data        []byte
}

func (p *proxy) ReadPump() {
	defer p.Close()
	for {
		messageType, data, err := p.conn.ReadMessage()
		klog.Info("data:", data)
		klog.Infof("messageType: %d message: %v err: %s\n", messageType, data, err)
		if err != nil {
			klog.V(2).Info(err)
			return
		}
		msg := &message{messageType: messageType, data: data}
		select {
		case p.readChan <- msg:
		case <-p.ctx.Done():
			klog.Info("ReadPump: proxy ctx cancel")
			return
		case <-time.After(time.Second * 5):
			klog.Info("ReadPump: write into readChan timeout 5s")
			return
		}
	}
}

func (p *proxy) WritePump() {
	defer p.Close()
	for {
		select {
		case msg, isClose := <-p.writeChan:
			if !isClose {
				return
			}
			klog.Info("proxy WritePump msg-data:", string(msg.data))
			if err := p.conn.WriteMessage(msg.messageType, msg.data); err != nil {
				klog.V(2).Info(err)
				return
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *proxy) Close() {
	klog.Info("proxy close")
	p.closeOnce.Do(func() {
		if p.status == proxyClose {
			return
		}
		p.status = proxyClose
		close(p.readChan)
		close(p.writeChan)
		if err := p.conn.Close(); err != nil {
			klog.V(2).Info(err)
		}
	})
}

func (p *proxy) Recv() (*message, error) {
	klog.Info("proxy Recv message")
	select {
	case msg, isClose := <-p.readChan:
		if !isClose {
			return nil, fmt.Errorf("readChan closed")
		}
		klog.Info("proxy recv-msg:", msg)
		klog.Info("proxy recv-data-string:", string(msg.data))
		return msg, nil
	case <-p.ctx.Done():
		return nil, fmt.Errorf("proxy ctx cancel")
	}
}

func (p *proxy) Send(messageType int, data []byte) error {
	klog.Infof("proxy send messageType:%v data:%v", messageType, string(data))
	select {
	case p.writeChan <- &message{messageType: messageType, data: data}:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("proxy ctx cancel")
	}
}
