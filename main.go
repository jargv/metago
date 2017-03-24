package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"metago/funcs"
	"os"
	"text/template"
)

func main() {
	err := main_()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main_() error {
	flag.Parse()
	mgoFileName := flag.Arg(0)
	if mgoFileName == "" {
		return fmt.Errorf("missing mgo file arg")
	}

	mgoFile, err := os.Open(mgoFileName)
	if err != nil {
		return fmt.Errorf("metago: %v", err)
	}

	mgo, err := ioutil.ReadAll(mgoFile)
	if err != nil {
		return fmt.Errorf("metago: %v", err)
	}

	tmpl, err := template.New(mgoFileName).
		Funcs(template.FuncMap{
			"package": funcs.PackageFunc,
		}).
		Option("missingkey=error").Parse(string(mgo))
	if err != nil {
		return fmt.Errorf("metago: %v", err)
	}

	err = tmpl.Execute(os.Stdout, nil)
	if err != nil {
		return fmt.Errorf("metago: %v", err)
	}

	return nil
}
