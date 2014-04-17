package main

import (
    _ "github.com/jbarham/gopgsqldriver"
    "github.com/google/go-github/github"
    "code.google.com/p/goauth2/oauth"
    "github.com/streadway/amqp"
    "fmt"
    "os"
    "regexp"
    "math/rand"
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

func processUser(login, dir string, client *github.Client, db *sql.DB, id int) {
    stmt_repo, _ := db.Prepare("INSERT INTO repos (id, name, owner, description, " + 
        "forks, createdat) VALUES ($1, $2, $3, $4, $5, $6)")
    stmt_file, _ := db.Prepare("INSERT INTO files (name, blank, comment, " + 
            "code, language, repoid) VALUES ($1, $2, $3, $4, $5, $6)")
    stmt_repo_update, _ := db.Prepare("UPDATE repos set language = $1 where id = $2")

    repos, _, err := client.Repositories.List(login, nil)
    fmt.Println("processing user: " + login)
    if err != nil {
        fmt.Println("Error getting repos from Github: " + err.Error())
        return
    }

    for i := 0 ; i < len(repos) ; i += 1 {
        repo := &(repos[i])
        folder := dir + *(repo.Owner.Login) + "/" + *(repo.Name)
        languages := make(map[string]int)
        if  !regexp.MustCompile(`^[a-zA-Z0-9 ._-]*$`).MatchString(*(repo.Owner.Login)) || 
            !regexp.MustCompile(`^[a-zA-Z0-9 ._-]*$`).MatchString(*(repo.Name)) {
            fmt.Println("FAIL")
            continue
        }
        
        fmt.Println(*(repo.Name))
        /*var repoName string
        db.QueryRow("select name from repos where id = $1", *(repo.ID)).Scan(&repoName)
        if len(repoName) != 0 {
            fmt.Fprintf(os.Stdout, "%d repo (%s) %s has already been processed\n", id, *(repo.Owner.Login), repoName)
            continue
        }*/
        tx, err := db.Begin()
        if err != nil {
            fmt.Println("Could not created tx: " + err.Error())
        }
        rows, _ := tx.Query("SELECT name from repos where id = $1", *(repo.ID))
        if rows.Next() {
            fmt.Fprintf(os.Stdout, "%d repo (%s) %s has already been processed\n", id, 
                *(repo.Owner.Login), *(repo.Name))
            continue
        }
        fmt.Fprintf(os.Stdout, "%d repo (%s) %s processing\n", id, 
            *(repo.Owner.Login), *(repo.Name))

        description := ""
        if repo.Description != nil {
            description = *(repo.Description)
        }
        _, err = tx.Stmt(stmt_repo).Exec(*(repo.ID), *(repo.Name), *(repo.Owner.Login), 
            description, *(repo.ForksCount), (*(repo.CreatedAt)).Unix())
        if err != nil {
            fmt.Fprintf(os.Stdout, "repo error: %s\n", err.Error())
        }
        cmd := exec.Command("git", "clone", *(repo.CloneURL), folder)
        cmd.Start()
        cmd.Wait()
        out, _ := exec.Command("cloc", "--by-file", "--yaml",  folder).Output()
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
                    fmt.Fprintf(os.Stdout, "%d file error: %s\n", id, err.Error())
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
        _, err = tx.Stmt(stmt_repo_update).Exec(maxLanguage, *(repo.ID))
        if err != nil {
            fmt.Fprintf(os.Stdout, "%d repo update error %s\n", id, err.Error())
        }

        os.RemoveAll(folder)
        fmt.Fprintf(os.Stdout, "\t%d Processed repo: (%s) %s\n", id, *(repo.Owner.Login), *(repo.Name))
        tx.Commit()
    }
}

func consumer(id int, db *sql.DB, apiToken string, conn *amqp.Connection, queueName, dir string) {
    t := &oauth.Transport{ Token: &oauth.Token{AccessToken: apiToken} }
    client := github.NewClient(t.Client())

    channel, _ := conn.Channel()
    channel.QueueDeclare("users", false, false, false, false, nil)
    channel.QueueDeclare("users-priority", false, false, false, false, nil)
    channel.Qos(1, 0, false) 

    fmt.Println("starting consumer " + queueName + "-" + strconv.Itoa(id))
    users, err := channel.Consume(queueName, "consumer-" + strconv.Itoa(rand.Int()),
        false, false, false, false, nil)
    if err != nil {
        fmt.Println("Error getting consumer: " + err.Error())
        return
    }
    for user := range users {
        fmt.Println(queueName + ": " + string(user.Body))
        processUser(string(user.Body), dir, client, db, id)
        
        _, err = db.Exec("UPDATE users set lastprocessed = $1 where login = $2", 
            time.Now().Unix(), string(user.Body))
        if err != nil {
            fmt.Fprintf(os.Stdout, "user error: %s\n", err.Error())
            os.Exit(1)
        }
        fmt.Println(queueName + ": finished processing user: " + string(user.Body))
        user.Ack(false)
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
    db.SetMaxIdleConns(*threadFlag * 2 + 1)
    db.SetMaxOpenConns(*threadFlag * 2 + 1)
    w.Add(*threadFlag * 2)
    
    conn, err := amqp.Dial(configuration.AmqpSpec)
    if err != nil {
        fmt.Println("could not setp up rabbitmq connection", err)
        return
    }
    for i := 0 ; i < *threadFlag ; i++ {
        go consumer(i, db, configuration.ApiToken, conn, 
           configuration.QueueName, configuration.Dir)
        go consumer(i, db, configuration.ApiToken, conn, 
           "users-priority", configuration.Dir)

    }
    
    w.Wait()
    db.Close()
}

