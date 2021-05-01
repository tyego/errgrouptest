package main

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

/*

功能描述：
使用 errgroup wait 两个 goroutine, 其中一个goroutine退出时，让另一个goroutine也优雅退出：
	goroutine1：
		启动 http server，监听本地8080端口，两个接口：一个/test接口，一个关停/stop接口
		当访问/close接口时，关闭server服务（同时该 goroutine 返回 err）
	goroutine2：
		监听 ctrl+c 信号，收到信号时，关闭 server 服务，并返回err
*/

func main() {

	g, ctx := errgroup.WithContext(context.Background())

	// prepare http
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	mux.HandleFunc("/test", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("200"))
		request.Body.Close()
	})
	// call this api stop http server
	mux.HandleFunc("/stop", func(writer http.ResponseWriter, request *http.Request) {
		request.Body.Close()
		server.Close()
	})

	// prepare signal
	c := make(chan os.Signal)
	stop := make(chan struct{}, 1)

	// start http
	g.Go(func() error {
		fmt.Println("http start")
		err := server.ListenAndServe()
		if err != nil {
			select {
			case <-stop: // received close signal
				fmt.Println("http close")
				return nil
			default:
			}

			fmt.Println("http err")
			return fmt.Errorf("server end, err = [%+v]", err)
		}
		return nil
	})

	// start signal
	g.Go(func() error {
		signal.Notify(c, syscall.SIGINT) // ctrl+c

		fmt.Println("signal start")
		select {
		case <-ctx.Done():
			fmt.Println("signal ctx down")
			return ctx.Err()
		case s := <-c:
			fmt.Println("signal:", s)
			fmt.Println("waiting stop...")
			// stop http
			stop <- struct{}{}
			server.Close()
			close(stop)
			// stop signal
			signal.Stop(c)
			fmt.Println("signal end")
			return errors.New("signal stop")
		}
	})

	// wait
	if err := g.Wait(); err != nil {
		fmt.Printf("wait err = %+v\n", err)
	}
	fmt.Println("end")
}
