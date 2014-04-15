package background

import (
    "github.com/streadway/amqp"
    "github.com/revel/revel"
    "time"
)

type ProcessUser struct {
    Login      string
}

func (request ProcessUser) Run() {
    if token, found := revel.Config.String("amqp_url"); !found {
        revel.ERROR.Printf("No amqp_url given")
    } else {
        conn, err := amqp.Dial(token) 
        if err != nil {
            revel.ERROR.Printf("error opening connection to rabbitmq\n%s", err.Error())
            return
        }
        c, _ := conn.Channel()
         msg := amqp.Publishing{                                                                                                             
            DeliveryMode: amqp.Persistent,                                                                                                   
            Timestamp:    time.Now(),                                                                                                        
            ContentType:  "text/plain",                                                                                                      
            Body:         []byte(request.Login),
        }
        err = c.Publish("", "users-priority", false, false, msg)
        if err != nil {
            revel.ERROR.Printf("error publishing user\n%s", err.Error())
            return
        }
        c.Close()
        revel.INFO.Printf("Added user %s to queue\n", request.Login)
    }
}
