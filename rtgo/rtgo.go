//    Title: rtgo.go
//    Author: Jon Cody
//
//    This program is free software: you can redistribute it and/or modify
//    it under the terms of the GNU General Public License as published by
//    the Free Software Foundation, either version 3 of the License, or
//    (at your option) any later version.
//
//    This program is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//    GNU General Public License for more details.
//
//    You should have received a copy of the GNU General Public License
//    along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

var (
	remjsline  = "([[:space:]]*)<script type=\"application/javascript\" src=\"(\\.?)%s\"></script>([[:space:]]*)"
	addjsline  = "\n        <script type=\"application/javascript\" src=\"%s\"></script>\n    </body>\n"
	viewtext   = "{{ define \"%s\" }}\n\n{{ end }}"
	jscode     = "rtgo.controllers.%s = function %s(view) {\n    'use strict';\n\n};\n"
	bodregex   = regexp.MustCompile("([[:space:]]*)</body>([[:space:]]*)")
	create     = flag.Bool("create", false, "Create a new rtgo project.")
	add        = flag.Bool("add", false, "Add either a view or controller.")
	del        = flag.Bool("del", false, "Delete either a view or controller.")
	view       = flag.String("view", "", "The name of the view to add or delete.")
	controller = flag.String("controller", "", "The name of the controller to add or delete.")
)

func CopyFile(source string, dest string) error {
	sourcefile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourcefile.Close()
	destfile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destfile.Close()
	_, err = io.Copy(destfile, sourcefile)
	if err == nil {
		if sourceinfo, err := os.Stat(source); err != nil {
			if err := os.Chmod(dest, sourceinfo.Mode()); err != nil {
				return err
			}
		}

	}
	return nil
}

func CopyDir(source string, dest string) error {
	sourceinfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, sourceinfo.Mode()); err != nil {
		return err
	}
	directory, _ := os.Open(source)
	objects, err := directory.Readdir(-1)
	for _, obj := range objects {
		sourcefilepointer := source + "/" + obj.Name()
		destinationfilepointer := dest + "/" + obj.Name()
		if obj.IsDir() {
			if err := CopyDir(sourcefilepointer, destinationfilepointer); err != nil {
				return err
			}
		} else if err := CopyFile(sourcefilepointer, destinationfilepointer); err != nil {
			return err
		}
	}
	return nil
}

func initDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsExist(err) {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return nil
}

func readFile(filename string) (string, error) {
	fbytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	file := strings.TrimSpace(string(fbytes[:]))
	return file, nil
}

func CreateNewProject() error {
	source := fmt.Sprintf("%s/src/github.com/gojonnygo/rtgo/static", os.Getenv("GOPATH"))
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dest := cwd + "/static"
	if err := CopyDir(source, dest); err != nil {
		return err
	}
	return nil
}

func DelController(name string) error {
	basefile := "./static/views/base.html"
	file, err := readFile(basefile)
	if err != nil {
		return err
	}
	jsfilename := fmt.Sprintf("/static/js/controllers/%s.js", name)
	if strings.Contains(file, jsfilename) {
		jsregex := regexp.MustCompile(fmt.Sprintf(remjsline, jsfilename))
		newfile := jsregex.ReplaceAllString(file, "\n    ")
		if err := ioutil.WriteFile(basefile, []byte(newfile), 0644); err != nil {
			return err
		}
	}
	os.Remove("." + jsfilename)
	return nil
}

func AddController(name string) error {
	initDirectory("./static/js/controllers")
	basefile := "./static/views/base.html"
	file, err := readFile(basefile)
	if err != nil {
		return err
	}
	jsfilename := fmt.Sprintf("/static/js/controllers/%s.js", name)
	if !strings.Contains(file, jsfilename) {
		jsline := fmt.Sprintf(addjsline, jsfilename)
		newfile := bodregex.ReplaceAllString(file, jsline)
		if err := ioutil.WriteFile(basefile, []byte(newfile), 0644); err != nil {
			return err
		}
	}
	if _, err := os.Stat("." + jsfilename); os.IsExist(err) {
		return nil
	}
	js := fmt.Sprintf(jscode, name, name)
	if err := ioutil.WriteFile("."+jsfilename, []byte(js), 0644); err != nil {
		return err
	}
	return nil
}

func DelView(name string) error {
	viewfile := fmt.Sprintf("./static/views/%s.html", name)
	return os.Remove(viewfile)
}

func AddView(name string) error {
	viewfile := fmt.Sprintf("./static/views/%s.html", name)
	if _, err := os.Stat(viewfile); os.IsExist(err) {
		return err
	}
	view := fmt.Sprintf(viewtext, name)
	if err := ioutil.WriteFile(viewfile, []byte(view), 0644); err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()
	if *create {
		if err := CreateNewProject(); err != nil {
			log.Fatal(err)
		}
	}
	if *add {
		if *controller != "" {
			if err := AddController(*controller); err != nil {
				log.Fatal(err)
			}
		}
		if *view != "" {
			if err := AddView(*view); err != nil {
				log.Fatal(err)
			}
		}
	} else if *del {
		if *controller != "" {
			if err := DelController(*controller); err != nil {
				log.Fatal(err)
			}
		}
		if *view != "" {
			if err := DelView(*view); err != nil {
				log.Fatal(err)
			}
		}
	}
}
