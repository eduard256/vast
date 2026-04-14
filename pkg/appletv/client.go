package appletv

import (
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	mqtt  mqtt.Client
	topic string
}

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Topic    string
}

// Dial connects to MQTT broker and returns Apple TV client
func Dial(cfg Config) (*Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port)).
		SetUsername(cfg.User).
		SetPassword(cfg.Password).
		SetClientID("vast-appletv")

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &Client{mqtt: c, topic: cfg.Topic}, nil
}

// Play wakes Apple TV and starts streaming the given URL
func (c *Client) Play(url string) error {
	_ = c.send("turn_on", nil)
	time.Sleep(3 * time.Second)
	return c.send("play_url", map[string]string{"url": url})
}

func (c *Client) Stop() error {
	return c.send("stop", nil)
}

func (c *Client) Close() {
	c.mqtt.Disconnect(250)
}

// internals

func (c *Client) send(action string, extra map[string]string) error {
	msg := map[string]string{"action": action}
	for k, v := range extra {
		msg[k] = v
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	token := c.mqtt.Publish(c.topic+"/set", 1, false, b)
	token.Wait()
	return token.Error()
}
