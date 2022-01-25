package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

const daemonfile = "fileserver.service"
const daemondirpath = "/usr/lib/systemd/system/" + daemonfile

var (
	port = flag.String("p", "1070", "listen to port.")
	mode = flag.String("m", "release", "debug is a dev mode.")
	dir  = flag.String("d", "./", "files dir.")
	cmd  = flag.String("c", "", "set start, to exec fileserver in a daemon. set stop, to quit the daemon. the default is start a normal process.")
)

func main() {
	flag.Parse()
	switch *cmd {
	case "start":
		start()
	case "stop":
		stop()
	default:
		// // 1. use gin framework
		// gin.SetMode(*mode)
		// router := gin.Default()
		// router.Static("/", *dir)
		// fmt.Printf("hosting a dir(%s) and listening port(%s) ...\n", *dir, *port)
		// router.Run("0.0.0.0:" + *port)

		// 2. use native net/http
		http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			start := time.Now()
			http.FileServer(http.Dir(*dir)).ServeHTTP(rw, r)
			end := time.Now()

			path := r.URL.Path
			if r.URL.RawQuery != "" {
				path = path + "?" + r.URL.RawQuery
			}
			ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
			if err != nil {
				ip = ""
			}
			fmt.Fprintf(
				os.Stdout,
				"[http] %v | %13v | %10s | %-4s %#v\n",
				end.Format("2006/01/02 15:04:05"),
				end.Sub(start),
				net.ParseIP(ip),
				r.Method,
				path,
			)
		})
		fmt.Printf("hosting a dir(%s) and listening port(%s) ...\n", *dir, *port)
		http.ListenAndServe(":"+*port, nil)
	}
}

func start() {
	// 创建daemon服务文件
	runtimeFlag := fmt.Sprintf("-m %s -p %s -d %s", *mode, *port, *dir)
	tpl, err := template.New("tpl").Parse(`
[Unit]
Description=Files server daemon

[Service]
ExecStart=/usr/bin/fileserver {{.}}
ExecReload=/bin/kill -HUP $MAINPID
Type=simple
KillMode=process
Restart=on-failure
RestartSec=42s

[Install]
WantedBy=multi-user.target
	`)
	if err != nil {
		log.Fatal(err)
	}
	configBuf := bytes.Buffer{}
	err = tpl.Execute(&configBuf, runtimeFlag)
	if err != nil {
		log.Fatal(err)
	}
	fd := &os.File{}
	defer fd.Close()
	if _, e := os.Stat(daemondirpath); e == nil {
		// 删除旧文件
		e = os.Remove(daemondirpath)
		if e != nil {
			log.Fatal(e.Error())
		}
	}
	fd, err = os.Create(daemondirpath)
	if err != nil {
		log.Fatal(err.Error())
	}
	_, err = io.WriteString(fd, configBuf.String())
	if err != nil {
		log.Fatal(err.Error())
	}

	// 立即运行fileserver
	cmd := exec.Command("systemctl", "start", daemonfile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	if !cmd.ProcessState.Success() {
		return
	}

	// 设置开机启动
	cmd = exec.Command("systemctl", "enable", daemonfile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	if !cmd.ProcessState.Success() {
		return
	}

	// 查看status
	cmd = exec.Command("systemctl", "status", daemonfile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	if !cmd.ProcessState.Success() {
		return
	}

	fmt.Println("start fileserver daemon success.")
}

func stop() {
	// 禁止开机启动
	cmd := exec.Command("systemctl", "disable", daemonfile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	if !cmd.ProcessState.Success() {
		return
	}

	// 停止运行
	cmd = exec.Command("systemctl", "stop", daemonfile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	if !cmd.ProcessState.Success() {
		return
	}

	fmt.Println("stop fileserver daemon success.")
}
