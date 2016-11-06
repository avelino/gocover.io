package views

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/avelino/cover.run/redis"
	r "github.com/garyburd/redigo/redis"
)

var (
	dockerSocket = flag.String("s", "", "Dockerd socket (e.g., /var/run/docker.sock)")
	dockerAddr   = flag.String("d", "", "Dockerd addr (e.g., 127.0.0.1:2375)")
	serveAddr    = flag.String("p", ":8080", "Address and port to serve")
	serveSAddr   = flag.String("ps", ":80443", "Address and port to serve HTTPS")
	redisAddr    = flag.String("r", "127.0.0.1:6379", "redis address")
	redisPass    = flag.String("rp", "", "redis password")
	certPath     = flag.String("tls", "", "cert path")
)

func docker(repo, version string, pool *r.Pool) string {
	var (
		worker = "vieux/gocover"
		conn   = pool.Get()
	)

	defer conn.Close()

	if version != "" {
		worker = worker + ":" + version
	}

	if version == "" {
		if cached, fresh, err := redis.GetRepo(conn, repo); err != nil {
			return err.Error()
		} else if fresh {
			return string(cached)
		}
	}

	host := "unix:///var/run/docker.sock"

	if *dockerSocket != "" {
		host = "unix://" + *dockerSocket
	} else if *dockerAddr != "" {
		host = "tcp://" + *dockerAddr
	}

	fmt.Println(repo)
	out, err := exec.Command("docker", "-H", host, "run", "--rm", "-a", "stdout", "-a", "stderr", worker, repo).CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		if strings.Contains(string(out), "Unable to find image") {
			return "go version '" + version + "' not found"
		}
		return string(out)
	}
	re, err := regexp.Compile("\\<script[\\S\\s]+?\\</script\\>")
	if err != nil {
		return err.Error()
	}
	content := re.ReplaceAllString(string(out), "")
	content = strings.Replace(content, "background: black;", "background: #222222;", 2)

	content = strings.Replace(content, ".cov1 { color: rgb(128, 128, 128) }", ".cov1 { color: #52987D }", 2)
	content = strings.Replace(content, ".cov2 { color: rgb(128, 128, 128) }", ".cov2 { color: #4BA180 }", 2)
	content = strings.Replace(content, ".cov3 { color: rgb(128, 128, 128) }", ".cov3 { color: #44AA83 }", 2)
	content = strings.Replace(content, ".cov4 { color: rgb(128, 128, 128) }", ".cov4 { color: #3DB487 }", 2)
	content = strings.Replace(content, ".cov5 { color: rgb(128, 128, 128) }", ".cov5 { color: #36BD8A }", 2)
	content = strings.Replace(content, ".cov6 { color: rgb(128, 128, 128) }", ".cov6 { color: #2FC68D }", 2)
	content = strings.Replace(content, ".cov7 { color: rgb(128, 128, 128) }", ".cov7 { color: #28D091 }", 2)
	content = strings.Replace(content, ".cov8 { color: rgb(128, 128, 128) }", ".cov8 { color: #21D994 }", 2)
	content = strings.Replace(content, ".cov9 { color: rgb(128, 128, 128) }", ".cov9 { color: #1AE297 }", 2)
	content = strings.Replace(content, "<option value=\"file0\">", "<option value=\"file0\" select=\"selected\">", -1)
	content = strings.Replace(content, "\">"+repo, "\">", -1)

	re = regexp.MustCompile("-- cov:([0-9.]*) --")
	matches := re.FindStringSubmatch(content)
	if len(matches) == 2 {
		cov, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			content = strings.Replace(content, "<select id=", fmt.Sprintf("<span class='cov%d'>%s%%</span> | <select id=", int((cov-0.0001)/10), matches[1]), 1)
		}
		if version != "" {
			content = strings.Replace(content, "<select id=", fmt.Sprintf("<span>%s</span> | <select id=", version), 1)
		} else {
			redis.SetCache(conn, repo, content, matches[1])
		}
	} else if version != "" {
		content = strings.Replace(content, "<select id=", fmt.Sprintf("<span>%s</span> | <select id=", version), 1)
	} else {
		redis.SetCache(conn, repo, content, "-1")
	}
	if version == "" {
		redis.SetStats(conn, repo)
	}
	return content
}

// HandleAbout flat page about
func HandleAbout(w http.ResponseWriter, r *http.Request) {
	Body := map[string]interface{}{"about_active": "active"}

	t := template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/about.tmpl"))
	t.Execute(w, Body)
}

