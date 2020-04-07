package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/szermatt/emacsclient"
)

var (
	clientOptions = emacsclient.OptionsFromFlags()
	input         = flag.Bool("input", false, "Put data from stdin into register")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "ereg write an Emacs register to stdout.\n")
		fmt.Fprintf(os.Stderr, "usage: ereg {args} [register]\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "  register\n")
		fmt.Fprintf(os.Stderr, "    \tRegister name (defaults to the kill ring)\n")
	}
	flag.Parse()

	var register string
	switch len(flag.Args()) {
	case 0:
		register = ""
	case 1:
		register = flag.Args()[0]
	default:
		fmt.Fprintf(os.Stderr, "ERROR: too many buffer names\n")
		flag.Usage()
		os.Exit(3)
	}

	c, err := emacsclient.Dial(clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	type templateArgs struct {
		Register string
		Fifo     string
	}
	args := &templateArgs{
		Register: register,
	}
	write := *input
	if write {
		if args.Fifo, err = emacsclient.StdinToFifo(); err != nil {
			log.Fatal(err)
		}
	}
	defer func() {
		if len(args.Fifo) != 0 {
			os.Remove(args.Fifo)
		}
	}()

	if err := emacsclient.SendEvalFromTemplate(
		c, args,
		`(with-temp-buffer
           (let ((reg {{if not .Register}}
                        nil
                      {{else if eq (len .Register) 1}}
                        {{char .Register}}
                      {{else}}
                        {{str .Register}}
                      {{end}}))
           {{if not .Fifo}}
             (if reg
                   (insert-register reg)
               (yank))
             (buffer-substring-no-properties (point-min) (point-max))
           {{else}}
             (insert-file-contents {{str .Fifo}})
             (if reg
                 (copy-to-register reg (point-min) (point-max))
               (copy-region-as-kill (point-min) (point-max)))
           {{end}}))`); err != nil {
		log.Fatal(err)
	}
	responses := make(chan emacsclient.Response, 1)
	go emacsclient.Receive(c, responses)
	if write {
		err = emacsclient.ConsumeAll(responses)
	} else {
		err = emacsclient.WriteUnquoted(responses, os.Stdout)
	}
	if err != nil {
		emacsclient.WriteError(err, os.Stderr)
		os.Exit(1)
	}
}
