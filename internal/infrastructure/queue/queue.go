package queue

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

func New(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err = ch.QueueDeclare("transactions.dlq", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	args := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": "transactions.dlq",
	}
	if _, err = ch.QueueDeclare("transactions", true, false, false, false, args); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	return &Client{Conn: conn, Channel: ch}, nil
}

func (c *Client) Close() {
	_ = c.Channel.Close()
	_ = c.Conn.Close()
}

func (c *Client) Publish(ctx context.Context, queue string, body []byte) error {
	return c.Channel.PublishWithContext(ctx, "", queue, false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		})
}
