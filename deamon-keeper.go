package main

import (
	"context"
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
	pidFPath = kingpin.Flag("pid-file", "pid file path. if empty, then will not write pid to file").Short('p').Default("").String()
	command  = kingpin.Flag("cmd", "command to run").Short('c').Required().String()
	Continue = kingpin.Flag("continue", "continue running after command finished").Short('C').Bool()
	noHup    = kingpin.Flag("no-hup", "").Short('N').Bool()

	pid         int
	ctx, cancel = context.WithCancel(context.Background())
)

func main() {
	kingpin.Parse()

	log.Println("deamon_keeper start. pid:", os.Getpid())

	preStart()

	go runCmd(ctx, *command)
	go checkPidRunning(*pidFPath, *interval)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		log.Printf("deamon_keeper get a signal %s\n", s.String())
		switch s {
		case syscall.SIGTERM, syscall.SIGSTOP, syscall.SIGINT:
			log.Println("exec clean")
			clean()
			log.Println("exit")
			return
		case syscall.SIGQUIT:
			cancel()
			// log.Println("kill cmd with pid", pid)
			// err := syscall.Kill(pid, syscall.SIGQUIT)
			// if err != nil {
			// 	log.Println("kill cmd failed:", err)
			// }
		case syscall.SIGHUP:
			if *noHup {
				log.Println("no hup")
				continue
			}
		default:
			return
		}
	}
}

func runCmd(ctx context.Context, command string) {
	log.Println("exec", command)
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	// cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Println(err)
	}
	pid = cmd.Process.Pid
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
		log.Fatalf("another deamon_keeper is running cmd with pid %d", pid)
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
		// pid, _ := OS.ReadFirstLineAsInt(pidFPath)
		if pid > 0 && !OS.IsPidRunning(pid) {
			log.Printf("daemon pid(%d) is not running, restarting", pid)
			ctx, cancel = context.WithCancel(context.Background())
			runCmd(ctx, *command)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
