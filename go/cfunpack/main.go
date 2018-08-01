package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tsukanov-as/cf"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func parseInt(s string) int {
	n, err := strconv.ParseInt(s, 10, 32)
	check(err)
	return int(n)
}

func main() {

	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <filepath>", os.Args[0])
		return
	}

	dir := cf.Load(os.Args[1])

	rootHead := dir["root"].(*cf.Tree)
	rootHead.Parse()
	rootBody := dir[rootHead.Read(1, 2)].(*cf.Tree)
	rootBody.Parse()

	objectBody := dir[rootBody.Read(1, 4, 2, 2, 4, 2, 2, 3)+".0"]

	objectModule := ""

	if objectBody != nil {
		objectModule = objectBody.(cf.Dir)["text"].(*cf.Tree).String()
	}

	fmt.Println(objectModule)

	childCount := parseInt(rootBody.Read(1, 4, 2, 3))

	childList := make([]string, 0, 8)
	for i := 0; i < childCount; i++ {
		childList = append(childList, rootBody.Read(1, 4, 2, 4+i))
	}

	formsModules := make(map[string]string)
	for _, v := range childList {
		switch v[1:37] {
		case "d5b0e5ed-256d-401c-9c36-f630cafd8a62", "a3b368c0-29e2-11d6-a3c7-0050bae0a776":
			forms := cf.Tree{}
			forms.Init(v)
			forms.Parse()
			count := parseInt(forms.Read(1, 2))
			for i := 0; i < count; i++ {
				formUUID := forms.Read(1, 3+i)
				formHead := dir[formUUID].(*cf.Tree)
				formHead.Parse()
				formName := ""
				if formHead.Read(1, 2, 1) == "1" {
					formName = formHead.Read(1, 2, 2, 2, 2, 3)
				} else {
					formName = formHead.Read(1, 2, 2, 2, 3)
				}
				formName = formName[1 : len(formName)-1]
				formBody := dir[formUUID+".0"]
				if formHead.Read(1, 2, 2, 2, 4) == "1" {
					body := formBody.(*cf.Tree)
					body.Parse()
					formsModules[formName] = "\xEF\xBB\xBF" + body.Read(1, 3)
				} else {
					body := formBody.(cf.Dir)
					formsModules[formName] = body["module"].(*cf.Tree).String()
				}

			}
		}
	}

	for k, v := range formsModules {
		fmt.Println(k)
		fmt.Println(v)
	}

}
