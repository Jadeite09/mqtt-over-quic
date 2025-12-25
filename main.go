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

var (
    topics = []string{"test/topic"} // 你要订阅的主题列表
)

func onConnect(client mqtt.Client) {
    fmt.Println("Subscribing...")
    for _, topic := range topics {
        if token := client.Subscribe(topic, 1, func(_ mqtt.Client, msg mqtt.Message) {
            fmt.Printf("Received message: [%s] %s\n", msg.Topic(),msg.Payload())
        }); token.Wait() && token.Error() != nil {
            fmt.Printf("Error subscribing to %s: %v", topic, token.Error())
        }
    }
}

func main() {
        // 配置MQTT客户端选项
        opts := mqtt.NewClientOptions()
        opts.AddBroker("quic://localhost:14567") // 替换为你的MQTT over QUIC服务器地址
        opts.SetClientID("go-client-quic")
        opts.SetCleanSession(false)
        opts.SetConnectRetry(true)
        opts.SetAutoReconnect(true)
        opts.SetOnConnectHandler(onConnect)
        opts.SetCustomOpenConnectionFn(openQUICConnection)

        // 创建客户端
        client := mqtt.NewClient(opts)

        // 连接MQTT服务器
        if token := client.Connect(); token.Wait() && token.Error() != nil {
                panic(token.Error())
        }
        fmt.Println("Connected to MQTT broker over QUIC")

        // 等待中断信号
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan

        // 断开连接
        client.Disconnect(250)
        fmt.Println("Disconnected")
}
