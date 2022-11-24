package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	// "syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/mux"

	// "github.com/go-redis/redis"
	"github.com/gomodule/redigo/redis"

	// "github.com/bitly/go-simplejson"
	"github.com/bitly/go-simplejson"
)

// spaHandler implements the http.Handler interface, so we can use it
// to respond to HTTP requests. The path to the static directory and
// path to the index file within that static directory are used to
// serve the SPA in the given static directory.
type spaHandler struct {
	staticPath string
	indexPath  string
}

// Define stuct of authentication Middleware token map
type authenticationMiddleware struct {
	tokenUsers map[string]string
}

// initializing it
func (amw *authenticationMiddleware) Populate() {
	amw.tokenUsers["0000"] = "user0"
	amw.tokenUsers["0001"] = "user1"
	amw.tokenUsers["0002"] = "user2"
	amw.tokenUsers["0003"] = "user3"
}

// middleware function which will be called for each request
func (amw *authenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Session-Token")
		if user, found := amw.tokenUsers[token]; found {
			// We found the token in out map
			log.Printf("Authenticted user %s\n", user)
			// pass down the request to next middleware(or finel handler)
			next.ServeHTTP(w, r)
		} else {
			// Write to output to client
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	})
}

// ServeHTTP inspects the URL path to locate a file within the static dir
// on the SPA handler. If a file is found, it will be served. If not, the
// file located at the index path on the SPA handler will be served. This
// is suitable behavior for serving an SPA (single page application).
func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// get the absolute path to prevent directory traversal
	path, err := filepath.Abs(r.URL.Path)
	if err != nil {
		// if we failed to get the absolute path respond with a 400 bad request
		// and stop
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// prepend the path with the path to the static directory
	path = filepath.Join(h.staticPath, path)

	// println(path)
	// println(h.staticPath)
	// println(h.indexPath)

	// check whether a file exists at the given path
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		// file does not exist, serve index.html
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	} else if err != nil {
		// if we got an error (that wasn't that the file doesn't exist) stating the
		// file, return a 500 internal server error and stop
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// otherwise, use http.FileServer to serve the static dir
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

func TopicHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Title: %v\nId: %v\n", vars["title"], vars["id"])
}

// Logging Middleware
func logginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff her
		log.Println(r.RequestURI)
		// call the next handler, which can be another middleware in the chain,
		next.ServeHTTP(w, r)
	})
}

// use testcors
func TestCors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		return
	}

	w.Write([]byte("cors foo"))
}

func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:   80,
		MaxActive: 12000,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", ":6379")
			if err != nil {
				panic(err.Error())
			}
			return c, err
		},
	}
}

var pool = newPool()

func redis_something(str1 string, topic string) {
	println("str1")
	println(str1, topic)
	println("str1End")
	client := pool.Get()
	println(client)
	defer client.Close()
	// zadd mqttdata:101 1.55 "farm 2"
	_, err := client.Do("ZADD", "mqttdata:101", str1, topic)
	if err != nil {
		panic(err)
	}

	// value, err := client.Do("GET", "mykey")
	// if err != nil {
	// 	panic(err)
	// }

	// fmt.Printf("RRREDIS: %s \n", value)

}

// !
var knt int
var f MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {

	// fmt.Printf("MSG: %s\n", msg.Payload())
	msgdata := msg.Payload()
	// my_redis()
	myString := string(msgdata[:])
	println(myString)
	println(msg.Topic())
	println("Helo")
	redis_something(myString, msg.Topic())
	// println(msg.Payload())
	text := fmt.Sprintf("this is result msg #%d!", knt)
	knt++
	token := client.Publish("nn/result", 0, false, text)
	token.Wait()
}

func run_this(c <-chan os.Signal, wg *sync.WaitGroup) {
	defer wg.Done()

	knt = 0
	// c := make(chan os.Signal, 1)
	// signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	opts := MQTT.NewClientOptions().AddBroker("tcp://localhost:1883")
	opts.SetClientID("linux-go")
	opts.SetDefaultPublishHandler(f)
	topic := "home/+/main-light"
	// topic := "sensors/#"

	opts.OnConnect = func(c MQTT.Client) {
		if token := c.Subscribe(topic, 0, f); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}
	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to server\n")
	}
	<-c

}

func myfunc(c <-chan os.Signal, srv *http.Server, wg *sync.WaitGroup) {
	defer wg.Done()

	// c := make(chan os.Signal, 1)
	// signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	if err := srv.ListenAndServe(); err != nil {
		log.Println(err)
	}
	<-c
}

// func ArticlesCategoryHandler(w http.ResponseWriter, r *http.Request) {
//     vars := mux.Vars(r)
//     w.WriteHeader(http.StatusOK)
//     fmt.Fprintf(w, "Category: %v\n", vars["category"])
// }

