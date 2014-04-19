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

type Configuration struct {
    ApiToken    string
    Driver      string
    DbSpec      string
    AmqpSpec    string
    QueueName   string
    QueueName1  string
    Dir         string
}

var w sync.WaitGroup
var configuration Configuration

func processRepo(repo *github.Repository, db *sql.DB, id int) {
    stmt_repo, _ := db.Prepare("INSERT INTO repos (id, name, owner, description, " + 
        "forks, createdat) VALUES ($1, $2, $3, $4, $5, $6)")
    stmt_file, _ := db.Prepare("INSERT INTO files (name, blank, comment, " + 
            "code, language, repoid) VALUES ($1, $2, $3, $4, $5, $6)")
    stmt_repo_update, _ := db.Prepare("UPDATE repos set language = $1 where id = $2")

    folder := configuration.Dir + *(repo.Owner.Login) + "/" + *(repo.Name)
    languages := make(map[string]int)
    if  !regexp.MustCompile(`^[a-zA-Z0-9 ._-]*$`).MatchString(*(repo.Owner.Login)) || 
        !regexp.MustCompile(`^[a-zA-Z0-9 ._-]*$`).MatchString(*(repo.Name)) {
        fmt.Println("FAIL")
        return
    }
    var repoName string
    db.QueryRow("select name from repos where id = $1", *(repo.ID)).Scan(&repoName)
    if len(repoName) != 0 {
        fmt.Fprintf(os.Stdout, "repo (%s) %s has already been processed\n", *(repo.Owner.Login), repoName)
        return
    }
    tx, err := db.Begin()
    if err != nil {
        fmt.Println("Could not created tx: " + err.Error())
    }
    rows, _ := tx.Query("SELECT name from repos where id = $1", *(repo.ID))
    if rows.Next() {
        fmt.Fprintf(os.Stdout, "repo (%s) %s has already been processed\n", 
            *(repo.Owner.Login), *(repo.Name))
        return
    }
    fmt.Fprintf(os.Stdout, "repo (%s) %s processing\n", 
        *(repo.Owner.Login), *(repo.Name))

    description := ""
    if repo.Description != nil { description = *(repo.Description) }
    forks := 0
    if repo.ForksCount != nil { forks = *(repo.ForksCount) }
    _, err = tx.Stmt(stmt_repo).Exec(*(repo.ID), *(repo.Name), *(repo.Owner.Login), 
        description, forks, (*(repo.CreatedAt)).Unix())
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
            name := lines[i][len(configuration.Dir):len(lines[i]) - 1]
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

func processUser(user amqp.Delivery, db *sql.DB, dir, apiToken string, id int) {
    t := &oauth.Transport{ Token: &oauth.Token{AccessToken: apiToken} }
    client := github.NewClient(t.Client())

    fmt.Println("Processing user: " + string(user.Body))
    repos, _, err := client.Repositories.List(string(user.Body), nil)
    if err != nil {
        fmt.Println(err.Error())
    }
    for i := 0 ; i < len(repos) ; i += 1 {
        processRepo(&(repos[i]), db, id)
    }
    db.Exec("UPDATE users set lastprocessed = $1 where lower(login) = lower($2)", 
        time.Now().Unix(), string(user.Body))
    user.Ack(false)
    fmt.Println("Finished processing user: " + string(user.Body))
}

func consumer(id int, db *sql.DB, conn *amqp.Connection) {
    t := &oauth.Transport{ Token: &oauth.Token{AccessToken: configuration.ApiToken} }
    client := github.NewClient(t.Client())

    fmt.Println("starting processer")
    channel, _ := conn.Channel()
    channel.QueueDeclare("repos", false, false, false, false, nil)
    channel.QueueDeclare("repos-priority", false, false, false, false, nil)
    channel.Qos(1, 0, true)

    priRepos, _ := channel.Consume("repos-priority", "consumer-" + strconv.Itoa(rand.Int()),
        false, false, false, false, nil)
    regRepos, _ := channel.Consume("repos", "consumer-" + strconv.Itoa(rand.Int()),
        false, false, false, false, nil)
    for {
        var repo amqp.Delivery
        select {
        case repo = <- priRepos:
            info := strings.Split(string(repo.Body), "|")
            if len(info) != 2 { 
                repo.Ack(false)
                continue 
            }
            githubRepo, _, _ := client.Repositories.Get(info[0], info[1])            
            processRepo(githubRepo, db, id)
            repo.Ack(false)
        case repo = <- regRepos:
            info := strings.Split(string(repo.Body), "|")
            if len(info) != 2 { 
                repo.Ack(false)
                continue 
            }
            githubRepo, _, _ := client.Repositories.Get(info[0], info[1])            
            processRepo(githubRepo, db, id)
            repo.Ack(false)
        }
    }


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
    err = decoder.Decode(&configuration)
    if err != nil {
        fmt.Println(err.Error())
        return
    } 

    db, err := sql.Open(configuration.Driver, configuration.DbSpec)
    if err != nil {
        fmt.Println("Could not connect to the database", err)
        return
    }
    db.SetMaxIdleConns(*threadFlag + 1)
    db.SetMaxOpenConns(*threadFlag + 1)
    w.Add(*threadFlag)
    
    conn, err := amqp.Dial(configuration.AmqpSpec)
    if err != nil {
        fmt.Println("could not setp up rabbitmq connection", err)
        return
    }


    for i := 0 ; i < *threadFlag ; i++ {
        go consumer(i, db, conn)
    }
    
    w.Wait()
    db.Close()
}

