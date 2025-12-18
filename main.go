package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/quic-go/quic-go"

)

// quicStreamConn 包装QUIC Stream实现net.Conn接口
type quicStreamConn struct {
	quic.Stream
	sess quic.Connection
}

func (c *quicStreamConn) LocalAddr() net.Addr {
	return c.sess.LocalAddr()
}

func (c *quicStreamConn) RemoteAddr() net.Addr {
	return c.sess.RemoteAddr()
}

func (c *quicStreamConn) SetDeadline(t time.Time) error {
	return c.Stream.SetDeadline(t)
}

func (c *quicStreamConn) SetReadDeadline(t time.Time) error {
	return c.Stream.SetReadDeadline(t)
}

func (c *quicStreamConn) SetWriteDeadline(t time.Time) error {
	return c.Stream.SetWriteDeadline(t)
}

// openQUICConnection 自定义QUIC连接建立函数
func openQUICConnection(broker *url.URL, _ mqtt.ClientOptions) (net.Conn, error) {
	// 配置TLS（根据服务器证书配置）
	tlsConf := &tls.Config{
		InsecureSkipVerify: true, // 测试环境跳过证书验证
		NextProtos:         []string{"mqtt"},
	}

	// 建立QUIC连接
	sess, err := quic.DialAddr(context.Background(), broker.Host, tlsConf, nil)
	if err != nil {
		return nil, fmt.Errorf("QUIC dial error: %w", err)
	}

	// 创建QUIC流
	stream, err := sess.OpenStreamSync(context.Background())
	if err != nil {
		return nil, fmt.Errorf("open stream error: %w", err)
	}

	return &quicStreamConn{Stream: stream, sess: sess}, nil
}

func main() {
	// 配置MQTT客户端选项
	opts := mqtt.NewClientOptions()
	opts.AddBroker("quic://localhost:14567") // 替换为你的MQTT over QUIC服务器地址
	opts.SetClientID("go-quic-client")
	opts.SetAutoReconnect(true)
	opts.SetCustomOpenConnectionFn(openQUICConnection)

	// 创建客户端
	client := mqtt.NewClient(opts)

	// 连接MQTT服务器
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Println("Connected to MQTT broker over QUIC")

	// 订阅主题
	topic := "test/topic"
	qos := 1
	if token := client.Subscribe(topic, byte(qos), func(_ mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Received message: [%s] %s\n", msg.Topic(), msg.Payload())
	}); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Printf("Subscribed to topic: %s\n", topic)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 断开连接
	client.Disconnect(250)
	fmt.Println("Disconnected")
}