func typeSwitchFunc(i interface{}) {
	switch j := i.(type) {

	case nil:
		// How do I print the type of j as interface{} ? Below only gives me the information on underlying type stored in j  -- (A)
		fmt.Printf("case nil: j is type %T, j value %v, i is type %T, i value %v\n", j, j, i, i)
		fmt.Printf("case nil: j is type: %v, j value: %v, j kind: %v\n", reflect.TypeOf(j), reflect.ValueOf(j), reflect.ValueOf(j).Kind())
	default:
		// How do I print the type of j as interface{} ? Below only gives me the information on underlying type stored in j -- (B)
		fmt.Printf("default: j is type %T, j value %v, i is type %T, i value %v\n", j, j, i, i)
		fmt.Printf("default: j is type: %v, j value: %v, j kind: %v\n", reflect.TypeOf(j), reflect.ValueOf(j), reflect.ValueOf(j).Kind())

	}
}

func GetDataFromRedis(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	client := pool.Get()
	defer client.Close()

	num, err := redis.Int64(client.Do("ZCARD", "mqttdata:101"))
	if err != nil {
		panic(err)
	}

	// print(int(num)-1, "int num -1")
	// zrange mqttdata:101 0 3 rev withscores
	data1, _ := redis.Strings(client.Do("ZRANGE", "mqttdata:101", "0", int(num)-1, "rev", "withscores"))
	println(strings.Join(data1, "|"))
	var temp bool = false
	var myd1 string
	json := simplejson.New()
	for p, q := range data1 {
		// println(p, q)
		if p%2 == 0 {
			println("even", p, q)
			myd1 = q
			temp = true
		} else {
			println("odd", p, q)
			if temp == true {
				println("Its true i need to put data in one", myd1)
				json.Set(myd1, q)
			}

		}
	}
	// typeSwitchFunc(num)
	// var four int64 = 4
	// fmt.Println(num == four)
	// l = num
	// num2 := string(num[:])
	// println(strconv.FormatInt(num), "Number of values?")
	// !
	// json := simplejson.New()
	// for i := 0; i < int(num); i++ {

	// 	json.Set("Someda"+strconv.Itoa(i), "aaa"+strconv.Itoa(i))
	// 	println(i)
	// }
	// json.Set("foo2", "bar2")
	payload, _ := json.MarshalJSON()
	// if err {
	// 	fmt.Printf(err)
	// }
	// json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	w.Write(payload)
}

func main() {
	// json := simplejson.New

	var wg sync.WaitGroup

	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	router := mux.NewRouter()
	router.Use(logginMiddleware)
	subroute := router.Host("mysite.com").Subrouter()

	client := pool.Get()
	defer client.Close()

	_, err := client.Do("SET", "mykey", "Hello from redigo!")
	if err != nil {
		panic(err)
	}

	value, err := client.Do("GET", "mykey")
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s \n", value)

	// !
	// var ctx = context.Background()
	// client := redis.NewClient(&redis.Options{
	// 	Addr: "localhost:6379",
	// 	Password: "",
	// 	DB: 0,
	// })
	// _, err := client.
	// // err := client.Set(ctx, "key", "value", 0).Err()
	// if err != nil {
	//     panic(err)
	// }

	// err = client.Set("name", "saurav", 0).Err()

	// if err != nil {
	// 	fmt.Println(err)
	// }
	// err = client.Set("id1234", 1, 0).Err()
	// if err != nil {
	//     fmt.Println(err)
	// }

	// res, err = client.Get("name").Result()
	// println(res)

	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		// an example API handler
		// println(r.Host)
		json.NewEncoder(w).Encode(map[string]string{"Data ": "true"})
	})

	router.HandleFunc("/api/mqtt/", GetDataFromRedis)
	// router.HandleFunc("/api/cors", TestCors).Methods(http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodOptions)

	// router.Use(mux.CORSMethodMiddleware(router))

	router.HandleFunc("/api/topic/{title}/{id:[0-9]+}", TopicHandler).Name("topic")

	router.Host("{subdomain}.mysite.com").
		Path("/api/tags/{title}/{id:[0-9]+}").
		HandlerFunc(TopicHandler).
		Name("tags")

	// url, err := router.Get("topic").URL("title", "I-can-do-it", "id", "101")
	// url2, err := router.Get("tags").URL("subdomain", "root", "title", "my-technology", "id", "103")

	// println(url2.Host + "" + url2.Path)

	// println("url")
	// println(url.Path)

	// // println(host.Host)

	// println("err")
	// println(err)

	subroute.HandleFunc("/api/test/{category}", func(w http.ResponseWriter, r *http.Request) {
		// an example API handler
		println(r.RequestURI)
		vars := mux.Vars(r)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Category: %v\n", vars["category"])
		// json.NewEncoder(w).Encode(map[string]string{"Data ": "true"})
	})

	spa := spaHandler{staticPath: "build", indexPath: "index.html"}
	router.PathPrefix("/").Handler(spa)

	// amw := authenticationMiddleware{tokenUsers: make(map[string]string)}
	// amw.Populate()

	// router.Use(amw.Middleware)

	srv := &http.Server{
		Handler: router,
		Addr:    "127.0.0.1:8000",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// log.Fatal(srv.ListenAndServe())
	wg.Add(2)
	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	go myfunc(c, srv, &wg)
	go run_this(c, &wg)
	close(c)

	fmt.Println("Waiting for goroutines to finish...")
	wg.Wait()
	fmt.Println("Done!")

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
	os.Exit(0)

}
