# gopgsql

## Install

```bash
go get -u -v github.com/danteay/gopgsql
```

And import in your files whit the next lines:

```go
import (
  "database/sql"
  "github.com/danteay/gopgsql"
)
```

## Configure

Setup config for circut and recover strategies

```go
conf := pgsqlcp.PgOptions{
  Url:        "postgres://localhost:5432/postgres",
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