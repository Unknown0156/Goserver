package main

import (
	"bufio"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type Page struct {
	Title string
	Body  []byte
}

var templates = template.Must(template.ParseFiles("templates/view.html", "templates/edit.html"))
var validPath = regexp.MustCompile("^/(edit/|save/|)([a-zA-Z0-9]+)$")

func (p *Page) save() error {
	filename := p.Title + ".html"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func (p *Page) load(title string) error {
	filename := title + ".html"
	p.Title = title
	var err error
	p.Body, err = ioutil.ReadFile(filename)
	return err
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request, title string) {
	var p Page
	err := p.load(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	w.Write(p.Body)
	//renderTemplate(w, "view", &p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	var p Page
	err := p.load(title)
	if err != nil {
		p = Page{Title: title}
	}
	renderTemplate(w, "edit", &p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
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

func execInput(input string) error {
	input = strings.TrimSuffix(input, "\n")
	cmd := exec.Command(input)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func main() {
	var srv http.Server
	srv.Addr = ":80"
	http.HandleFunc("/", makeHandler(mainHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	var err error
	go func() {
		err = srv.ListenAndServe()
	}()
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("GoServer Shell")
	fmt.Println("---------------------")
	fmt.Println("Server started")
	for {
		fmt.Print("-> ")
		input, err := reader.ReadString('\n')
		input = strings.TrimSuffix(input, "\n")
		input = strings.TrimSuffix(input, "\r")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		if input == "exit" {
			break
		}
		switch input {
		case "start":
			go func() {
				err = srv.ListenAndServe()
			}()
			fmt.Println("Server started")
		case "shutdown":
			srv.Shutdown(context.Background())
			fmt.Println("Server shutdown")
		default:
			fmt.Println("No such command")
		}
	}
	//log.Fatal(http.ListenAndServe(":80", nil))
}

/*TODO
Terminate command
TestLoad css
TestLoad Image
Edit btn
*/