// HandleHome home page
// get top process, get via redis
func HandleHome(w http.ResponseWriter, r *http.Request) {
	flag.Parse()

	pool, err := redis.NewPool("tcp", *redisAddr, *redisPass)
	if err != nil {
		log.Fatalf("%v", err)
	}

	conn := pool.Get()
	defer conn.Close()

	top, err := redis.Top(conn, "top", 4)
	if err != nil {
		log.Println(err.Error())
	}
	last, err := redis.Top(conn, "last", 4)
	if err != nil {
		log.Println(err.Error())
	}

	Body := map[string]interface{}{"top": top, "last": last, "cover_active": "active"}

	t := template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/cover.tmpl"))
	t.Execute(w, Body)
}

// HandleRepo start process cover
func HandleRepo(w http.ResponseWriter, r *http.Request) {
	flag.Parse()

	pool, err := redis.NewPool("tcp", *redisAddr, *redisPass)
	if err != nil {
		log.Fatalf("%v", err)
	}

	var (
		repo    = r.RequestURI[1:len(r.RequestURI)]
		conn    = pool.Get()
		version = ""
	)
	defer conn.Close()

	if r.ParseForm() == nil {
		version = r.FormValue("version")
	}

	Body := make(map[string]interface{})
	t := template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/cached.tmpl"))
	if cached, fresh, err := redis.GetRepo(conn, repo+version); err != nil {
		Body["cover_active"] = "active"
		Body["error"] = err
	} else if fresh {
		redis.SetStats(conn, repo)
		Body["repo"] = repo
		Body["cover_active"] = "active"
		Body["cache"] = template.HTML(cached)
		Body["version"] = version
	} else {
		Body["repo"] = repo
		Body["cover_active"] = "active"
		Body["version"] = version
		if cached != "" {
			Body["cache"] = "ok"
		}
		t = template.Must(template.ParseFiles("./templates/layout.tmpl", "./templates/loading.tmpl"))
	}
	t.Execute(w, Body)
}

// HandleDocker up docker to testing runing
func HandleDocker(w http.ResponseWriter, r *http.Request) {
	flag.Parse()

	pool, err := redis.NewPool("tcp", *redisAddr, *redisPass)
	if err != nil {
		log.Fatalf("%v", err)
	}

	repo := strings.Split(r.RequestURI[3:len(r.RequestURI)], "?")[0]
	version := r.FormValue("version")
	if version == "" {
		version = "1.7"
	}
	ret := docker(repo, version, pool)
	if ret == "" {
		ret = "Error"
	}
	w.Write([]byte(ret))
}

// HandleCache find object on cache (redis)
func HandleCache(w http.ResponseWriter, r *http.Request) {
	flag.Parse()

	pool, err := redis.NewPool("tcp", *redisAddr, *redisPass)
	if err != nil {
		log.Fatalf("%v", err)
	}

	var (
		repo = r.RequestURI[1:len(r.RequestURI)]
		conn = pool.Get()
	)
	defer conn.Close()

	if r.FormValue("version") == "" {
		if cached, _, err := redis.GetRepo(conn, repo); err != nil {
			w.Write([]byte(err.Error()))
		} else if cached != "" {
			redis.SetStats(conn, repo)
			w.Write([]byte(string(cached)))
		}
	}
	w.Write([]byte("No cached version of " + repo))
}

// HandleBadge shields.io image URL for amount coverage
func HandleBadge(w http.ResponseWriter, r *http.Request) {
	flag.Parse()

	pool, err := redis.NewPool("tcp", *redisAddr, *redisPass)
	if err != nil {
		log.Fatalf("%v", err)
	}

	var (
		repo     = r.RequestURI[1:len(r.RequestURI)]
		conn     = pool.Get()
		badgeURL = ""
	)
	defer conn.Close()

	if coverage, err := redis.GetCoverage(conn, repo); err != nil {
		badgeURL = fmt.Sprintf("https://img.shields.io/badge/coverage-error-lightgrey.svg?style=flat")
	} else if coverage < 25.0 {
		badgeURL = fmt.Sprintf("https://img.shields.io/badge/coverage-%.1f%%25-red.svg?style=flat", coverage)
	} else if coverage < 50.0 {
		badgeURL = fmt.Sprintf("https://img.shields.io/badge/coverage-%.1f%%25-orange.svg?style=flat", coverage)
	} else if coverage < 75.0 {
		badgeURL = fmt.Sprintf("https://img.shields.io/badge/coverage-%.1f%%25-green.svg?style=flat", coverage)
	} else {
		badgeURL = fmt.Sprintf("https://img.shields.io/badge/coverage-%.1f%%25-brightgreen.svg?style=flat", coverage)
	}

	http.Redirect(w, r, badgeURL, 301)
}
