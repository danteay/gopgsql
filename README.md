# pgsqlcp

## Install

Complete the [prerequisites](https://github.com/compropago/gomodules/blob/master/README.md) before install the module.
After, you have to execute the next command:

```bash
go get -u -v github.com/compropago/gomodules/pgsqlcp
```

And import in your files whit the next lines:

```go
import (
  "database/sql"
  "github.com/compropago/gomodules/pgsqlcp"
)
```

## Configure

Setup config for circut and recover strategies

```go
conf := pgsqlcp.PgOptions{
  Url:        "postgres://devusername:devpassword@dev-postgres.cjuxp0rkz2bd.us-west-2.rds.amazonaws.com:5432/msreceipts",
  Poolsize:   10,
  FailRate:   0.25,
  Regenerate: time.Second * 5,
  TimeOut:    time.Second * 1,
}
```

Init connection pool

```go
pool, err := pgsqlcp.InitPool(conf)

if err != nil {
  log.Println(err)
}
```

Execute querys inside of the circuit breaker

```go
var suma int

errQuery := pool.Execute(func(db *sql.DB) error {
  log.Println("Entra callback")
  return db.QueryRow("SELECT 1+1 AS suma").Scan(&suma)
})

if errQuery != nil {
  log.Println(errQuery)
}
```

Helt check of the pool connection

```go
log.Println("==>> State: ", pool.State())
```