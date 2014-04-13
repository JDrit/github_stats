package main

import (
    _ "github.com/jbarham/gopgsqldriver"
    "github.com/google/go-github/github"
    "code.google.com/p/goauth2/oauth"
    "fmt"
    "os/exec"
    "os"
    "sync"
    "database/sql"
    "strconv"
    "sync/atomic"
    "strings"
    "time"   
    "flag"
    "encoding/json"
)

var w sync.WaitGroup
var count int64 = 0

/**
 * Process the repos from the queue. Determines the languages in the repo 
 * and commits them to the database
 */
func cloner(id int, c chan *github.Repository, db *sql.DB) {
    dir := "/home/jd/tmp/" 
    languages := make(map[string]int)

    for repo := range c { 
        tx, err := db.Begin()
        if err != nil {
            fmt.Println("could not create transaction", err)
            break
        }
        stmt_repo, err := tx.Prepare("INSERT INTO repos (id, name, owner, description, " + 
            "language, createdat) VALUES ($1, $2, $3, $4, $5, $6)")
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
        description := ""
        if repo.Description != nil {
            description = *(repo.Description)
        }
        fmt.Fprintf(os.Stdout, "\t%d: %s (%s)\n", atomic.AddInt64(&count, 1), 
            *(repo.Name), *(repo.Owner.Login))
        cmd := exec.Command("git", "clone", *(repo.CloneURL), dir + *(repo.Name))
        cmd.Start()
        cmd.Wait()
        out, _ := exec.Command("cloc", "--by-file", "--yaml", dir + *(repo.Name)).Output()
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
                    fmt.Println(err)
                }
            }
        }
        var maxLanguage string
        maxCount := 0
        for key, value := range languages {
            if value > maxCount {
                maxLanguage = key
                maxCount = value
            }
        }
        _, err = tx.Stmt(stmt_repo).Exec(*(repo.ID), *(repo.Name), *(repo.Owner.Login), 
            description, maxLanguage, time.Now());
        if err != nil {
            fmt.Println(err)
        }
        tx.Commit()
        os.RemoveAll(dir + *(repo.Name))
    }
    w.Done() 
}

/**
 * Generates the list of repos to be processed and sends them to the given
 * queue 
 */
func get_repos(db *sql.DB, index int, c chan *github.Repository, apiToken string) { 
    t := &oauth.Transport{ Token: &oauth.Token{AccessToken: apiToken} }
    client := github.NewClient(t.Client())
    
    for {
        users, _, err := client.Users.ListAll(&github.UserListOptions{ Since: index }) 
        if err != nil { // deals with if the api limit is hit
            fmt.Println(err)
            time.Sleep(10 * time.Minute) 
        } else {
            for i := 0 ; i < len(users) ; i++ {
                fmt.Fprintf(os.Stdout, "%d: %s\n", (index + i), *(users[i].Login))
                repos, _, _ := client.Repositories.List(*(users[i].Login), nil)
                for j := 0 ; j < len(repos) ; j++ {
                    /* Makes sure that the repo has not already be processed */
                    var name string
                    db.QueryRow("SELECT name FROM repos WHERE owner = $1 and name = $2", 
                        *(users[i].Login), *(repos[j].Name)).Scan(&name)
                    if name == "" {
                        c <- &(repos[j])    
                    }
                }
            }
            index += len(users)
        }
    }
    close(c)
}

type Configuration struct {
    ApiToken string
    Driver   string
    Spec     string
}

func main() {
    offsetFlag := flag.Int("offset", 0, "The offset to use when looking up users")
    threadFlag := flag.Int("threads", 1, "The number of threads to download repos")
    configFlag := flag.String("config", "", "The config file to use for auth")
    helpFlag := flag.Bool("help", false, "Display help message")
    flag.Parse()

    if *helpFlag {
        fmt.Println("Downloads and parses github repoitories\n" + 
                    "--offset        the number of users to offset\n" +
                    "--threads       the number of goroutines to use\n" + 
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
     

    fmt.Fprintf(os.Stdout, "offset: %d\nnumber of threads: %d\n", 
        *offsetFlag, *threadFlag)
    db, err := sql.Open(configuration.Driver, configuration.Spec)
    if err != nil {
        fmt.Println("Could not connect to the database", err)
        return
    }
    db.SetMaxIdleConns(*threadFlag + 1)
    db.SetMaxOpenConns(*threadFlag + 1)
    
    c := make(chan *github.Repository, 300)
    w.Add(*threadFlag)
    
    go get_repos(db, *offsetFlag, c, configuration.ApiToken)

    for i := 0 ; i < *threadFlag ; i++ {
        go cloner(i, c, db)
    }
    
    w.Wait()
    db.Close()
}

