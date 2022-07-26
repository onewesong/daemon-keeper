package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	OS "github.com/onewesong/go-common/os"
)

var (
	interval = kingpin.Flag("interval", "check pid running interval").Short('i').Default("1").Int()
	pidFPath = kingpin.Flag("pid-file", "pid file path. if empty, then will not write pid to file").Short('p').Default("/tmp/deamon.pid").String()
	command  = kingpin.Flag("cmd", "command to run").Short('c').Required().String()
	Continue = kingpin.Flag("continue", "continue running after command finished").Short('C').Bool()
)

func main() {
	kingpin.Parse()

	preStart()
	go runCmd(*command)
	go checkPidRunning(*pidFPath, *interval)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		log.Printf("pppd_keeper get a signal %s\n", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGSTOP, syscall.SIGINT:
			log.Println("exec clean")
			clean()
			log.Println("exit")
			return
		case syscall.SIGHUP:
			// keep running
		default:
			return
		}
	}
}

func runCmd(command string) {
	log.Println("exec", command)
	cmd := exec.Command("bash", "-c", command)
	// cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Println(err)
	}
	pid := cmd.Process.Pid
	writePidFile(pid)
	cmd.Wait()
	if cmd.ProcessState.Success() {
		if !*Continue {
			log.Println("cmd normal finished, exit")
			clean()
			os.Exit(0)
		}
	}
}

func preStart() {
	pid, _ := OS.ReadFirstLineAsInt(*pidFPath)
	if pid > 0 && OS.IsPidRunning(pid) {
		log.Fatalf("another pppd_keeper is running with pid %d", pid)
	}
}

func clean() {
	os.Remove(*pidFPath)
}

func writePidFile(pid int) {
	if *pidFPath == "" {
		return
	}
	err := os.WriteFile(*pidFPath, []byte(strconv.Itoa(pid)), 0644)
	if err != nil {
		log.Fatalf("writePidFile failed: %s", err)
	}
	log.Println("wrote pid file to", *pidFPath)
}

func checkPidRunning(pidFPath string, interval int) {
	for {
		pid, _ := OS.ReadFirstLineAsInt(pidFPath)
		if pid > 0 && !OS.IsPidRunning(pid) {
			log.Printf("daemon pid(%d) is not running, restarting", pid)
			runCmd(*command)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
