package xwatermill

import "github.com/ThreeDotsLabs/watermill/message"

func NewWatermillMessage(topic string, msg *message.Message) *Message {
	return &Message{
		Msg:   msg,
		Topic: topic,
	}
}

type Message struct {
	Msg   *message.Message
	Topic string
}
