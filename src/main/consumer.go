package main

import (
    _ "github.com/jbarham/gopgsqldriver"
    "github.com/google/go-github/github"
    "code.google.com/p/goauth2/oauth"
    "github.com/streadway/amqp"
    "fmt"
    "os"
    "sync"
    "os/exec"
    "strings"
    "database/sql"
    "strconv"
    "flag"
    "time"
    "encoding/json"
)

var w sync.WaitGroup

func consumer(id int, db *sql.DB, apiToken, amqpSpec, queueName, dir string) {
    t := &oauth.Transport{ Token: &oauth.Token{AccessToken: apiToken} }
    client := github.NewClient(t.Client())
    conn, err := amqp.Dial(amqpSpec)
    if err != nil {
        fmt.Fprintf(os.Stderr, "error opening rabbitmq connection\n")
        fmt.Println(err.Error())
        return
    }
    c, _ := conn.Channel()
    _, err = c.QueueDeclarePassive("users", false, false, false, false, nil)
    if err != nil {
        fmt.Println(err)
        return
    }
    users, err := c.Consume(queueName, "consumer-" + strconv.Itoa(id), false, false, false, false, nil)
    if err != nil {
        fmt.Println(err.Error())
        return
    }
    for user := range users {
        repos, _, err := client.Repositories.List(string(user.Body), nil)
        fmt.Println("proccessing user: " + string(user.Body))
        if err != nil {
            fmt.Println(err)
            time.Sleep(5 * time.Minute)
            for {
                repos, _, err = client.Repositories.List(string(user.Body), nil)
                if err == nil {
                    break
                }
            }
        }
        for i := 0 ; i < len(repos) ; i += 1 {
            repo := &(repos[i])
            languages := make(map[string]int)
            tx, err := db.Begin()
            if err != nil {
                fmt.Println("could not create transaction", err)
                break
            }
            stmt_repo, err := tx.Prepare("INSERT INTO repos (id, name, owner, description, " + 
                "language, forks) VALUES ($1, $2, $3, $4, $5, $6)")
            if err != nil {
                fmt.Println(err)
                break
            }
            stmt_file, err := tx.Prepare("INSERT INTO files (name, blank, comment, " + 
                "code, language, repoid) VALUES ($1, $2, $3, $4, $5, $6)")
            if err != nil {
                fmt.Println(err)
                break
            }
            cmd := exec.Command("git", "clone", *(repo.CloneURL), dir + strconv.Itoa(*(repo.ID)))
            cmd.Start()
            cmd.Wait()
            out, _ := exec.Command("cloc", "--by-file", "--yaml", dir + strconv.Itoa(*(repo.ID))).Output()
            header := true
            lines := strings.Split(string(out), "\n")
            for i := 0 ; i < len(lines) ; i ++ {
                if header && strings.HasPrefix(strings.Trim(lines[i], " "), "lines_per_second") {
                    header = false
                } else if strings.Trim(lines[i], " ") == "SUM:" {
                    break
                } else if !header {
                    name := lines[i][len(dir):len(lines[i]) - 1]
                    i += 1
                    blank, _ := strconv.Atoi(strings.Trim(strings.Split(lines[i], ":")[1], " "))
                    i += 1
                    comment, _ := strconv.Atoi(strings.Trim(strings.Split(lines[i], ":")[1], " "))
                    i += 1
                    code, _ := strconv.Atoi(strings.Trim(strings.Split(lines[i], ":")[1], " "))
                    i += 1
                    language := strings.Trim(strings.Split(lines[i], ":")[1], " ")
                    languages[language] = languages[language] + blank + comment + code
                    _, err = tx.Stmt(stmt_file).Exec(name, blank, comment, code, 
                        language, *(repo.ID)) 
                    if err != nil {
                        fmt.Fprintf(os.Stdout, "file error: %s\n", err.Error())
                    }
                }
            }
            maxLanguage := ""
            maxCount := 0
            for key, value := range languages {
                if value > maxCount {
                    maxLanguage = key
                    maxCount = value
                }
            }
            description := ""
            if repo.Description != nil {
                description = *(repo.Description)
            }
            _, err = tx.Stmt(stmt_repo).Exec(*(repo.ID), *(repo.Name), *(repo.Owner.Login), 
                description, maxLanguage, *(repo.ForksCount));
            if err != nil {
                fmt.Fprintf(os.Stdout, "repo error: %s\n", err.Error())
            }
            tx.Commit()
            os.RemoveAll(dir + *(repo.Name))
            fmt.Fprintf(os.Stdout, "\tProcessed repo: (%s) %s\n", *(repo.Owner.Login), *(repo.Name))
            user.Ack(false)
        }
        fmt.Println("finished processing user: ", string(user.Body))
    }
    fmt.Println("done")
    w.Done()
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
    threadFlag := flag.Int("threads", 1, "The number of threads to download repos")
    configFlag := flag.String("config", "", "The config file to use for auth")
    helpFlag := flag.Bool("help", false, "Display help message")
    flag.Parse()

    if *helpFlag {
        fmt.Println("Downloads and parses github repoitories\n" + 
                    "--threads       the number of goroutines to use\n" + 
                    "--config        the config file to use\n" + 
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
    db.SetMaxIdleConns(*threadFlag + 1)
    db.SetMaxOpenConns(*threadFlag + 1)
    w.Add(*threadFlag)

    for i := 0 ; i < *threadFlag ; i++ {
        go consumer(i, db, configuration.ApiToken, configuration.AmqpSpec, 
            configuration.QueueName, configuration.Dir)
    }
    
    w.Wait()
    db.Close()
}

