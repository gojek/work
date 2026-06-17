package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/gojek/work/webui"
	"github.com/gomodule/redigo/redis"
)

var (
	redisHostPort  = flag.String("redis", "redis://:6379", "redis hostport")
	redisDatabase  = flag.String("database", "0", "redis database")
	redisNamespace = flag.String("ns", "work", "redis namespace")
	webHostPort    = flag.String("listen", ":5040", "hostport to listen for HTTP JSON API")
	authUser       = flag.String("auth-user", envOr("WORK_WEBUI_AUTH_USER", "admin"), "username for HTTP Basic Auth (or WORK_WEBUI_AUTH_USER env)")
	authPassword   = flag.String("auth-password", os.Getenv("WORK_WEBUI_AUTH_PASSWORD"), "password for HTTP Basic Auth over the whole UI; empty disables auth (or WORK_WEBUI_AUTH_PASSWORD env)")
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	flag.Parse()

	fmt.Println("Starting workwebui:")
	fmt.Println("redis = ", *redisHostPort)
	fmt.Println("database = ", *redisDatabase)
	fmt.Println("namespace = ", *redisNamespace)
	fmt.Println("listen = ", *webHostPort)
	if *authPassword == "" {
		fmt.Println("auth = disabled (set -auth-password or WORK_WEBUI_AUTH_PASSWORD to require Basic Auth)")
	} else {
		fmt.Printf("auth = enabled (user=%q)\n", *authUser)
	}

	database, err := strconv.Atoi(*redisDatabase)
	if err != nil {
		fmt.Printf("Error: %v is not a valid database value", *redisDatabase)
		return
	}

	pool := newPool(*redisHostPort, database)

	var opts []webui.HandlerOption
	if *authPassword != "" {
		opts = append(opts, webui.WithBasicAuth(*authUser, *authPassword))
	}

	server := webui.NewServer(*redisNamespace, pool, *webHostPort, opts...)
	server.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	<-c

	server.Stop()

	fmt.Println("\nQuitting...")
}

func newPool(addr string, database int) *redis.Pool {
	return &redis.Pool{
		MaxActive:   3,
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.DialURL(addr, redis.DialDatabase(database))
		},
		Wait: true,
	}
}
