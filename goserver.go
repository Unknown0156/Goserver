package main

import (
	"bufio"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/search"
)

var templates = template.Must(template.ParseFiles("templates/edit.html"))
var validPath = regexp.MustCompile("^/(edit/|save/|)([a-zA-Z0-9]+)$")

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := p.Title
	if !strings.Contains(p.Title, ".") {
		filename += ".html"
	}
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func (p *Page) load(title string) error {
	filename := title
	if !strings.Contains(title, ".") {
		filename += ".html"
	}
	p.Title = title
	var err error
	p.Body, err = ioutil.ReadFile(filename)
	return err
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	if err := templates.ExecuteTemplate(w, tmpl+".html", p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func htmlHandler(w http.ResponseWriter, r *http.Request, title string) {
	log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
	var p Page
	if err := p.load(title); err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}

	index, ok := substringIndex(string(p.Body), "</body>")
	edit := []byte("")
	if ok {
		p.Body = append(p.Body[:index], append(edit, p.Body[index:]...)...)
	} else {
		p.Body = edit
	}
	//w.Header().Set("Cache-Control", "max-age=8640, public")
	w.Write(p.Body)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
	var p Page
	if err := p.load(title); err != nil {
		p = Page{Title: title}
	}
	renderTemplate(w, "edit", &p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	if err := p.save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+title, http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fn(w, r, "index")
			return
		}
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func resHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
	var p Page
	p.Title = r.URL.Path[1:]
	if err := p.load(p.Title); err != nil {
		fmt.Printf("%v", err)
		return
	}
	var contentType string
	if strings.HasSuffix(p.Title, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(p.Title, ".png") || strings.HasSuffix(p.Title, ".jpg") {
		contentType = "image/png"
	} else {
		contentType = "text/plain"
	}
	w.Header().Add("Content-Type", contentType)
	//w.Header().Set("Cache-Control", "max-age=8640, public")
	w.Write(p.Body)
}

func substringIndex(str string, substr string) (int, bool) {
	m := search.New(language.English, search.IgnoreCase)
	start, _ := m.IndexString(str, substr)
	if start == -1 {
		return start, false
	}
	return start, true
}

func startServer(logger *log.Logger, addr string, srvch chan string) *http.Server {
	srv := &http.Server{Addr: addr}
	go func() {
		defer func() {
			srvch <- srv.Addr
		}()
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatalf("ListenAndServe (%s): %v", srv.Addr, err)
		} else {
			logger.Printf("Server %s shuting down", srv.Addr)
		}
	}()
	logger.Printf("Server %s started", srv.Addr)
	return srv
}

func main() {
	logger := log.New(os.Stdout, "http: ", log.LstdFlags)
	srvch := make(chan string)
	srvmap := make(map[string]*http.Server)
	srvmap[":80"] = startServer(logger, ":80", srvch)
	http.HandleFunc("/", makeHandler(htmlHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.HandleFunc("/fonts/", resHandler)
	http.HandleFunc("/css/", resHandler)
	http.HandleFunc("/images/", resHandler)
	http.HandleFunc("/script/", resHandler)
	http.HandleFunc("/json/", resHandler)

	//Server shell
	fmt.Println("GoServer Shell")
	fmt.Println("---------------------")
	time.Sleep(1 * time.Second)
	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case srvaddr := <-srvch:
			_, ok := srvmap[srvaddr]
			if ok {
				delete(srvmap, srvaddr)
			} else {
				logger.Panicf("No %s in server's map", srvaddr)
			}
			continue
		default:
		}
		fmt.Print(">")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		input = strings.TrimSuffix(input, "\n")
		input = strings.TrimSuffix(input, "\r")
		if input == "exit" {
			for _, srv := range srvmap {
				srv.Shutdown(context.Background())
			}
			break
		}
		switch input {
		case "help":
			fmt.Println("Commands: start, stop, status, exit")
		case "start":
			newaddr := ":80"
			_, ok := srvmap[newaddr]
			if ok {
				fmt.Println("Server is already running!")
			} else {
				srvmap[newaddr] = startServer(logger, newaddr, srvch)
			}
		case "stop":
			stopaddr := ":80"
			srv, ok := srvmap[stopaddr]
			if ok {
				srv.Shutdown(context.Background())
			} else {
				fmt.Println("No such server running!")
			}
		case "status":
			for addr, _ := range srvmap {
				fmt.Printf("Server %s is runing\n", addr)
			}
		default:
			fmt.Println("No such command, use 'help'")
		}
	}
}
