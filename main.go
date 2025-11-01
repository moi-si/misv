package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/proxy"
)

var (
	bindAddr     = flag.String("bind", "", "Bind address (required)")
	originServer = flag.String("origin", "", "Origin server domain (required)")
	rootDir      = flag.String("root", "", "Server root directory (defaults to `./<origin>`)")
	socks5Addr   = flag.String("socks5", "", "SOCKS5 proxy address")
	ua           = flag.String("ua", "", "Custom User-Agent header")
	xff          = flag.String("xff", "", "Custom X-Forwarded-For header")
	xffEnabled   bool
	httpClient   *http.Client
)

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `moi-si/misv v0.1.0 - A simple proxying static file server

Usage:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *bindAddr == "" {
		panic("missing required argument: -bind")
	}
	if *originServer == "" {
		panic("missing required argument: -origin")
	}

	if *rootDir == "" {
		fmt.Printf("Root directory not specified; defaulting to `./%s`.\n", *originServer)
		*rootDir = *originServer
		if info, err := os.Stat(*rootDir); os.IsNotExist(err) {
			if err := os.MkdirAll(*rootDir, os.ModePerm); err != nil {
				panic(fmt.Sprintf("create %s: %s", *rootDir, err))
			}
			fmt.Println(*rootDir, "not found. Created automatically.")
		} else if err != nil {
			panic(fmt.Sprintf("accessing %s: %s", *rootDir, err))
		} else if !info.IsDir() {
			panic(fmt.Sprintln(*rootDir, "is not a directory"))
		}
	} else {
		if info, err := os.Stat(*rootDir); err == nil {
			if !info.IsDir() {
				panic(fmt.Sprintln(*rootDir, "is not a directory"))
			}
		} else if os.IsNotExist(err) {
			if err = os.MkdirAll(*rootDir, os.ModePerm); err == nil {
				fmt.Println("Root directory", *rootDir, "not found. Created automatically.")
			} else {
				panic(fmt.Sprintf("create %s: %s", *rootDir, err))
			}
		} else {
			panic(fmt.Sprintf("accessing %s: %s", *rootDir, err))
		}
	}

	httpClient = http.DefaultClient
	if *socks5Addr != "" {
		dialer, err := proxy.SOCKS5("tcp", *socks5Addr, nil, proxy.Direct)
		if err != nil {
			panic(fmt.Sprintf("create SOCKS5 dialer: %s", err))
		}
		httpClient.Transport = &http.Transport{
			Dial: dialer.Dial,
		}
	}

	if *ua == "" {
		*ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:146.0) Gecko/20100101 Firefox/146.0"
	}
	xffEnabled = *xff != ""

	http.HandleFunc("/", handle)
	fmt.Println("Listen on", *bindAddr)
	if err := http.ListenAndServe(*bindAddr, nil); err != nil {
		panic(err)
	}
}

func fetch(w http.ResponseWriter, filePath, urlPath string) bool {
	originURL := "https://" + *originServer + urlPath
	log.Println(filePath, "not found, fetching from origin URL...")
	req, err := http.NewRequest("GET", originURL, nil)
	if err != nil {
		log.Println("Failed to create new request:", err)
		http.Error(w, fmt.Sprintln("Failed to create new request:", err), 500)
		return true
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", *ua)
	if xffEnabled {
		req.Header.Set("X-Forwarded-For", *xff)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Println("Failed to fetch file:", err)
		http.Error(w, fmt.Sprintln("Failed to fetch file:", err), http.StatusBadGateway)
		return true
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("Origin server response:", resp.Status)
		http.Error(w, fmt.Sprintln("Origin server response:", resp.Status), http.StatusBadGateway)
		return true
	}
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Printf("Failed to create %s: %s", dir, err)
		http.Error(w, fmt.Sprintf("Failed to create %s: %s", dir, err), 500)
		return true
	}
	outFile, err := os.Create(filePath)
	if err != nil {
		log.Printf("Failed to create %s: %s", filePath, err)
		http.Error(w, fmt.Sprintf("Failed to create %s: %s", dir, err), 500)
		return true
	}
	defer func() {
		if err = outFile.Close(); err != nil {
			log.Printf("Failed to close %s: %s", filePath, err)
		}
	}()
	if _, err = io.Copy(outFile, resp.Body); err != nil {
		log.Printf("Failed to write %s: %s", filePath, err)
		http.Error(w, fmt.Sprintf("Failed to write %s: %s", filePath, err), 500)
		return true
	}
	return false
}

func handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "only GET is allowed", http.StatusMethodNotAllowed)
		return
	}
	urlPath := path.Clean(r.URL.Path)
	if !strings.HasPrefix(urlPath, "/") {
		log.Println("Invalid path: ", urlPath)
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	filePath := filepath.Join(*rootDir, urlPath)
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		if fetch(w, filePath, urlPath) {
			return
		}
	} else if err != nil {
		log.Printf("Accessing %s: %s", filePath, err)
		http.Error(w, fmt.Sprintln("cannot open file: ", err), 500)
		return
	} else if info.IsDir() {
		filePath = filepath.Join(*rootDir, "index.html")
		if info, err = os.Stat(filePath); os.IsNotExist(err) {
			if fetch(w, filePath, urlPath) {
				return
			}
		} else if err != nil {
			log.Printf("Accessing %s: %s", filePath, err)
			http.Error(w, fmt.Sprintln("cannot open file: ", err), 500)
			return
		} else if info.IsDir() {
			log.Println(filePath, "is a directory")
			http.Error(w, fmt.Sprintln(filePath, "is a directory"), 500)
			return
		}
	}

	http.ServeFile(w, r, filePath)
}
