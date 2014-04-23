package background

import (
    "github.com/revel/revel/cache"
    "github.com/revel/revel"
    "github.com/coopernurse/gorp"
    "github.com/revel/revel/modules/db/app"
    "github_stats/app/models"
    "time"
)

type ProcessSpeed struct {  }

/**
 * Background job to get the number of lines processed
 * per second
 */
func (s ProcessSpeed) Run() {
    revel.INFO.Printf("processing speed\n")
    dbm := &gorp.DbMap{Db: db.Db, 
        Dialect: gorp.PostgresDialect{}}
    txn, err := dbm.Begin()
    if err != nil {
        panic(err)
    }
    beginTime := time.Now().Unix()
    fileStats, _ := txn.Select(models.FileStat{}, "select sum(code + comment + blank) as sum from files")
    beginLines := fileStats[0].(*models.FileStat).Sum
    time.Sleep(time.Minute * 2)
    fileStats, _ = txn.Select(models.FileStat{}, "select sum(code + comment + blank) as sum from files")
    endTime := time.Now().Unix()
    endLines := fileStats[0].(*models.FileStat).Sum
    speed := float64(endLines - beginLines) / float64(endTime - beginTime)
    cache.Set("speed", int(speed), time.Hour)
    revel.INFO.Printf("%f\n", speed)
}

