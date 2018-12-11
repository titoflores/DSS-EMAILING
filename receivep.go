package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/streadway/amqp"
)

type User struct {
	ID    string
	Name  string
	Email string
}

type EmailDao struct {
	From    User
	To      []User
	Content string
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"rpc_Mail", // name
		true,       // durable
		false,      // delete when unused
		false,      // exclusive
		false,      // no-wait
		nil,        // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		for d := range msgs {
			d.Ack(true)
			myEmail := FromGOB(d.Body)
			var sendit string
			if SendMail(myEmail) {
				sendit = "OK"
			} else {
				sendit = "ERROR"
			}
			fmt.Println("processing petition: ", d.CorrelationId)
			err = ch.Publish(
				"",        // exchange
				d.ReplyTo, // routing key
				false,     // mandatory
				false,     // immediate
				amqp.Publishing{
					ContentType:   "text/plain",
					CorrelationId: d.CorrelationId,
					Body:          []byte(sendit),
				})
			failOnError(err, "Failed to publish a message")

		}
	}()
	forever := make(chan bool)
	<-forever
}

func FromGOB(by []byte) EmailDao {
	m := EmailDao{}
	b := bytes.Buffer{}
	b.Write(by)
	d := gob.NewDecoder(&b)
	err := d.Decode(&m)
	if err != nil {
		fmt.Println(`failed gob Decode`, err)
	}
	return m
}

func SendMail(data EmailDao) bool {

	sendit := true

	f, err := os.OpenFile("./mail_log.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
		sendit = false
	}

	defer f.Close()

	if _, err = f.WriteString("\n"); err != nil {
		panic(err)
		sendit = false
	}

	if _, err = f.WriteString(parseMail(data)); err != nil {
		panic(err)
		sendit = false
	}

	return sendit

}

func parseMail(data EmailDao) string {
	currentTime := time.Now()
	currentTime.Format("2006-01-02 15:04:05")
	var mail bytes.Buffer
	mail.WriteString(currentTime.Format("2006-01-02 15:04:05"))
	mail.WriteString("> Enviando correo a: ")
	for _, v := range data.To {
		mail.WriteString(v.Email)
		mail.WriteString(", ")
	}
	mail.WriteString(" Contenido del correo: ")
	mail.WriteString(data.Content)
	//fmt.Println(mail.String())
	return mail.String()
}
