package rabbitmq_client

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

type RabbitClient struct {
	sendConn *amqp.Connection
	recConn  *amqp.Connection
	sendChan *amqp.Channel
	recChan  *amqp.Channel

	Creds RabbitClientCreds
}

type RabbitClientCreds struct {
	Username string
	Password string
	Host     string
	Port     string
}

func (rcl *RabbitClient) connect(isRec, reconnect bool) (*amqp.Connection, error) {
	if reconnect {
		if isRec {
			rcl.recConn = nil
		} else {
			rcl.sendConn = nil
		}
	}
	if isRec && rcl.recConn != nil {
		return rcl.recConn, nil
	} else if !isRec && rcl.sendConn != nil {
		return rcl.sendConn, nil
	}
	var c string
	if rcl.Creds.Username == "" {
		c = fmt.Sprintf("amqp://%s:%s/", rcl.Creds.Host, rcl.Creds.Port)
	} else {
		c = fmt.Sprintf("amqp://%s:%s@%s:%s/", rcl.Creds.Username, rcl.Creds.Password, rcl.Creds.Host, rcl.Creds.Port)
	}
	conn, err := amqp.Dial(c)
	if err != nil {
		log.Printf("\r\n--- could not create a conection ---\r\n")
		time.Sleep(1 * time.Second)
		return nil, err
	}
	if isRec {
		rcl.recConn = conn
		return rcl.recConn, nil
	} else {
		rcl.sendConn = conn
		return rcl.sendConn, nil
	}
}

func (rcl *RabbitClient) channel(isRec, recreate bool) (*amqp.Channel, error) {
	if recreate {
		if isRec {
			rcl.recChan = nil
		} else {
			rcl.sendChan = nil
		}
	}
	if isRec && rcl.recConn == nil {
		rcl.recChan = nil
	}
	if !isRec && rcl.sendConn == nil {
		rcl.recChan = nil
	}
	if isRec && rcl.recChan != nil {
		return rcl.recChan, nil
	} else if !isRec && rcl.sendChan != nil {
		return rcl.sendChan, nil
	}
	for {
		_, err := rcl.connect(isRec, recreate)
		if err == nil {
			break
		}
	}
	var err error
	if isRec {
		rcl.recChan, err = rcl.recConn.Channel()
	} else {
		rcl.sendChan, err = rcl.sendConn.Channel()
	}
	if err != nil {
		log.Println("--- could not create channel ---")
		time.Sleep(1 * time.Second)
		return nil, err
	}
	if isRec {
		return rcl.recChan, err
	} else {
		return rcl.sendChan, err
	}
}

func (rcl *RabbitClient) Consume(n string, f func([]byte) error) {
	for {
		for {
			_, err := rcl.channel(true, true)
			if err == nil {
				break
			}
		}
		log.Printf("--- connected to consume '%s' ---\r\n", n)
		q, err := rcl.recChan.QueueDeclare(
			n,
			true,
			false,
			false,
			false,
			amqp.Table{"x-queue-mode": "lazy"},
		)
		if err != nil {
			log.Println("--- failed to declare a queue, trying to reconnect ---")
			continue
		}
		connClose := rcl.recConn.NotifyClose(make(chan *amqp.Error))
		connBlocked := rcl.recConn.NotifyBlocked(make(chan amqp.Blocking))
		chClose := rcl.recChan.NotifyClose(make(chan *amqp.Error))
		m, err := rcl.recChan.Consume(
			q.Name,
			uuid.New().String(),
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.Println("--- failed to consume from queue, trying again ---")
			continue
		}
		shouldBreak := false
		for {
			if shouldBreak {
				break
			}
			select {
			case _ = <-connBlocked:
				log.Println("--- connection blocked ---")
				shouldBreak = true
				break
			case err = <-connClose:
				log.Println("--- connection closed ---")
				shouldBreak = true
				break
			case err = <-chClose:
				log.Println("--- channel closed ---")
				shouldBreak = true
				break
			case d := <-m:

				if err = f(d.Body); err != nil {
					_ = d.Ack(true)
					break
				}
				_ = d.Ack(false)
			}
		}
	}
}

func (rcl *RabbitClient) Publish(n string, b []byte) {
	r := false
	for {
		for {
			_, err := rcl.channel(false, r)
			if err == nil {
				break
			}
		}
		q, err := rcl.sendChan.QueueDeclare(
			n,
			true,
			false,
			false,
			false,
			amqp.Table{"x-queue-mode": "lazy"},
		)
		if err != nil {
			log.Println("--- failed to declare a queue, trying to resend ---")
			r = true
			continue
		}
		err = rcl.sendChan.Publish(
			"",
			q.Name,
			false,
			false,
			amqp.Publishing{
				MessageId:    uuid.New().String(),
				DeliveryMode: amqp.Persistent,
				ContentType:  "text/plain",
				Body:         b,
			})
		if err != nil {
			log.Println("--- failed to publish to queue, trying to resend ---")
			r = true
			continue
		}
		break
	}
}

func NewRMQ(c RabbitClientCreds) *RabbitClient {
	if c.Host == "" || c.Port == "" {
		return nil
	}
	return &RabbitClient{
		Creds: c,
	}
}

func NewRMQCreds(username, password, host, port string) RabbitClientCreds {
	return RabbitClientCreds{
		Username: strings.TrimSpace(username),
		Password: strings.TrimSpace(password),
		Host:     strings.TrimSpace(host),
		Port:     strings.TrimSpace(port),
	}
}
