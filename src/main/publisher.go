package main

import (
    _ "github.com/jbarham/gopgsqldriver"
    "github.com/google/go-github/github"
    "code.google.com/p/goauth2/oauth"
    "github.com/streadway/amqp"
    "fmt"
    "os"
    "database/sql"
    "flag"
    "time"
    "encoding/json"
)

/**
 * Generates the list of repos to be processed and sends them to the given
 * queue 
 */
func publisher(db *sql.DB, index int, apiToken, spec, queueName string, queueSize int) { 
    t := &oauth.Transport{ Token: &oauth.Token{AccessToken: apiToken} }
    client := github.NewClient(t.Client())
    conn, err := amqp.Dial(spec)
    if err != nil {
        fmt.Fprintf(os.Stderr, "error opening rabbitmq connection\n")
        return
    }
    c, _ := conn.Channel()
    queue, err := c.QueueDeclare(queueName, false, false, false, false, nil)
    if err != nil {
        fmt.Println(err.Error())
    }

    for {
        users, _, _ := client.Users.ListAll(&github.UserListOptions{ Since: index }) 
        for i := 0 ; i < len(users) ; i++ {
             msg := amqp.Publishing{
                DeliveryMode: amqp.Persistent,
                Timestamp:    time.Now(),
                ContentType:  "text/plain",
                Body:         []byte(*(users[i].Login)),
            }
            var userName string
            db.QueryRow("select login from users where id = $1", *(users[i].ID)).Scan(&userName)

            if len(userName) == 0 {
                tx, _ := db.Begin()
                stmt_user, _ := tx.Prepare("INSERT INTO users (id, name, login, email, " + 
                    "avatarUrl, followers, following, createdat) VALUES " + 
                    "($1, $2, $3, $4, $5, $6, $7, $8)")
                user, _, _ := client.Users.Get(*(users[i].Login))
                name := ""
                if user.Name != nil {
                    name = *(user.Name)
                }
                email := ""
                if user.Email != nil {
                    email = *(user.Email)
                }
                _, err := tx.Stmt(stmt_user).Exec(*(user.ID), name, 
                    *(user.Login), email, *(user.AvatarURL), 
                    *(user.Followers), *(user.Following), *(user.CreatedAt))
                if err != nil {
                    fmt.Fprintf(os.Stdout, "user sql error: %s\n", err.Error())
                }
                tx.Commit()
                queue, _ = c.QueueInspect(queueName)
                for ; queue.Messages > queueSize ;  {
                    queue, _ = c.QueueInspect(queueName)
                    fmt.Println("queue full, sleeping...")
                    time.Sleep(5 * time.Second)
                }
                fmt.Fprintf(os.Stdout, "%d: %s added to queue\n", (index + i), *(user.Login))
                c.Publish("", queueName, false, false, msg)
            } else {
                fmt.Fprintf(os.Stdout, "user %s is has already ben processed\n", userName)
            }
        }
        index += len(users)
    }
}

type Configuration struct {
    ApiToken    string
    Driver      string
    DbSpec      string
    AmqpSpec    string
    QueueName   string
    QueueSize   int
}

func main() {
    offsetFlag := flag.Int("offset", 0, "The offset to use when looking up users")
    configFlag := flag.String("config", "", "The config file to use for auth")
    helpFlag := flag.Bool("help", false, "Display help message")
    flag.Parse()

    if *helpFlag {
        fmt.Println("Downloads and parses github repoitories\n" + 
                    "--offset        the number of users to offset\n" +
                    "--threads       the number of goroutines to use\n" +
                    "--config        the json encoded config file to use\n" +
                    "--help          show this message")
        return
    }
    
    if *configFlag == "" {
        fmt.Fprintln(os.Stderr, "No config file specified. Use --config")
        return
    }

    file, err := os.Open(*configFlag)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error openening config file\n", err)
        return
    }
    decoder := json.NewDecoder(file)
    configuration := Configuration{}
    decoder.Decode(&configuration)
     
    db, err := sql.Open(configuration.Driver, configuration.DbSpec)
    if err != nil {
        fmt.Println("Could not connect to the database", err)
        return
    }
    
    publisher(db, *offsetFlag, configuration.ApiToken, configuration.AmqpSpec,
        configuration.QueueName, configuration.QueueSize)

    db.Close()
}

