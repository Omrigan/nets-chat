package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/marcusolsson/tui-go"
	"golang.org/x/sync/errgroup"
)

func main() {
	conn, err := net.Dial("tcp", ":8080")
	if err != nil {
		log.Fatal(err.Error())
	}
	scan := bufio.NewScanner(conn)

	history := tui.NewVBox()
	historyScroll := tui.NewScrollArea(history)
	historyScroll.SetAutoscrollToBottom(true)

	historyBox := tui.NewVBox(historyScroll)
	historyBox.SetBorder(true)

	input := tui.NewEntry()
	input.SetFocused(true)
	input.SetSizePolicy(tui.Expanding, tui.Maximum)

	inputBox := tui.NewHBox(input)
	inputBox.SetBorder(true)
	inputBox.SetSizePolicy(tui.Expanding, tui.Maximum)

	chat := tui.NewVBox(historyBox, inputBox)
	chat.SetSizePolicy(tui.Expanding, tui.Expanding)

	input.OnSubmit(func(e *tui.Entry) {
		str := fmt.Sprintf("%v\r\n", e.Text())
		io.WriteString(conn, str)
		input.SetText("")
	})

	ui, err := tui.New(chat)
	if err != nil {
		log.Fatal(err)
	}

	ui.SetKeybinding("Esc", func() { ui.Quit() })

	var g errgroup.Group
	g.Go(func() error {
		for {
			res := scan.Scan()
			if !res {
				log.Fatal("Cannot read from connection")
			}
			str := scan.Text()
			ui.Update(func() {
				history.Append(tui.NewLabel(str))
			})
		}
		return nil
	})
	g.Go(ui.Run)

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}
