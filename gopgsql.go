package gopgsql

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	circuit "github.com/rubyist/circuitbreaker"
)

// Regenerate is the status of the circuit breaker to restart conections
var Regenerate string = "regenerate"

// Success is the status OK for the circuit breaker conection
var Success string = "success"

// Fail is the status of failure conection of the circuit breaker
var Fail string = "fail"

// PgOptions conection options structre
type PgOptions struct {
	Url        string
	Host       string
	User       string
	Pass       string
	Dbas       string
	Poolsize   int64
	FailRate   float64
	Universe   int64
	TimeOut    time.Duration
	Regenerate time.Duration
}

// PgPool pool structure
type PgPool struct {
	cb         *circuit.Breaker
	conn       chan *sql.DB
	state      string
	trippedAt  int64
	failCount  int64
	regenTryes int64
	Configs    PgOptions
}

// InitPool sets all connections to pool
func InitPool(opts PgOptions) (*PgPool, error) {
	pool := new(PgPool)
	pool.Configs = opts

	configValidate(&opts)
	pool.cb = circuit.NewRateBreaker(pool.Configs.FailRate, pool.Configs.Universe)
	pool.subscribe()

	err := generatePool(pool, false)
	if err != nil {
		pool.state = Fail
	}

	return pool, err
}

// Execute wrapper for manage failures in circuit breaker
func (pool *PgPool) Execute(callback func(*sql.DB) error) error {
	if pool.State() == Fail {
		pool.regenerate()
		return errors.New("unavailable service")
	}

	if pool.State() == Regenerate {
		return errors.New("unavailable service")
	}

	conn := pool.popConx()
	if conn == nil {
		pool.cb.Fail()
		return errors.New("empty connection")
	}

	var err error

	pool.cb.Call(func() error {
		err = callback(conn)
		return nil
	}, pool.Configs.TimeOut)
	pool.pushConx(conn)

	return err
}

// Subscribe events for control reset
func (pool *PgPool) subscribe() {
	events := pool.cb.Subscribe()

	go func() {
		for {
			e := <-events
			switch e {
			case circuit.BreakerTripped:
				pool.state = Fail
			case circuit.BreakerReset:
				pool.state = Regenerate
			case circuit.BreakerFail:
				log.Println(":::::: breaker fail ::::::")
			case circuit.BreakerReady:
				pool.state = Success
			}
		}
	}()
}

// PopConx return a conection of the pool
func (pool *PgPool) popConx() *sql.DB {
	return <-pool.conn
}

// PushConx restore a conection into the pool
func (pool *PgPool) pushConx(conx *sql.DB) {
	pool.conn <- conx
}

func configValidate(options *PgOptions) {
	var urlConnect string
	if options.Host != "" {
		urlConnect = "user=" + options.User + " dbname=" + options.Dbas + " host=" + options.Host + " password=" + options.Pass
	}
	if options.Url != "" && urlConnect == "" {
		urlConnect = options.Url
	}
	if urlConnect == "" {
		urlConnect = os.Getenv("DATABASE_URL")
	}
	options.Url = urlConnect

	if options.FailRate < 0.0 {
		options.FailRate = 0.0
	}
	if options.FailRate > 1.0 {
		options.FailRate = 1.0
	}
	if options.Poolsize <= 0 {
		options.Poolsize = 5
	}
	if options.Universe <= options.Poolsize {
		options.Universe = options.Poolsize
	}
	if options.TimeOut < 0 {
		options.TimeOut = 0
	}
	if options.Regenerate <= 0 {
		options.Regenerate = time.Second * 3
	}
}

func (pool *PgPool) connect() (*sql.DB, error) {
	var err error
	var db *sql.DB

	if pool.Configs.Url == "" {
		return nil, errors.New("can't find url")
	}
	if pool.cb.Tripped() {
		return nil, errors.New("unavailable service")
	}

	err = pool.cb.Call(func() error {
		log.Println(pool.Configs.Url)

		conn, err := sql.Open("postgres", pool.Configs.Url)
		if err != nil {
			return err
		}

		errp := conn.Ping()
		if errp != nil {
			return errp
		}

		db = conn
		return nil
	}, pool.Configs.TimeOut)

	return db, err
}

func generatePool(pool *PgPool, failFirst bool) error {
	pool.conn = make(chan *sql.DB, pool.Configs.Poolsize)

	for x := int64(0); x < pool.Configs.Poolsize; x++ {
		var conn *sql.DB
		var err error

		if conn, err = pool.connect(); err != nil {
			if failFirst {
				pool.setTrippedTime()
				return err
			} else {
				pool.failCount++
			}
		}
		pool.conn <- conn
	}

	if pool.cb.Tripped() {
		pool.setTrippedTime()
		return errors.New("failed to create connection pool")
	}

	pool.state = Success
	return nil
}

func (pool *PgPool) regenerate() {
	epoch := time.Now().Unix()
	diference := epoch - pool.trippedAt
	regentime := int64(pool.Configs.Regenerate / 1000000000)

	if diference >= regentime && pool.State() == Fail {
		pool.regenTryes++
		pool.reset()

		if err := generatePool(pool, true); err != nil {
			pool.cb.Trip()
			pool.setTrippedTime()
		} else {
			pool.regenTryes = 0
		}
	}
}

func (pool *PgPool) reset() {
	pool.clean()
	pool.cb.Reset()
	pool.failCount = 0
	pool.trippedAt = 0
}

func (pool *PgPool) clean() {
	if pool.regenTryes == 0 {
		for x := int64(0); x < pool.Configs.Poolsize; x++ {
			if aux := <-pool.conn; aux != nil {
				aux.Close()
			}
		}
	}
	close(pool.conn)
}

func (pool *PgPool) setTrippedTime() {
	if pool.trippedAt == 0 {
		trip := time.Now().Unix()
		pool.trippedAt = trip
	}
}

// GetUrl returns the connection url
func (pool *PgPool) GetUrl() string {
	return pool.Configs.Url
}

// State returns actual state of the pool
func (pool *PgPool) State() string {
	return pool.state
}
