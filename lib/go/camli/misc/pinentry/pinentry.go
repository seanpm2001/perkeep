/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pinentry

import (
	"bufio"
	"exec"
	"fmt"
	"log"
	"os"
	"strings"
)

var _ = log.Printf

var ErrCancel = os.NewError("pinentry: Cancel")

type Request struct {
	Desc, Prompt, OK, Cancel, Error string
}

func (r *Request) GetPIN() (pin string, outerr os.Error) {
	defer func() {
		if e, ok := recover().(string); ok {
			pin = ""
			outerr = os.NewError(e)
		}
	}()
	bin, err := exec.LookPath("pinentry")
	if err != nil {
		return r.getPINNaïve()
	}
	c, err := exec.Run(bin,
		[]string{bin},
		os.Environ(),
		"/",
		exec.Pipe,
		exec.Pipe,
		exec.DevNull)
	if err != nil {
		return "", err
	}
	defer func() {
		c.Stdin.Close()
		c.Stdout.Close()
		c.Close()
		c.Wait(0)
	}()
	br := bufio.NewReader(c.Stdout)
	lineb, _, err := br.ReadLine()
	if err != nil {
		return "", fmt.Errorf("Failed to get getpin greeting")
	}
	line := string(lineb)
	if !strings.HasPrefix(line, "OK") {
		return "", fmt.Errorf("getpin greeting said %q", line)
	}
	set := func(cmd string, val string) {
		if val == "" {
			return
		}
		fmt.Fprintf(c.Stdin, "%s %s\n", cmd, val)
		line, _, err := br.ReadLine()
		if err != nil {
			panic("Failed to " + cmd)
		}
		if string(line) != "OK" {
			panic("Response to " + cmd + " was " + string(line))
		}
	}
	set("SETPROMPT", r.Prompt)
	set("SETDESC", r.Desc)
	set("SETOK", r.OK)
	set("SETCANCEL", r.Cancel)
	set("SETERROR", r.Error)
	set("OPTION", "ttytype=" + os.Getenv("TERM"))
	tty, err := os.Readlink("/proc/self/fd/0")
	if err == nil {
		set("OPTION", "ttyname=" + tty)
	}
	fmt.Fprintf(c.Stdin, "GETPIN\n")
	lineb, _, err = br.ReadLine()
	if err != nil {
		return "", fmt.Errorf("Failed to read line after GETPIN: %v", err)
	}
	line = string(lineb)
	if strings.HasPrefix(line, "D ") {
		return line[2:], nil
	}
	if strings.HasPrefix(line, "ERR 83886179 ") {
		return "", ErrCancel
	}
	return "", fmt.Errorf("GETPIN response didn't start with D; got %q", line)
}

func runPass(bin string, args ...string) {
	a := []string{bin}
	a = append(a, args...)
	c, err := exec.Run(bin, a, os.Environ(), "/", exec.PassThrough, exec.PassThrough, exec.PassThrough)
	if err != nil {
		return
	}
	defer func() {
		c.Close()
		c.Wait(0)
	}()
}

func (r *Request) getPINNaïve() (string, os.Error) {
	stty, err := exec.LookPath("stty")
	if err != nil {
		return "", os.NewError("no pinentry or stty found")
	}
	runPass(stty, "-echo")
	defer runPass(stty, "echo")

	if r.Desc != "" {
		fmt.Printf("%s\n\n", r.Desc)
	}
	prompt := r.Prompt
	if prompt == "" {
		prompt = "Password"
	}
	fmt.Printf("%s: ", prompt)
	br := bufio.NewReader(os.Stdin)
	line, _, err := br.ReadLine()
	if err != nil {
		return "", err
	}
	return string(line), nil
}