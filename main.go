package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const tpl = `---
title: "%s"
tags: ["%s"]
categories: ["%s"]
date: "%s"
url: "%s"
toc: true
draft: false
---

<!--more-->

`

type MetaData struct {
	Title      string
	Categories string
	Tag        string
	Date       string
	URL        string
}

const location = "./content/post"

func main() {
	// AddHugoHeader(location)
	CreateIndex(location)
}

// AddHugoHeader adds a header for each article
func AddHugoHeader(pos string) {
	filepath.WalkDir(pos, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		fi, _ := d.Info()

		// 过滤非md文件
		if !strings.HasSuffix(fi.Name(), ".md") {
			return nil
		}

		title := fi.Name()
		title = title[:len(title)-3] // 删除.md

		tag := strings.Split(path, "/")[0]
		if tag == "go" {
			tag = "golang"
		}
		categories := tag

		date := fi.ModTime().Local().Format("2006-01-02T15:04:05+08:00")
		day := fi.ModTime().Local().Format("2006-01-02")

		url := day + "-" + title + ".html" // 2006-01-02-${title}.html

		writeHeader(path, MetaData{
			Title:      title,
			Categories: categories,
			Tag:        tag,
			Date:       date,
			URL:        url})
		return nil
	})
}

func writeHeader(file string, meta MetaData) {
	f, err := os.OpenFile(file, os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}

	head := fmt.Sprintf(tpl, meta.Title, meta.Tag, meta.Categories, meta.Date, meta.URL)

	b, _ := ioutil.ReadAll(f)
	all := append([]byte(head), b...)

	n, err := f.WriteAt(all, 0)
	fmt.Println(n, file, meta, "done")
	f.Close()
}

const readmeHeader = `# Blog

support by 
* [hugo](https://gohugo.io)
* [utteranc](https://utteranc.es)
* [firebase](https://firebase.google.com)


## Index

`

// CreateIndex creates a index for all articles.
func CreateIndex(pos string) {
	f, err := os.OpenFile("README.md", os.O_CREATE|os.O_TRUNC|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// 写入固定的header
	f.Write([]byte(readmeHeader))

	filepath.WalkDir(pos, func(path string, d fs.DirEntry, err error) error {
		if path == "./content/post" {
			return nil
		}
		if d.IsDir() {
			appendIndex(f, path)
		}
		return nil
	})
}

func appendIndex(f *os.File, path string) {
	urls := strings.Split(path, "/")
	f.Write([]byte("### " + urls[len(urls)-1] + "\n"))
	filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		fi, _ := d.Info()

		// 过滤非md文件
		if !strings.HasSuffix(fi.Name(), ".md") {
			return nil
		}

		title := fi.Name()
		title = title[:len(title)-3] // 删除.md

		line := fmt.Sprintf("* [%s](%s)\n", title, path)
		n, err := f.Write([]byte(line))
		fmt.Println(line, n, "done")
		return nil
	})
}
