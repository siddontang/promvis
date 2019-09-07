package main

import (
	"context"
	"flag"
	"io"
	"os"
	"time"

	"github.com/chzyer/readline"
	"github.com/pingcap/go-ycsb/pkg/util"
	"github.com/prometheus/common/model"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

var (
	promAddr = flag.String("prom", "http://127.0.0.1:9090", "Prometheus address")
)

func perr(err error) {
	if err == nil {
		return
	}

	println(err.Error())
	os.Exit(1)
}

func newPromClientAPI() v1.API {
	client, err := api.NewClient(api.Config{
		Address: *promAddr,
	})
	perr(err)
	api := v1.NewAPI(client)
	return api
}

var promAPI v1.API

func queryValue(query string) model.Value {
	r := v1.Range{
		Start: time.Now().Add(-time.Minute * 15),
		End:   time.Now(),
		Step:  time.Second * 15,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	result, _, err := promAPI.QueryRange(ctx, query, r)
	cancel()
	perr(err)
	return result
}

func queryData(query string) []float64 {
	result := queryValue(query)

	m, ok := result.(model.Matrix)
	if !ok || len(m) == 0 {
		return nil
	}

	data := make([]float64, len(m[0].Values))
	for i, v := range m[0].Values {
		data[i] = float64(v.Value)
	}
	return data
}

func render(query string) {
	err := ui.Init()
	perr(err)
	defer ui.Close()

	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(15 * time.Second)

	defer ticker.Stop()

	_, termHeight := ui.TerminalDimensions()

	data := queryData(query)

	lc := widgets.NewPlot()
	lc.Title = query
	lc.Data = make([][]float64, 1)
	lc.Data[0] = data
	lc.SetRect(10, 10, 70, termHeight-10)
	lc.AxesColor = ui.ColorWhite
	lc.LineColors[0] = ui.ColorRed
	lc.Marker = widgets.MarkerDot

	ui.Render(lc)

	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return
			}
		case <-ticker.C:
			data = queryData(query)
			lc.Data[0] = data
			ui.Render(lc)
		}
	}
}

func main() {
	flag.Parse()

	promAPI = newPromClientAPI()

	l, err := readline.NewEx(&readline.Config{
		Prompt:            "\033[31mInput your Prometheus Query -> \033[0m ",
		HistoryFile:       "/tmp/readline.tmp",
		InterruptPrompt:   "^C",
		EOFPrompt:         "^D",
		HistorySearchFold: true,
	})
	if err != nil {
		util.Fatal(err)
	}
	defer l.Close()

	for {
		line, err := l.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				return
			} else if err == io.EOF {
				return
			}
			continue
		}
		if line == "exit" {
			return
		}

		render(line)
	}
}
