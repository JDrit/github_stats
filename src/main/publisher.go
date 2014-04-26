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
        fmt.Println(err)
        return
    }
    c, _ := conn.Channel()
    _, err = c.QueueDeclare("repos", false, false, false, false, nil)
    _, err = c.QueueDeclare("repos-priority", false, false, false, false, nil)
    if err != nil {
        fmt.Println(err.Error())
        return
    }

    for {
        users, _, err := client.Users.ListAll(&github.UserListOptions{ Since: index }) 
        if err != nil {
            fmt.Println(err)
            time.Sleep(5 * time.Minute)
        }
        for i := 0 ; i < len(users) ; i++ {
            user, _, err := client.Users.Get(*(users[i].Login))
            if err != nil {
                fmt.Println(err.Error())
                os.Exit(1)
            }
            stmt_user, _ := db.Prepare("INSERT INTO users (id, name, login, email, " + 
                "avatarUrl, followers, following, createdat, reposleft) VALUES " + 
                "($1, $2, $3, $4, $5, $6, $7, $8, $9)")
            name := ""
            email := ""
            if user.Name != nil { name = *(user.Name) }
            if user.Email != nil { email = *(user.Email) }
            page := 0
            var totalRepos []github.Repository
            for ; ; {
                options := github.RepositoryListOptions {
                    ListOptions: github.ListOptions {
                        Page: page,
                        PerPage: 100,
                    },
                }
                repos, _, _ := client.Repositories.List(*(user.Login), &options)
                for i := 0 ; i < len(repos) ; i++ {
                    totalRepos = append(totalRepos, repos[i])
                }
                page++
                if len(repos) != 100 {
                    break
                }
            }
            _, err = stmt_user.Exec(*(user.ID), name, 
                *(user.Login), email, *(user.AvatarURL), 
                *(user.Followers), *(user.Following), (*(user.CreatedAt)).Unix(), 
                len(totalRepos))
            for j := 0 ; j < len(totalRepos) ; j++ {
                repo := (totalRepos[j])
                message := *(repo.Owner.Login) + "|" + *(repo.Name)
                msg := amqp.Publishing{
                    DeliveryMode: amqp.Persistent,
                    Timestamp:    time.Now(),
                    ContentType:  "text/plain",
                    Body:         []byte(message),
                }
                var name string
                db.QueryRow("select name from repos where name = $1 and owner = $2", 
                    *(repo.Name), *(repo.Owner.Login)).Scan(&name)

                if len(name) == 0 {
                    fmt.Fprintf(os.Stdout, "%d: (%s) %s added to queue\n",
                        (index + i), *(repo.Owner.Login), *(repo.Name))
                    err = c.Publish("", "repos", false, false, msg)
                    if err != nil {
                        fmt.Println(err)
                    }
                } else {
                    fmt.Fprintf(os.Stdout, "repo (%s) %s has already ben processed\n", 
                        *(repo.Owner.Login), *(repo.Name))
                }
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
    Dir         string
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

